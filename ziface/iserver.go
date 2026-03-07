package ziface

import (
	"context"
	"time"
)

// heartbeatConfigurable 内部接口，用于设置 channel 的心跳超时
type heartbeatConfigurable interface {
	setHeartbeatTimeout(d time.Duration)
}

type (
	IServer interface {
		Server(ctx context.Context) // 服务启动
		Close()                     // 关闭连接
		GetEncrypt() IEncrypt
		HandleRead(channel IChannel, message IWireMessage) // 收包分发（由 channel 内部调用）
		GetChannel(channelId uint64) IChannel
		GetAddr() string             // 返回实际监听地址
		SetMaxConnections(max int64) // 设置最大连接数（0 = 不限）
		SetTLSConfig(cfg *TLSConfig) // 配置 TLS/GM-TLS（nil = 不启用）

		// 认证管理（原 SessionManager 职责）
		SetChannelAuth(channelId uint64, authId int64) // 绑定认证 ID
		GetChannelByAuthId(authId int64) IChannel      // 通过认证 ID 获取 Channel

		// 内部方法：由 channel.Close() 自动调用，外部不应直接使用
		RemoveChannel(channelId uint64)

		SetEncrypt(iEncrypt IEncrypt)
	}
)
