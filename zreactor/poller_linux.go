//go:build linux

package zreactor

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// ReadEvent 可读事件（含 accept 的 listener fd）
	ReadEvent = unix.EPOLLIN
	// WriteEvent 可写事件；EnableWriteEvent 时与 ReadEvent 组合使用（EPOLLIN|EPOLLOUT）。
	WriteEvent = unix.EPOLLOUT

	// defaultMinEvents 默认事件槽数；高并发可经 NewPollerWithSize 或 ServeConfig.MinEvents 调大。
	defaultMinEvents = 256
)

// ErrHup 表示对端关闭或错误，用于 reactor 显式处理 EPOLLHUP/EPOLLERR，避免依赖 Read 返回值。
const ErrHup = unix.EPOLLHUP | unix.EPOLLERR

// Poller 基于 epoll 的 I/O 多路复用，仅 Linux。
type Poller struct {
	epfd   int
	events []unix.EpollEvent
	closed bool
}

// NewPoller 创建 epoll 实例，事件槽数为默认 defaultMinEvents。
func NewPoller() (*Poller, error) {
	return NewPollerWithSize(0)
}

// NewPollerWithSize 创建 epoll 实例，eventsCap 为事件数组初始容量；<=0 时用 defaultMinEvents。
func NewPollerWithSize(eventsCap int) (*Poller, error) {
	if eventsCap <= 0 {
		eventsCap = defaultMinEvents
	}
	epfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &Poller{
		epfd:   epfd,
		events: make([]unix.EpollEvent, eventsCap),
	}, nil
}

// Add 将 fd 加入 epoll，监听 events（默认 ReadEvent）；若需写就绪可传 ReadEvent|WriteEvent。
func (p *Poller) Add(fd int, events ...uint32) error {
	ev := uint32(ReadEvent)
	if len(events) > 0 {
		ev = events[0]
	}
	return unix.EpollCtl(p.epfd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
		Events: ev,
		Fd:     int32(fd),
	})
}

// Mod 修改 fd 监听事件（如增加/移除 EPOLLOUT）。
func (p *Poller) Mod(fd int, events uint32) error {
	return unix.EpollCtl(p.epfd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	})
}

// Remove 从 epoll 移除 fd。fd 已关闭或已移除时返回错误会忽略（EBADF/ENOENT），避免资源清理时报错。
func (p *Poller) Remove(fd int) error {
	err := unix.EpollCtl(p.epfd, unix.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		if err == syscall.EBADF || err == syscall.ENOENT {
			return nil
		}
		return err
	}
	return nil
}

// ReadyEvent 就绪 fd 及其事件标志，用于显式处理 EPOLLHUP/EPOLLERR 等。
type ReadyEvent struct {
	Fd     int
	Events uint32
}

// Wait 等待事件，返回就绪的 fd 列表；msec < 0 表示阻塞。
// 内部调用 WaitWithEvents 并丢弃事件，兼容旧用法。
func (p *Poller) Wait(msec int) (ready []int, err error) {
	evs, err := p.WaitWithEvents(msec)
	if err != nil || len(evs) == 0 {
		return nil, err
	}
	ready = make([]int, len(evs))
	for i := range evs {
		ready[i] = evs[i].Fd
	}
	return ready, nil
}

// WaitWithEvents 等待事件，返回就绪的 (fd, events) 列表；msec < 0 表示阻塞。
// EINTR 时自动重试。调用方可根据 Events 判断 EPOLLHUP/EPOLLERR 做显式关连接等处理。
func (p *Poller) WaitWithEvents(msec int) (ready []ReadyEvent, err error) {
	for {
		n, err := unix.EpollWait(p.epfd, p.events, msec)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return nil, err
		}
		if n == 0 {
			return nil, nil
		}
		ready = make([]ReadyEvent, n)
		for i := 0; i < n; i++ {
			ready[i] = ReadyEvent{Fd: int(p.events[i].Fd), Events: p.events[i].Events}
		}
		return ready, nil
	}
}

// Close 关闭 epoll 实例。
func (p *Poller) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return unix.Close(p.epfd)
}
