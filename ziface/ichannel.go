package ziface

import (
	"context"
	"time"
)

// ITransport 传输层接口（纯网络 I/O）。
//
// 负责底层连接的读写和生命周期管理。
// 实现者：znet.BaseChannel（通过 ztcp/zws/zkcp 的具体 Channel 嵌入）。
type ITransport interface {
	// Start 启动读取循环（阻塞当前 goroutine）。
	Start()

	// SendBatchMsg 批量发送多条消息（同步，直接写入网络，内部可能使用 writev）。
	SendBatchMsg(messages []IMessage)

	// Close 关闭连接。
	Close()

	// GetChannelId 返回通道 ID。
	GetChannelId() uint64

	// IsOpen 返回连接是否仍然打开。
	IsOpen() bool

	// Flush 刷新写缓冲区，将积压数据推送到网络。
	Flush() error

	// GetWriterTier 返回当前 writer 的缓冲等级，用于监控。
	GetWriterTier() BufferTier

	// GetBuffered 返回写缓冲区中已写入但尚未刷新的字节数。
	GetBuffered() int

	// WriteImmediate 读协程内同步直写，sync/RPC 场景使用，直接写出降低延迟。
	WriteImmediate(msg IWireMessage) error
}

// ISession 会话层接口（业务状态管理）。
//
// 负责认证、心跳、限流、异步发送等会话级逻辑。
// 实现者：znet.BaseChannel。
type ISession interface {
	// 认证管理

	// GetAuthId 获取当前会话绑定的认证 ID（例如用户 ID）。
	GetAuthId() uint64

	// SetAuthId 设置会话的认证 ID。
	SetAuthId(authId uint64)

	// RPC 管理

	// GetRpcId 获取并递增 RPC ID，用于请求追踪。
	GetRpcId() uint64

	// 限流

	// SetLimit 设置限流器。
	SetLimit(rate ILimit)

	// Allow 检查当前请求是否允许通过（限流检查）。
	Allow() bool

	// 心跳检测

	// SetHeartbeatTimeout 设置心跳超时（0 表示禁用），由 Server.AddChannel 调用。
	SetHeartbeatTimeout(d time.Duration)

	// UpdateLastRecTime 更新最后一次接收数据的时间。
	UpdateLastRecTime()

	// Check 检查是否发生心跳超时。
	Check() bool

	// 关闭回调

	// SetCloseCall 设置连接关闭时的回调。
	SetCloseCall(closeCall func(IChannel))

	// 异步发送（入队列，由内部 goroutine 处理）

	// Send 异步发送单条消息。
	Send(msg IMessage)

	// 启动发送 goroutine

	// StartSend 启动异步发送循环，通常在单独 goroutine 中调用。
	StartSend(ctx context.Context)

	// RecordRecv 记录一次接收的字节数（用于统计）。
	RecordRecv(dataLen int)
}

// IChannel 通道接口 = 传输层 + 会话层。
//
// 代表一个客户端连接的完整抽象，组合了网络 I/O 和会话管理。
// 实现者：znet.BaseChannel。
type IChannel interface {
	ITransport
	ISession
}
