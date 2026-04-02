//go:build darwin

package zreactor

import (
	"context"
	"net"
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

// Serve 在调用方 goroutine 中运行 reactor 循环；仅 darwin 实现（kqueue）。
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

	// wakeup: use unix.Pipe instead of eventfd (not available on darwin).
	// unix.Pipe2 在部分 x/sys 版本里不可用，这里用 Pipe + SetNonblock/CloseOnExec 组合实现。
	wakeFds := []int{0, 0}
	if err := unix.Pipe(wakeFds); err != nil {
		return err
	}
	wakeR, wakeW := wakeFds[0], wakeFds[1]
	_ = unix.SetNonblock(wakeR, true)
	_ = unix.SetNonblock(wakeW, true)
	unix.CloseOnExec(wakeR)
	unix.CloseOnExec(wakeW)
	defer unix.Close(wakeR)
	defer unix.Close(wakeW)

	poller, err := NewPollerWithSize(cfg.MinEvents)
	if err != nil {
		return err
	}
	defer poller.Close()

	if err := poller.Add(listenerFd); err != nil {
		return err
	}
	if err := poller.Add(wakeR); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_, _ = unix.Write(wakeW, []byte{1})
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
			if fd == wakeR {
				return nil
			}
			if fd == listenerFd {
				// kqueue listener 事件为 EV_CLEAR（边沿触发），并且 kevent.Data 对 listener socket 表示待 accept 数量。
				// 我们按 Data 次数尽量 drain backlog，避免漏触发导致“丢连接/少接入”。
				toAccept := int(re.Data)
				if toAccept <= 0 {
					toAccept = 1
				}
				if err := acceptConn(tcpListener, toAccept, accept, poller, fdMap, metrics, cfg, connEvents); err != nil {
					return err
				}
				continue
			}
			// Explicitly handle EOF/ERROR-like events to avoid read loop relying only on read result.
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

func acceptConn(tcpListener *net.TCPListener, toAccept int, accept AcceptFunc, poller *Poller, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig, connEvents uint32) error {
	for i := 0; i < toAccept; i++ {
		if cfg.MaxConns > 0 && fdMap.Len() >= cfg.MaxConns {
			return nil
		}
		conn, err := tcpListener.Accept()
		if err != nil {
			if isListenerClosed(err) {
				return err
			}
			zlog.Warn("zreactor accept error", zap.Error(err))
			if metrics != nil && metrics.OnAcceptErr != nil {
				metrics.OnAcceptErr(err)
			}
			return nil
		}

		tcpConn, ok := conn.(*net.TCPConn)
		if !ok {
			_ = conn.Close()
			continue
		}
		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			_ = conn.Close()
			continue
		}

		connFd := -1
		if err := rawConn.Control(func(fd uintptr) {
			connFd = int(fd)
		}); err != nil || connFd < 0 {
			_ = conn.Close()
			continue
		}
		if err := unix.SetNonblock(connFd, true); err != nil {
			_ = conn.Close()
			continue
		}

		ch, ok := accept(conn)
		if !ok {
			_ = conn.Close()
			continue
		}

		if err := poller.Add(connFd, connEvents); err != nil {
			ch.Close()
			_ = conn.Close()
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
	entry.ch.Close()
	_ = entry.conn.Close()
	if readBufSize == defaultReadBufSize && cap(entry.readBuf) >= defaultReadBufSize {
		readBufPool.Put(entry.readBuf)
	}
	fdMap.Delete(fd)
	if metrics != nil && metrics.OnClose != nil {
		metrics.OnClose()
	}
}
