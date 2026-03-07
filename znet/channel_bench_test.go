package znet

import (
	"bytes"
	"encoding/binary"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"io"
	"testing"
)

// ================================================================
// 基准测试：对比传统方式 vs 零拷贝方式的核心差异
// ================================================================
//
// 运行命令:
//   go test -bench="Channel" -benchmem -run=^$ ./...
//
// 关键指标:
//   - ns/op:     每次操作耗时
//   - B/op:      每次操作分配的内存
//   - allocs/op: 每次操作的内存分配次数

// 模拟消息：msgId(4) + seqId(4) + dataLen(4) + data
func makeTestMessage(msgId, seqId uint32, dataSize int) []byte {
	buf := make([]byte, 12+dataSize)
	binary.BigEndian.PutUint32(buf[0:4], msgId)
	binary.BigEndian.PutUint32(buf[4:8], seqId)
	binary.BigEndian.PutUint32(buf[8:12], uint32(dataSize))
	for i := 0; i < dataSize; i++ {
		buf[12+i] = byte(i % 256)
	}
	return buf
}

// ================================================================
// 1. 单条消息解析对比
// ================================================================

// BenchmarkBaseChannel_ParseSingle 模拟 BaseChannel 的解析方式
// 每条消息 2 次 io.ReadFull + 2 次内存分配
func BenchmarkBaseChannel_ParseSingle(b *testing.B) {
	msg := makeTestMessage(1001, 1, 128)
	reader := bytes.NewReader(msg)
	headBuf := make([]byte, 12)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader.Reset(msg)

		// 第 1 次 ReadFull: 读取头部
		_, _ = io.ReadFull(reader, headBuf)

		dataLen := int(binary.BigEndian.Uint32(headBuf[8:12]))

		// 第 2 次分配 + ReadFull: 读取数据
		dataBuf := zpool.GetBytesBuffer(dataLen)
		_, _ = io.ReadFull(reader, dataBuf.B)
		dataBuf.Release()
	}
}

// BenchmarkZeroCopyChannel_ParseSingle 零拷贝解析方式
// 数据连续时：0 次分配，只有 Peek 操作
func BenchmarkZeroCopyChannel_ParseSingle(b *testing.B) {
	msg := makeTestMessage(1001, 1, 128)
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()
		_, _ = rb.Write(msg)

		// 零拷贝 Peek 头部
		_, _ = rb.PeekUint32BE(0) // msgId
		_, _ = rb.PeekUint32BE(4) // seqId
		dataLen, _ := rb.PeekUint32BE(8)

		// 零拷贝获取数据（数据连续，不分配）
		first, second, _ := rb.PeekTwoSlices(12, int(dataLen))
		_ = first
		_ = second

		rb.Discard(12 + int(dataLen))
	}
}

// ================================================================
// 2. 批量消息解析对比（更能体现差异）
// ================================================================

// BenchmarkBaseChannel_ParseBatch 模拟 BaseChannel 批量解析
// N 条消息 = 2N 次 ReadFull + 2N 次分配
func BenchmarkBaseChannel_ParseBatch(b *testing.B) {
	// 准备 10 条消息
	var allMsgs []byte
	for i := 0; i < 10; i++ {
		allMsgs = append(allMsgs, makeTestMessage(uint32(1000+i), uint32(i), 64)...)
	}
	reader := bytes.NewReader(allMsgs)
	headBuf := make([]byte, 12)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader.Reset(allMsgs)

		for j := 0; j < 10; j++ {
			// 2 次 ReadFull per message
			_, _ = io.ReadFull(reader, headBuf)
			dataLen := int(binary.BigEndian.Uint32(headBuf[8:12]))
			dataBuf := zpool.GetBytesBuffer(dataLen)
			_, _ = io.ReadFull(reader, dataBuf.B)
			dataBuf.Release()
		}
	}
}

// BenchmarkZeroCopyChannel_ParseBatch 零拷贝批量解析
// N 条消息 = 1 次 Write + 0~N 次分配（取决于是否跨边界）
func BenchmarkZeroCopyChannel_ParseBatch(b *testing.B) {
	// 准备 10 条消息
	var allMsgs []byte
	for i := 0; i < 10; i++ {
		allMsgs = append(allMsgs, makeTestMessage(uint32(1000+i), uint32(i), 64)...)
	}
	rb := NewRingBuffer(RingBufferConfig{Size: 8192})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()

		// 1 次批量写入
		_, _ = rb.Write(allMsgs)

		// 循环解析
		for j := 0; j < 10; j++ {
			_, _ = rb.PeekUint32BE(0)
			_, _ = rb.PeekUint32BE(4)
			dataLen, _ := rb.PeekUint32BE(8)

			first, second, _ := rb.PeekTwoSlices(12, int(dataLen))
			_ = first
			_ = second

			rb.Discard(12 + int(dataLen))
		}
	}
}

// ================================================================
// 3. 完整解析流程（使用真实 BaseSocket）
// ================================================================

// BenchmarkBaseSocket_ParseFromRingBuffer 使用 BaseSocket.ParseFromRingBuffer
func BenchmarkBaseSocket_ParseFromRingBuffer(b *testing.B) {
	msg := makeTestMessage(1001, 1, 256)
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})
	socket := NewBaseSocket()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()
		_, _ = rb.Write(msg)

		parseData := GetParseData()
		_, _ = socket.ParseFromRingBuffer(rb, parseData)
		PutParseData(parseData)
	}
}

// ================================================================
// 3b. 完整解析流程（复用 ParseData，零池化开销）
// ================================================================

// BenchmarkBaseSocket_ParseFromRingBuffer_Reuse 使用复用 ParseData 的路径
func BenchmarkBaseSocket_ParseFromRingBuffer_Reuse(b *testing.B) {
	msg := makeTestMessage(1001, 1, 256)
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})
	socket := NewBaseSocket()

	var parseMsg NetMessage
	pd := ParseData{
		Message:      &parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()
		_, _ = rb.Write(msg)

		pd.ResetForReuse()
		_, _ = socket.ParseFromRingBuffer(rb, &pd)
	}
}

// ================================================================
// 4. 不同消息大小测试
// ================================================================

func BenchmarkParse_SmallMsg(b *testing.B) {
	benchParseReuse(b, 32)
}

func BenchmarkParse_MediumMsg(b *testing.B) {
	benchParseReuse(b, 512)
}

func BenchmarkParse_LargeMsg(b *testing.B) {
	benchParseReuse(b, 4096)
}

// benchParseReuse 模拟 BaseChannel.read() 的复用路径
func benchParseReuse(b *testing.B, dataSize int) {
	msg := makeTestMessage(1001, 1, dataSize)
	rb := NewRingBuffer(RingBufferConfig{Size: max(dataSize*2, 4096)})
	socket := NewBaseSocket()

	var parseMsg NetMessage
	pd := ParseData{
		Message:      &parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()
		_, _ = rb.Write(msg)

		pd.ResetForReuse()
		_, _ = socket.ParseFromRingBuffer(rb, &pd)
	}
}

// benchParsePool 旧路径（对照组）
func benchParsePool(b *testing.B, dataSize int) {
	msg := makeTestMessage(1001, 1, dataSize)
	rb := NewRingBuffer(RingBufferConfig{Size: max(dataSize*2, 4096)})
	socket := NewBaseSocket()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Reset()
		_, _ = rb.Write(msg)

		parseData := GetParseData()
		_, _ = socket.ParseFromRingBuffer(rb, parseData)
		PutParseData(parseData)
	}
}

func BenchmarkParse_SmallMsg_Pool(b *testing.B) {
	benchParsePool(b, 32)
}

func BenchmarkParse_SmallMsg_Reuse(b *testing.B) {
	benchParseReuse(b, 32)
}

// ================================================================
// 5. 内存分配统计
// ================================================================

func TestAllocStats(t *testing.T) {
	msg := makeTestMessage(1001, 1, 256)

	// 零拷贝方式
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})
	socket := NewBaseSocket()
	zeroCopyAllocs := testing.AllocsPerRun(1000, func() {
		rb.Reset()
		_, _ = rb.Write(msg)
		parseData := GetParseData()
		_, _ = socket.ParseFromRingBuffer(rb, parseData)
		PutParseData(parseData)
	})

	t.Logf("零拷贝方式 分配次数: %.2f allocs/op", zeroCopyAllocs)
}

// ================================================================
// 6. 发送侧基准测试
// ================================================================

// BenchmarkSend_OldWay 模拟原来的发送方式（每条消息单独写 header）
func BenchmarkSend_OldWay(b *testing.B) {
	messages := make([]struct {
		MsgId uint32
		SeqId uint32
		Data  []byte
	}, 10)
	for i := range messages {
		messages[i].MsgId = uint32(1000 + i)
		messages[i].SeqId = uint32(i)
		messages[i].Data = make([]byte, 64)
	}

	buf := bytes.NewBuffer(make([]byte, 0, 4096))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		for _, msg := range messages {
			// 每次循环分配一个 header
			var header [12]byte
			binary.BigEndian.PutUint32(header[0:], msg.MsgId)
			binary.BigEndian.PutUint32(header[4:], msg.SeqId)
			binary.BigEndian.PutUint32(header[8:], uint32(len(msg.Data)))
			buf.Write(header[:])
			buf.Write(msg.Data)
		}
	}
}

// BenchmarkSend_ZeroCopy 零拷贝发送（预分配 header 数组）
func BenchmarkSend_ZeroCopy(b *testing.B) {
	messages := make([]struct {
		MsgId uint32
		SeqId uint32
		Data  []byte
	}, 10)
	for i := range messages {
		messages[i].MsgId = uint32(1000 + i)
		messages[i].SeqId = uint32(i)
		messages[i].Data = make([]byte, 64)
	}

	buf := bytes.NewBuffer(make([]byte, 0, 4096))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()

		// ✅ 预分配所有 header
		headerBuf := zpool.GetBytesBuffer(len(messages) * 12)

		for j, msg := range messages {
			offset := j * 12
			binary.BigEndian.PutUint32(headerBuf.B[offset:], msg.MsgId)
			binary.BigEndian.PutUint32(headerBuf.B[offset+4:], msg.SeqId)
			binary.BigEndian.PutUint32(headerBuf.B[offset+8:], uint32(len(msg.Data)))
			buf.Write(headerBuf.B[offset : offset+12])
			buf.Write(msg.Data)
		}

		headerBuf.Release()
	}
}
