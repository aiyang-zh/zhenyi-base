package ziface

import (
	"context"
)

// ITransport 传输层接口（纯网络 I/O）
//
// 负责底层连接的读写和生命周期管理。
// 实现者：znet.BaseChannel（通过 ztcp/zws/zkcp 的具体 Channel 嵌入）
type ITransport interface {
	Start()                           // 启动读取循环（阻塞）
	SendBatchMsg(messages []IMessage) // 批量发送（同步，直接写入网络，writev）
	Close()                           // 关闭连接
	GetChannelId() uint64             // 获取通道 ID
	IsOpen() bool                     // 是否打开
	Flush() error                     // 刷新缓冲区
	GetWriterTier() BufferTier        // 获取 writer 等级（用于监控）
	GetBuffered() int                 // 获取缓冲区已写入但未刷新的字节数
}

// ISession 会话层接口（业务状态管理）
//
// 负责认证、心跳、限流、异步发送等会话级逻辑。
// 实现者：znet.BaseChannel
type ISession interface {
	// 认证管理
	GetAuthId() int64       // 获取认证 ID（用户 ID）
	SetAuthId(authId int64) // 设置认证 ID

	// RPC 管理
	GetRpcId() uint64 // 获取并递增 RPC ID

	// 限流
	SetLimit(rate ILimit) // 设置限流器
	Allow() bool          // 检查是否允许通过（限流检查）

	// 心跳检测
	UpdateLastRecTime() // 更新最后接收时间
	Check() bool        // 检查是否超时

	// 关闭回调
	SetCloseCall(closeCall func(IChannel)) // 设置关闭回调

	// 异步发送（入队列，由 runSend goroutine 处理）
	Send(msg IMessage) // 异步发送单条消息

	// 启动发送 goroutine
	StartSend(ctx context.Context) // 启动异步发送循环

	RecordRecv(dataLen int)
}

// IChannel 通道接口 = 传输层 + 会话层
//
// 代表一个客户端连接的完整抽象，组合了网络 I/O 和会话管理。
// 实现者：znet.BaseChannel
type IChannel interface {
	ITransport
	ISession
}
