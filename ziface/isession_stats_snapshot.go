package ziface

// ISessionStatsSnapshot 由 ISessionStats 实现者可选实现，用于从单连接统计对象读出数值快照
//（供 BaseServer 聚合进 Gate / Prometheus，避免每条连接单独注册 IMonitorable）。
type ISessionStatsSnapshot interface {
	ISessionStats
	SessionStatsValues() (sendCount, recvCount, sendBytes, recvBytes, connectedAtMs, lastActiveMs int64)
}

// IChannelSessionStatsSnapshot 由 Channel（如 *znet.BaseChannel）实现，从挂接的 ISessionStats 拉取快照。
type IChannelSessionStatsSnapshot interface {
	IChannel
	SessionStatsSnapshot() (sendCount, recvCount, sendBytes, recvBytes, connectedAtMs, lastActiveMs int64, ok bool)
}
