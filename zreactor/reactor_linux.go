//go:build linux

package zreactor

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// readBufPool 默认 4096 字节缓冲池；当 ServeConfig.ReadBufSize == defaultReadBufSize 时复用。
var readBufPool = sync.Pool{
	New: func() any { return make([]byte, defaultReadBufSize) },
}

type connEntry struct {
	conn    net.Conn
	ch      ReactorChannel
	readBuf []byte
}

// readResult 批量读时暂存单次 Read 的结果
type readResult struct {
	n   int
	err error
}

const batchSliceMinCap = 64

type batchWork struct {
	results []readResult
	entries []*connEntry
}

var batchPool = sync.Pool{
	New: func() any {
		return &batchWork{
			results: make([]readResult, 0, batchSliceMinCap),
			entries: make([]*connEntry, 0, batchSliceMinCap),
		}
	},
}

func getBatchSlices(n int) *batchWork {
	w := batchPool.Get().(*batchWork)
	if cap(w.results) < n {
		w.results = make([]readResult, n)
		w.entries = make([]*connEntry, n)
	} else {
		w.results = w.results[:n]
		w.entries = w.entries[:n]
	}
	return w
}

func putBatchSlices(w *batchWork) {
	if w == nil {
		return
	}
	for i := range w.entries {
		w.entries[i] = nil
	}
	w.results = w.results[:0]
	w.entries = w.entries[:0]
	batchPool.Put(w)
}

// Serve 在调用方 goroutine 中运行 reactor 循环；仅 Linux，listener 须为 *net.TCPListener。
// 等价于 ServeWithConfig(ctx, listener, accept, metrics, nil)。
func Serve(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics) error {
	return ServeWithConfig(ctx, listener, accept, metrics, nil)
}

// ServeWithConfig 与 Serve 相同，但使用 config 控制缓冲大小、事件槽数、批量解析与写事件；config 为 nil 时用默认值。
func ServeWithConfig(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics, config *ServeConfig) error {
	cfg := applyServeConfig(config)
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return ErrNotTCPListener
	}
	listenerFile, err := tcpListener.File()
	if err != nil {
		return err
	}
	defer listenerFile.Close()
	listenerFd := int(listenerFile.Fd())
	// accept4 的 nonblock 标志只作用于“新连接 fd”，不会让监听 fd 变成非阻塞。
	// 监听 fd 若仍是阻塞模式，drain backlog 到空后会卡在 accept4，导致 reactor 主循环不再回到 epoll_wait。
	if err := unix.SetNonblock(listenerFd, true); err != nil {
		return err
	}

	wakeFd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return err
	}
	defer unix.Close(wakeFd)

	poller, err := NewPollerWithSize(cfg.MinEvents)
	if err != nil {
		return err
	}
	defer poller.Close()
	if err := poller.Add(listenerFd); err != nil {
		return err
	}
	if err := poller.Add(wakeFd); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_, _ = unix.Write(wakeFd, []byte{1, 0, 0, 0, 0, 0, 0, 0})
	}()

	fdMap := newShardedFDMap()
	connEvents := connEventsForAdd(cfg.EnableWriteEvent)
	pollMs := heartbeatPollMs(cfg)

	for {
		ready, err := poller.WaitWithEvents(pollMs)
		if err != nil {
			return err
		}
		if len(ready) == 0 {
			if pollMs > 0 {
				checkHeartbeats(poller, fdMap, metrics, cfg)
			}
			continue
		}

		var connReady []ReadyEvent
		for _, re := range ready {
			fd, ev := re.Fd, re.Events
			if fd == wakeFd {
				return nil
			}
			if fd == listenerFd {
				if err := acceptConn(listenerFd, accept, poller, fdMap, metrics, cfg, connEvents); err != nil {
					return err
				}
				continue
			}
			// 显式处理 EPOLLHUP/EPOLLERR，避免 fd 已删但 epoll 仍上报事件的极端场景下空转或重复处理
			if ev&ErrHup != 0 {
				if entry, ok := fdMap.Get(fd); ok {
					closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
				} else {
					_ = poller.Remove(fd)
				}
				continue
			}
			if cfg.BatchRead {
				connReady = append(connReady, re)
				continue
			}
			handleConnRead(poller, fd, fdMap, metrics, cfg)
		}

		if cfg.BatchRead && len(connReady) > 0 {
			batchReadAndParse(poller, connReady, fdMap, metrics, cfg)
		}
	}
}

func connEventsForAdd(enableWrite bool) uint32 {
	if enableWrite {
		return ReadEvent | WriteEvent
	}
	return ReadEvent
}

func acceptConn(listenerFd int, accept AcceptFunc, poller *Poller, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig, connEvents uint32) error {
	// 使用 accept4(nonblock) drain backlog，避免 net.TCPListener.Accept 在 reactor goroutine 中阻塞。
	// 但在持续新连接涌入时，“一直 accept 到 EAGAIN”会饿死读事件处理；
	// 因此每次 listener 事件最多接入一批，随后返回主循环继续处理连接读。
	const maxAcceptPerTurn = 64
	acceptedThisTurn := 0
	for {
		if cfg.MaxConns > 0 && fdMap.Len() >= cfg.MaxConns {
			return nil
		}
		if acceptedThisTurn >= maxAcceptPerTurn {
			return nil
		}

		nfd, _, err := unix.Accept4(listenerFd, unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			// drained
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				return nil
			}
			if isListenerClosed(err) {
				return err
			}
			zlog.Warn("zreactor accept error", zap.Error(err))
			if metrics != nil && metrics.OnAcceptErr != nil {
				metrics.OnAcceptErr(err)
			}
			return nil
		}
		acceptedThisTurn++

		// 将 fd 转为 net.Conn 交给上层 acceptFn；FileConn 会 dup fd，因此要关闭原始 file 防泄漏。
		f := os.NewFile(uintptr(nfd), "zreactor-accepted")
		conn, ferr := net.FileConn(f)
		_ = f.Close()
		if ferr != nil {
			_ = unix.Close(nfd)
			continue
		}

		tcpConn, ok := conn.(*net.TCPConn)
		if !ok {
			_ = conn.Close()
			continue
		}

		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			conn.Close()
			continue
		}
		connFd := -1
		if err := rawConn.Control(func(fd uintptr) {
			connFd = int(fd)
		}); err != nil || connFd < 0 {
			conn.Close()
			continue
		}
		if err := unix.SetNonblock(connFd, true); err != nil {
			conn.Close()
			continue
		}
		ch, ok := accept(conn)
		if !ok {
			conn.Close()
			continue
		}
		if err := poller.Add(connFd, connEvents); err != nil {
			ch.Close()
			conn.Close()
			continue
		}
		buf := allocReadBuf(cfg.ReadBufSize)
		fdMap.Set(connFd, &connEntry{
			conn:    conn,
			ch:      ch,
			readBuf: buf,
		})
		if metrics != nil && metrics.OnAccept != nil {
			metrics.OnAccept()
		}
	}
}

func allocReadBuf(size int) []byte {
	if size == defaultReadBufSize {
		b := readBufPool.Get().([]byte)
		if cap(b) < size {
			b = make([]byte, size)
		}
		return b[:size]
	}
	return make([]byte, size)
}

func handleConnRead(poller *Poller, fd int, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig) {
	entry, ok := fdMap.Get(fd)
	if !ok {
		_ = poller.Remove(fd)
		return
	}
	endRead := beginReactorRead(entry.ch)
	defer endRead()

	n, err := syscall.Read(fd, entry.readBuf)
	if n > 0 {
		reportReadBytes(metrics, fd, n)
		if ingestConnReadAndDispatch(entry, fd, entry.readBuf[:n]) {
			closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
			return
		}
		if err != nil && err != syscall.EAGAIN {
			reportReadErr(metrics, fd, err)
			closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
			return
		}
	}
	if err != nil {
		if err == syscall.EAGAIN {
			return
		}
		reportReadErr(metrics, fd, err)
		closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
		return
	}
	if n == 0 {
		closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
	}
}

func reportReadBytes(metrics *Metrics, fd, n int) {
	if metrics != nil && metrics.OnReadBytes != nil {
		metrics.OnReadBytes(fd, n)
	}
}

// batchReadAndParse 批量收集就绪 FD → 统一 Read → 统一 ParseAndDispatch，减少跨 FD 切换。
// entry 为 nil 时（fd 已从 fdMap 删除）会从 epoll 移除该 fd，避免后续空转。
func batchReadAndParse(poller *Poller, connReady []ReadyEvent, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig) {
	n := len(connReady)
	w := getBatchSlices(n)
	defer putBatchSlices(w)
	results, entries := w.results, w.entries
	for i := range results {
		results[i] = readResult{}
		entries[i] = nil
	}
	for i, re := range connReady {
		fd := re.Fd
		entry, ok := fdMap.Get(fd)
		if !ok {
			results[i].err = syscall.EBADF
			continue
		}
		entries[i] = entry
		results[i].n, results[i].err = syscall.Read(fd, entry.readBuf)
	}
	for i, re := range connReady {
		fd := re.Fd
		entry := entries[i]
		if entry == nil {
			_ = poller.Remove(fd)
			continue
		}
		rn, err := results[i].n, results[i].err
		if rn > 0 {
			endRead := beginReactorRead(entry.ch)
			reportReadBytes(metrics, fd, rn)
			if ingestConnReadAndDispatch(entry, fd, entry.readBuf[:rn]) {
				endRead()
				closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
				continue
			}
			endRead()
			if err != nil && err != syscall.EAGAIN {
				reportReadErr(metrics, fd, err)
				closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
			}
			continue
		}
		if err != nil {
			if err == syscall.EAGAIN {
				continue
			}
			reportReadErr(metrics, fd, err)
		}
		closeConn(poller, fd, entry, fdMap, metrics, cfg.ReadBufSize)
	}
}

func reportReadErr(metrics *Metrics, fd int, err error) {
	kind := classifyReadErr(err)
	zlog.Warn("zreactor read error",
		zap.Int("fd", fd),
		zap.String("kind", kind.String()),
		zap.Error(err))
	if metrics == nil {
		return
	}
	if metrics.OnReadErrWithKind != nil {
		metrics.OnReadErrWithKind(fd, err, kind)
		return
	}
	if metrics.OnReadErr != nil {
		metrics.OnReadErr(fd, err)
	}
}

func isListenerClosed(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "listener closed")
}

func closeConn(poller *Poller, fd int, entry *connEntry, fdMap *shardedFDMap, metrics *Metrics, readBufSize int) {
	_ = poller.Remove(fd)
	closeReactorChannel(entry.ch)
	// 按 cap 归还缓冲：截断后 len 可能小于 defaultReadBufSize，只要 cap 足够即可复用
	if readBufSize == defaultReadBufSize && cap(entry.readBuf) >= defaultReadBufSize {
		readBufPool.Put(entry.readBuf)
	}
	fdMap.Delete(fd)
	if metrics != nil && metrics.OnClose != nil {
		metrics.OnClose()
	}
}
