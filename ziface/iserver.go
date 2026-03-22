package ziface

import (
	"context"
	"time"
)

// IServer 网络服务器抽象接口。
//
// 代表一类通用的服务器实现（TCP/WebSocket/KCP 等），由 znet/ztcp/zws/zkcp 具体实现。
// 业务侧通常不直接实现该接口，而是通过 zserver 进行封装和使用。
type IServer interface {
	// Server 启动服务主循环，阻塞直至上下文被取消。
	Server(ctx context.Context)

	// Close 关闭服务器并断开所有连接。
	Close()

	// GetEncrypt 返回当前启用的加密实现（如有）。
	GetEncrypt() IEncrypt

	// HandleRead 收包分发回调，由底层 channel 在读取到完整消息后调用。
	HandleRead(channel IChannel, message IWireMessage)

	// GetChannel 根据 ChannelID 获取连接。
	GetChannel(channelId uint64) IChannel

	// GetAddr 返回实际监听地址（可能不同于配置的 :0）。
	GetAddr() string

	// SetMaxConnections 设置最大连接数（0 表示不限制）。
	SetMaxConnections(max int64)

	// SetTLSConfig 配置 TLS/GM-TLS（传入 nil 表示不启用加密）。
	SetTLSConfig(cfg *TLSConfig)

	// SetHeartbeatTimeout 配置心跳超时（基于 conn.SetReadDeadline）。<=0 表示禁用；不调用则使用默认 30s。
	SetHeartbeatTimeout(timeout time.Duration)

	// 认证管理（原 SessionManager 职责）

	// SetChannelAuth 为指定 Channel 绑定业务侧的认证 ID。
	SetChannelAuth(channelId uint64, authId uint64)

	// GetChannelByAuthId 通过业务侧认证 ID 查找对应 Channel。
	GetChannelByAuthId(authId uint64) IChannel

	// RemoveChannel 从服务器管理中移除一个 Channel。
	// 该方法由 channel.Close() 内部自动调用，业务侧不应直接使用。
	RemoveChannel(channelId uint64)

	// SetEncrypt 设置当前服务器使用的加密实现。
	SetEncrypt(iEncrypt IEncrypt)

	// SyncMode 是否同步模式（无发送队列，handler 用 ReplyImmediate 直写）。
	SyncMode() bool
}
