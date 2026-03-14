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
	file    *os.File
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

	for {
		ready, err := poller.WaitWithEvents(-1)
		if err != nil {
			return err
		}
		if len(ready) == 0 {
			continue
		}

		var connReady []ReadyEvent
		for _, re := range ready {
			fd, ev := re.Fd, re.Events
			if fd == wakeFd {
				return nil
			}
			if fd == listenerFd {
				if err := acceptConn(listener, accept, poller, fdMap, metrics, cfg, connEvents); err != nil {
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

func acceptConn(listener net.Listener, accept AcceptFunc, poller *Poller, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig, connEvents uint32) error {
	if cfg.MaxConns > 0 && fdMap.Len() >= cfg.MaxConns {
		return nil
	}
	conn, err := listener.Accept()
	if err != nil {
		if isListenerClosed(err) {
			return err
		}
		if metrics != nil && metrics.OnAcceptErr != nil {
			metrics.OnAcceptErr(err)
		}
		return nil
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil
	}
	file, err := tcpConn.File()
	if err != nil {
		conn.Close()
		return nil
	}
	connFd := int(file.Fd())
	if err := unix.SetNonblock(connFd, true); err != nil {
		file.Close()
		conn.Close()
		return nil
	}
	ch, ok := accept(conn)
	if !ok {
		file.Close()
		conn.Close()
		return nil
	}
	if err := poller.Add(connFd, connEvents); err != nil {
		ch.Close()
		file.Close()
		conn.Close()
		return nil
	}
	buf := allocReadBuf(cfg.ReadBufSize)
	fdMap.Set(connFd, &connEntry{
		conn:    conn,
		ch:      ch,
		file:    file,
		readBuf: buf,
	})
	zlog.Info("zreactor connection accepted",
		zap.Int("fd", connFd),
		zap.Uint64("channelId", ch.GetChannelId()),
		zap.String("remote", conn.RemoteAddr().String()))
	if metrics != nil && metrics.OnAccept != nil {
		metrics.OnAccept()
	}
	return nil
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

// parseAndDispatchSafe 调用 ch.ParseAndDispatch()；若发生 panic 则恢复并返回 true，由调用方关闭该连接。
func parseAndDispatchSafe(entry *connEntry, poller *Poller, fd int, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig) (shouldClose bool) {
	defer func() {
		if r := recover(); r != nil {
			zlog.Recover("zreactor ParseAndDispatch panic",
				zap.Int("fd", fd),
				zap.Uint64("channelId", entry.ch.GetChannelId()),
				zap.Any("panic", r))
			shouldClose = true
		}
	}()
	return entry.ch.ParseAndDispatch()
}

func handleConnRead(poller *Poller, fd int, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig) {
	entry, ok := fdMap.Get(fd)
	if !ok {
		_ = poller.Remove(fd)
		return
	}
	n, err := syscall.Read(fd, entry.readBuf)
	if n > 0 {
		reportReadBytes(metrics, fd, n)
		_, _ = entry.ch.WriteToReadBuffer(entry.readBuf[:n])
		if parseAndDispatchSafe(entry, poller, fd, fdMap, metrics, cfg) {
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
			reportReadBytes(metrics, fd, rn)
			_, _ = entry.ch.WriteToReadBuffer(entry.readBuf[:rn])
			if parseAndDispatchSafe(entry, poller, fd, fdMap, metrics, cfg) {
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
	zlog.Info("zreactor connection closed",
		zap.Int("fd", fd),
		zap.Uint64("channelId", entry.ch.GetChannelId()))
	entry.ch.Close()
	entry.file.Close()
	entry.conn.Close()
	// 按 cap 归还缓冲：截断后 len 可能小于 defaultReadBufSize，只要 cap 足够即可复用
	if readBufSize == defaultReadBufSize && cap(entry.readBuf) >= defaultReadBufSize {
		readBufPool.Put(entry.readBuf)
	}
	fdMap.Delete(fd)
	if metrics != nil && metrics.OnClose != nil {
		metrics.OnClose()
	}
}
