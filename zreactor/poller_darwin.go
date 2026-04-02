//go:build darwin

package zreactor

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// ReadEvent 可读事件；仅用于 reactor/poller 内部语义映射（非 epoll 事件值）。
	ReadEvent = 1 << 0
	// WriteEvent 可写事件；仅用于 reactor/poller 内部语义映射（非 epoll 事件值）。
	WriteEvent = 1 << 1
	// ErrHup 表示连接 EOF/错误等应关闭连接的事件标志（仅用于 reactor loop 分支）。
	ErrHup = 1 << 2
)

// defaultMinEvents 默认事件槽数；高并发可通过 ServeConfig.MinEvents 调大。
const defaultMinEvents = 256

// Poller 基于 kqueue 的 I/O 多路复用器，仅 darwin。
type Poller struct {
	kq     int
	events []unix.Kevent_t
	closed bool
}

func NewPoller() (*Poller, error) {
	return NewPollerWithSize(0)
}

// NewPollerWithSize 创建 kqueue，事件槽容量为 eventsCap；<=0 时使用 defaultMinEvents。
func NewPollerWithSize(eventsCap int) (*Poller, error) {
	if eventsCap <= 0 {
		eventsCap = defaultMinEvents
	}
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	return &Poller{
		kq:     kq,
		events: make([]unix.Kevent_t, eventsCap),
	}, nil
}

// Add 将 fd 加入 kqueue，监听 events 位掩码（包含 ReadEvent/WriteEvent）。
func (p *Poller) Add(fd int, events ...uint32) error {
	mask := uint32(ReadEvent)
	if len(events) > 0 {
		mask = events[0]
	}

	changes := make([]unix.Kevent_t, 0, 2)
	if mask&ReadEvent != 0 {
		changes = append(changes, unix.Kevent_t{
			Ident:  uint64(fd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_CLEAR,
		})
	}
	if mask&WriteEvent != 0 {
		changes = append(changes, unix.Kevent_t{
			Ident:  uint64(fd),
			Filter: unix.EVFILT_WRITE,
			Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_CLEAR,
		})
	}
	_, err := unix.Kevent(p.kq, changes, nil, nil)
	return err
}

// Mod 修改 fd 关注事件（Read/Write 组合）；当前 reactor 不使用 Mod，可实现为“删除再添加”。
func (p *Poller) Mod(fd int, events uint32) error {
	_ = p.Remove(fd)
	return p.Add(fd, events)
}

// Remove 从 kqueue 移除 fd；忽略不存在的删除错误。
func (p *Poller) Remove(fd int) error {
	changes := []unix.Kevent_t{
		{Ident: uint64(fd), Filter: unix.EVFILT_READ, Flags: unix.EV_DELETE},
		{Ident: uint64(fd), Filter: unix.EVFILT_WRITE, Flags: unix.EV_DELETE},
	}
	_, err := unix.Kevent(p.kq, changes, nil, nil)
	if err == syscall.ENOENT {
		return nil
	}
	return err
}

// ReadyEvent 就绪事件，用于 reactor loop 处理。
type ReadyEvent struct {
	Fd     int
	Events uint32
	Data   int64
}

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

// WaitWithEvents 等待就绪事件并返回 (fd, events) 列表；msec < 0 表示阻塞。
func (p *Poller) WaitWithEvents(msec int) (ready []ReadyEvent, err error) {
	for {
		var ts *unix.Timespec
		if msec >= 0 {
			sec := int64(msec / 1000)
			nsec := int64(msec%1000) * int64(1e6)
			ts = &unix.Timespec{Sec: sec, Nsec: nsec}
		}

		n, err := unix.Kevent(p.kq, nil, p.events, ts)
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
			ke := p.events[i]
			fd := int(ke.Ident)
			evMask := uint32(0)

			switch ke.Filter {
			case unix.EVFILT_READ:
				evMask |= ReadEvent
			case unix.EVFILT_WRITE:
				evMask |= WriteEvent
			}

			// Treat only EV_ERROR as ErrHup.
			// EOF（EV_EOF）可能与“仍有未读数据”同事件到达，避免在 Parse 前直接 close。
			if ke.Flags&unix.EV_ERROR != 0 {
				evMask |= ErrHup
			}

			ready[i] = ReadyEvent{Fd: fd, Events: evMask, Data: ke.Data}
		}
		return ready, nil
	}
}

func (p *Poller) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return unix.Close(p.kq)
}
