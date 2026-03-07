package znet

import "github.com/aiyang-zh/zhenyi-core/zpool"

// ================================================================
// ParseData — 协议解析结果容器
// ================================================================

type ParseData struct {
	Message      *NetMessage
	Error        error
	OwnedBuffers []*zpool.Buffer
}

// ResetForReuse 复用前重置（释放上一轮持有的 buffer，重置 message）
func (p *ParseData) ResetForReuse() {
	for _, buf := range p.OwnedBuffers {
		buf.Release()
	}
	p.OwnedBuffers = p.OwnedBuffers[:0]
	p.Message.Reset()
	p.Error = nil
}

// ================================================================
// NetMessage — 线协议消息（实现 IWireMessage + IMessage）
// ================================================================
//
// NetMessage 是网络层的最小消息单元，只包含线协议所需的三个字段。
// 业务层的 Message（proto）通过 Marshal/Unmarshal 序列化到 NetMessage.Data 中。
//
// 生命周期管理：
//   - 通过 GetNetMessage() 从池获取时，pooled=true，Release() 归还到池
//   - 嵌入在 BaseChannel 中时，pooled=false（默认），Release() 为 no-op

type NetMessage struct {
	MsgId    int32         // 消息 ID（与 proto Message.MsgId 类型一致）
	SeqId    uint32        // 序列号
	Data     []byte        // 消息体
	pooled   bool          // 是否从池获取（控制 Release 行为）
	ownedBuf *zpool.Buffer // 发送路径持有的序列化 buffer，Release 时归还 bytes pool
}

// ---- IWireMessage 实现 ----

func (b *NetMessage) GetMsgId() int32            { return b.MsgId }
func (b *NetMessage) SetMsgId(msgId int32)       { b.MsgId = msgId }
func (b *NetMessage) GetSeqId() uint32           { return b.SeqId }
func (b *NetMessage) SetSeqId(seqId uint32)      { b.SeqId = seqId }
func (b *NetMessage) GetMessageData() []byte     { return b.Data }
func (b *NetMessage) SetMessageData(data []byte) { b.Data = data }

func (b *NetMessage) Reset() {
	b.MsgId = 0
	b.Data = nil
	b.SeqId = 0
}

// ---- IMessage 实现 ----

// Release 归还消息到池。
//   - 如果持有 ownedBuf（MarshalToWire 产生），先归还 bytes pool
//   - 如果 pooled=true（GetNetMessage 产生），归还 NetMessage 到池
//   - 嵌入在 BaseChannel 中的消息两者皆 false，Release 为 no-op
func (b *NetMessage) Release() {
	if b.ownedBuf != nil {
		b.ownedBuf.Release()
		b.ownedBuf = nil
	}
	if b.pooled {
		b.pooled = false
		b.Reset()
		netMessagePool.Put(b)
	}
}

// ---- Clone（具体类型方法，不在接口中）----

// Clone 深拷贝消息。返回的消息 pooled=false，不受池管理。
// 如需池管理的拷贝，使用 GetNetMessage() 后手动赋值。
func (b *NetMessage) Clone() *NetMessage {
	if len(b.Data) == 0 {
		return &NetMessage{
			MsgId: b.MsgId,
			SeqId: b.SeqId,
		}
	}
	buf := make([]byte, len(b.Data))
	copy(buf, b.Data)

	return &NetMessage{
		MsgId: b.MsgId,
		Data:  buf,
		SeqId: b.SeqId,
	}
}

// ================================================================
// NetMessage Pool
// ================================================================

var netMessagePool = zpool.NewPool(func() *NetMessage {
	return &NetMessage{}
})

// GetNetMessage 从池获取 NetMessage（pooled=true，Release 归还到池）
func GetNetMessage() *NetMessage {
	msg := netMessagePool.Get()
	msg.Reset()
	msg.pooled = true
	return msg
}
