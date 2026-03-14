package zreactor

import "net"

// ReactorChannel 由 reactor 驱动的通道，不阻塞读循环；实现者如 *znet.BaseChannel。
type ReactorChannel interface {
	WriteToReadBuffer(p []byte) (n int, err error)
	ParseAndDispatch() bool
	GetChannelId() uint64
	Close()
}

// AcceptFunc 接受新连接并返回要加入 reactor 的 channel；返回 (nil, false) 表示拒绝。
type AcceptFunc func(conn net.Conn) (channel ReactorChannel, ok bool)

// ReadErrKind 读错误分类，便于监控与问题定位。
type ReadErrKind int

const (
	ReadErrKindUnknown ReadErrKind = iota
	ReadErrKindTimeout
	ReadErrKindReset
	ReadErrKindClosed
	ReadErrKindOther
)

func (k ReadErrKind) String() string {
	switch k {
	case ReadErrKindTimeout:
		return "timeout"
	case ReadErrKindReset:
		return "reset"
	case ReadErrKindClosed:
		return "closed"
	case ReadErrKindOther:
		return "other"
	default:
		return "unknown"
	}
}

// Metrics 可选监控回调；均为 nil 时不埋点。无内置 Prometheus 等实现，需调用方自行适配。
// OnReadErr 与 OnReadErrWithKind 二选一：若设置了 OnReadErrWithKind 则优先调用，否则调用 OnReadErr。
// OnReadBytes 每次成功读后调用，可用于统计读字节数；OnAccept/OnClose 可用来维护连接数。
type Metrics struct {
	OnAccept          func()
	OnClose           func()
	OnReadErr         func(fd int, err error)
	OnReadErrWithKind func(fd int, err error, kind ReadErrKind)
	OnAcceptErr       func(err error)
	OnReadBytes       func(fd int, n int)
}

// ServeConfig 可选配置；nil 时使用默认值。
type ServeConfig struct {
	// ReadBufSize 每连接读缓冲字节数，默认 4096；可按场景调大以减少 read 次数。
	ReadBufSize int
	// MinEvents epoll 事件数组初始大小，默认 256；高并发可适当调大（如 1024）减少 Wait 轮次。
	MinEvents int
	// BatchRead 为 true 时先批量收集就绪 fd 再统一 Read，然后统一 ParseAndDispatch，减少跨 fd 切换。
	BatchRead bool
	// EnableWriteEvent 为 true 时新连接以 EPOLLIN|EPOLLOUT 加入 epoll，其余写路径未实现：
	// 未实现写缓冲、epoll_ctl Mod 触发写、写完成回调、写超时与写错误监控，属预留能力。
	EnableWriteEvent bool
	// MaxConns 连接数上限；>0 时 accept 前检查 fdMap.Len()，超过则拒绝新连接（不调用 accept）。
	// 0 表示不限制，极端高并发时可能耗尽文件描述符。
	MaxConns int
}

const (
	defaultReadBufSize  = 4096
	defaultMinEventsNum = 256 // 与 poller 默认事件槽一致，仅用于 ServeConfig 默认值
)

// defaultServeConfig 返回默认配置，调用方不应修改返回值。
func defaultServeConfig() *ServeConfig {
	return &ServeConfig{
		ReadBufSize:      defaultReadBufSize,
		MinEvents:        defaultMinEventsNum,
		BatchRead:        false,
		EnableWriteEvent: false,
		MaxConns:         0,
	}
}

// applyServeConfig 合并 config，未设置字段用默认值；返回的配置供 reactor 使用。
func applyServeConfig(c *ServeConfig) *ServeConfig {
	if c == nil {
		return defaultServeConfig()
	}
	out := *c
	if out.ReadBufSize <= 0 {
		out.ReadBufSize = defaultReadBufSize
	}
	if out.MinEvents <= 0 {
		out.MinEvents = defaultMinEventsNum
	}
	return &out
}
