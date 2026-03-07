package znet

import (
	"encoding/binary"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"github.com/aiyang-zh/zhenyi-util/zerrs"
)

// BaseSocket 协议解析器（零拷贝实现）
//
// 协议格式: msgId(4) + seqId(4) + dataLen(4) + data
// 总 header 长度: 12 字节
type BaseSocket struct {
	config SocketConfig
}

func NewBaseSocket(config ...SocketConfig) *BaseSocket {
	cfg := DefaultSocketConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &BaseSocket{config: cfg}
}

const (
	headerSizeV0 = 12 // v0: msgId(4) + seqId(4) + dataLen(4)
	headerSizeV1 = 13 // v1: version(1) + msgId(4) + seqId(4) + dataLen(4)
	headerSize   = 12 // alias for backward compatibility
)

// ParseFromRingBuffer 从 RingBuffer 零拷贝解析协议（主要方法）
//
// 返回值：
//   - (true, nil): 成功解析一条消息
//   - (false, nil): 数据不足，需要等待更多数据（不消费任何数据）
//   - (false, err): 协议错误
//
// ⚠️ 零拷贝注意：返回的 message.Data 可能直接引用 RingBuffer 内存
// 调用方必须在下次写入 RingBuffer 前完成数据处理
func (base *BaseSocket) HeaderLen() int {
	if base.config.ProtocolVersion >= 1 {
		return headerSizeV1
	}
	return headerSizeV0
}

func (base *BaseSocket) ParseFromRingBuffer(rb *RingBuffer, parseData *ParseData) (parsed bool, err error) {
	hLen := base.HeaderLen()
	if rb.Len() < hLen {
		return false, nil
	}

	off := 0
	if base.config.ProtocolVersion >= 1 {
		ver, e := rb.PeekByte(0)
		if e != nil {
			return false, e
		}
		if ver != base.config.ProtocolVersion {
			return false, zerrs.Newf(zerrs.ErrTypeValidation, "protocol version mismatch: got %d, expect %d", ver, base.config.ProtocolVersion)
		}
		off = 1
	}

	msgId, seqId, dataLen := rb.PeekHeader12(off)

	// 安全验证
	msgIdInt := int32(msgId)
	if int(msgIdInt) < -base.config.MaxMsgId || int(msgIdInt) > base.config.MaxMsgId {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid msgId: %d (max: %d)", msgIdInt, base.config.MaxMsgId)
	}

	dataLength := int(dataLen)
	if dataLength > base.config.MaxDataLength {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid data length: %d (max: %d)", dataLength, base.config.MaxDataLength)
	}

	totalLen := hLen + dataLength
	if rb.Len() < totalLen {
		return false, nil // 需要更多数据
	}

	// 设置消息头
	parseData.Message.SetMsgId(msgIdInt)
	parseData.Message.SetSeqId(seqId)

	// 处理消息体
	if dataLength == 0 {
		parseData.Message.SetMessageData(nil)
	} else {
		first, second, err := rb.PeekTwoSlices(hLen, dataLength)
		if err != nil {
			return false, err
		}

		if second == nil {
			// ✅ 数据连续，真正零拷贝
			parseData.Message.SetMessageData(first)
		} else {
			// ⚠️ 数据跨边界，需要拷贝合并
			buf := zpool.GetBytesBuffer(dataLength)
			parseData.OwnedBuffers = append(parseData.OwnedBuffers, buf)
			copy(buf.B[:len(first)], first)
			copy(buf.B[len(first):], second)
			parseData.Message.SetMessageData(buf.B)
		}
	}

	// 消费数据
	_ = rb.Discard(totalLen)

	return true, nil
}

// PreparePacket 准备发送数据包（主要方法，配合 net.Buffers 使用）
//
// 参数:
//   - message: 待发送的消息
//   - headerBuf: 调用方提供的 header 缓冲区（至少 12 字节）
//
// 返回:
//   - hdrLen: 写入的 header 长度（v0=12, v1=13）
//   - body: 消息体数据（直接引用，不拷贝）
//
// 使用示例:
//
//	var header [13]byte
//	hdrLen, body := socket.PreparePacket(msg, header[:])
//	buffers := net.Buffers{header[:hdrLen], body}
//	buffers.WriteTo(conn)  // writev 系统调用
func (base *BaseSocket) PreparePacket(message *NetMessage, headerBuf []byte) (hdrLen int, body []byte) {
	msgId := message.GetMsgId()
	seqId := message.GetSeqId()
	body = message.GetMessageData()

	off := 0
	if base.config.ProtocolVersion >= 1 {
		headerBuf[0] = base.config.ProtocolVersion
		off = 1
	}
	binary.BigEndian.PutUint32(headerBuf[off:off+4], uint32(msgId))
	binary.BigEndian.PutUint32(headerBuf[off+4:off+8], seqId)
	binary.BigEndian.PutUint32(headerBuf[off+8:off+12], uint32(len(body)))

	return base.HeaderLen(), body
}
