package zmodel

func (m *Message) SmartReset() {
	m.MsgId = 0
	m.Data = nil
	m.AuthId = 0
	m.ToClient = false
	m.SrcActor = 0
	m.TarActor = 0
	m.SessionId = 0
	m.FromClient = false
	m.IsResponse = false
	m.RpcId = 0
	m.TraceIdHi = 0
	m.TraceIdLo = 0
	m.SpanId = 0
	m.SeqId = 0
	m.RefCount = 0
	m.AuthIds = nil
}

// PoolReset 池专用重置：清空字段值但保留 Data/AuthIds 的底层容量
// ⚠️ 仅供 GetMessage() 使用，不要在其他地方调用
func (m *Message) PoolReset() {
	m.MsgId = 0
	m.Data = m.Data[:0] // 保留 cap，避免重新分配
	m.AuthId = 0
	m.ToClient = false
	m.SrcActor = 0
	m.TarActor = 0
	m.SessionId = 0
	m.FromClient = false
	m.IsResponse = false
	m.RpcId = 0
	m.TraceIdHi = 0
	m.TraceIdLo = 0
	m.SpanId = 0
	m.SeqId = 0
	m.AuthIds = m.AuthIds[:0] // 保留 cap，避免重新分配
}
