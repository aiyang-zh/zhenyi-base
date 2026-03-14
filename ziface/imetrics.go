package ziface

// IMetrics 服务级别的连接指标接口。
//
// 用于统计整体连接数量及被拒绝连接数量，便于对外暴露监控。
type IMetrics interface {
	// ConnInc 记录新连接建立。
	ConnInc()

	// ConnDec 记录连接关闭。
	ConnDec()

	// ConnRejectedInc 记录连接因限流/容量等原因被拒绝。
	ConnRejectedInc()
}

// IChannelMetrics 单连接维度的指标接口。
//
// 用于统计每条连接上的收发字节数、错误数等。
type IChannelMetrics interface {
	// BytesRecAdd 累加收到的字节数。
	BytesRecAdd(delta int64)

	// BytesSentAdd 累加发送的字节数。
	BytesSentAdd(delta int64)

	// ConnErrorsInc 累加连接错误次数。
	ConnErrorsInc()

	// ConnHeartbeatTimeoutInc 累加心跳超时次数。
	ConnHeartbeatTimeoutInc()
}

// ISessionStats 会话级统计接口。
//
// 用于在业务层记录发送/接收的消息与字节数。
type ISessionStats interface {
	// RecordSend 记录一次发送操作（消息数量与字节数）。
	RecordSend(count int, bytes int)

	// RecordRec 记录一次接收操作的字节数。
	RecordRec(bytes int)
}

// IChannelMetricsSetter 用于向 Channel 注入单连接维度指标收集器。
// 由 znet.BaseChannel 实现；BaseServer.SetChannelMetrics 后，AddChannel 时会对实现该接口的 channel 自动注入。
type IChannelMetricsSetter interface {
	SetChannelMetrics(m IChannelMetrics)
}
