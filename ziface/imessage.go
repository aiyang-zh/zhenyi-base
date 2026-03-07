package ziface

// IWireMessage 线协议消息接口（网络层）
//
// 表示线上传输的最小消息单元：消息ID + 序列号 + 数据。
// 由 BaseSocket 解析产生，也用于 PreparePacket 构建发送数据包。
// 实现者：zmodel.NetMessage, zmodel.Message
type IWireMessage interface {
	GetMsgId() int32
	SetMsgId(int32)
	GetSeqId() uint32
	SetSeqId(uint32)
	GetMessageData() []byte
	SetMessageData([]byte)
	Reset()
}

// IMessage 可发送消息接口（含生命周期管理）
//
// 用于通过 Channel.Send 发送消息。框架在发送完成后自动调用 Release 归还到池。
// 实现者：
//   - zmodel.NetMessage — 轻量线协议消息（客户端通信用）
//   - zmodel.Message — 业务信封消息（服务间通信 + 网关下发用）
type IMessage interface {
	IWireMessage
	Release()
}
