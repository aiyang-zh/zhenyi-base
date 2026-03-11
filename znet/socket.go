package znet

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zpool"
)

// BaseSocket 协议解析器（零拷贝实现）
//
// 协议格式: msgId(4) + seqId(4) + dataLen(4) + data
// 总 header 长度: 12 字节
type BaseSocket struct {
	config SocketConfig
}

// NewBaseSocket 创建协议解析器；不传 config 时使用 DefaultSocketConfig。
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

// HeaderLen 返回当前协议头长度（字节），依 ProtocolVersion 为 12 或 13。
func (base *BaseSocket) HeaderLen() int {
	if base.config.ProtocolVersion >= 1 {
		return headerSizeV1
	}
	return headerSizeV0
}

// ParseFromRingBuffer 从 RingBuffer 零拷贝解析协议（主要方法）
//
// 返回值：
//   - (true, nil): 成功解析一条消息
//   - (false, nil): 数据不足，需要等待更多数据（不消费任何数据）
//   - (false, err): 协议错误
//
// ⚠️ 零拷贝注意：返回的 message.Data 可能直接引用 RingBuffer 内存
// 调用方必须在下次写入 RingBuffer 前完成数据处理
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

// ParseFromBufio 从 bufio.Reader 直接解析，无 RingBuffer 拷贝，适合 echo 等高吞吐场景。
// 返回 (true, nil) 表示成功解析；(false, nil) 表示数据不足；(false, err) 表示协议错误。
func (base *BaseSocket) ParseFromBufio(r *bufio.Reader, parseData *ParseData) (parsed bool, err error) {
	hLen := base.HeaderLen()
	if r.Buffered() < hLen {
		return false, nil
	}
	peeked, e := r.Peek(hLen)
	if e != nil || len(peeked) < hLen {
		return false, e
	}
	off := 0
	if base.config.ProtocolVersion >= 1 {
		if peeked[0] != base.config.ProtocolVersion {
			return false, zerrs.Newf(zerrs.ErrTypeValidation, "protocol version mismatch")
		}
		off = 1
	}
	msgId := binary.BigEndian.Uint32(peeked[off : off+4])
	seqId := binary.BigEndian.Uint32(peeked[off+4 : off+8])
	dataLen := binary.BigEndian.Uint32(peeked[off+8 : off+12])
	msgIdInt := int32(msgId)
	if int(msgIdInt) < -base.config.MaxMsgId || int(msgIdInt) > base.config.MaxMsgId {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid msgId: %d", msgIdInt)
	}
	dataLength := int(dataLen)
	if dataLength > base.config.MaxDataLength {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid data length: %d", dataLength)
	}
	totalLen := hLen + dataLength
	if r.Buffered() < totalLen {
		return false, nil
	}
	parseData.Message.SetMsgId(msgIdInt)
	parseData.Message.SetSeqId(seqId)
	if dataLength == 0 {
		parseData.Message.SetMessageData(nil)
	} else {
		full, _ := r.Peek(totalLen)
		// 引用 bufio 内部 buffer，调用方必须在下次 r.Read/Discard 前完成处理
		parseData.Message.SetMessageData(full[hLen:totalLen])
	}
	_, _ = r.Discard(totalLen)
	return true, nil
}

// ReadOneFromBufio 从 bufio 读取并解析一条消息（使用 io.ReadFull，无 Peek 引用）。
// 适用于需要独立 body 副本的场景，body 从 pool 获取。
func (base *BaseSocket) ReadOneFromBufio(r *bufio.Reader, parseData *ParseData) (parsed bool, err error) {
	hLen := base.HeaderLen()
	var header [13]byte
	if _, e := io.ReadFull(r, header[:hLen]); e != nil {
		if e == io.EOF {
			return false, nil
		}
		return false, e
	}
	off := 0
	if base.config.ProtocolVersion >= 1 {
		if header[0] != base.config.ProtocolVersion {
			return false, zerrs.Newf(zerrs.ErrTypeValidation, "protocol version mismatch")
		}
		off = 1
	}
	msgId := binary.BigEndian.Uint32(header[off : off+4])
	seqId := binary.BigEndian.Uint32(header[off+4 : off+8])
	dataLen := binary.BigEndian.Uint32(header[off+8 : off+12])
	msgIdInt := int32(msgId)
	if int(msgIdInt) < -base.config.MaxMsgId || int(msgIdInt) > base.config.MaxMsgId {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid msgId: %d", msgIdInt)
	}
	dataLength := int(dataLen)
	if dataLength > base.config.MaxDataLength {
		return false, zerrs.Newf(zerrs.ErrTypeValidation, "invalid data length: %d", dataLength)
	}
	parseData.Message.SetMsgId(msgIdInt)
	parseData.Message.SetSeqId(seqId)
	if dataLength == 0 {
		parseData.Message.SetMessageData(nil)
		return true, nil
	}
	buf := zpool.GetBytesBuffer(dataLength)
	parseData.OwnedBuffers = append(parseData.OwnedBuffers, buf)
	if _, e := io.ReadFull(r, buf.B[:dataLength]); e != nil {
		buf.Release()
		parseData.OwnedBuffers = parseData.OwnedBuffers[:len(parseData.OwnedBuffers)-1]
		return false, e
	}
	parseData.Message.SetMessageData(buf.B[:dataLength])
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
	return base.PreparePacketFromWire(message, headerBuf)
}

// PreparePacketFromWire 从 IWireMessage 构建发送包（供 WriteImmediate 等直写场景）
func (base *BaseSocket) PreparePacketFromWire(msg ziface.IWireMessage, headerBuf []byte) (hdrLen int, body []byte) {
	msgId := msg.GetMsgId()
	seqId := msg.GetSeqId()
	body = msg.GetMessageData()

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
