package ziface

type IMetrics interface {
	ConnInc()
	ConnDec()
	ConnRejectedInc()
}

type IChannelMetrics interface {
	BytesRecAdd(delta int64)
	BytesSentAdd(delta int64)
	ConnErrorsInc()
	ConnHeartbeatTimeoutInc()
}
type ISessionStats interface {
	RecordSend(count int, bytes int)
	RecordRec(bytes int)
}
