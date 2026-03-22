package znet

import (
	"bytes"
	"encoding/binary"
	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"io"
	"sync"
	"testing"
	"time"
)

// ================================================================
// NetMessage 单元测试
// ================================================================

func TestNetMessage_GetSet(t *testing.T) {
	msg := &NetMessage{}

	msg.SetMsgId(1001)
	if msg.GetMsgId() != 1001 {
		t.Errorf("expected MsgId=1001, got %d", msg.GetMsgId())
	}

	msg.SetSeqId(42)
	if msg.GetSeqId() != 42 {
		t.Errorf("expected SeqId=42, got %d", msg.GetSeqId())
	}

	data := []byte("hello")
	msg.SetMessageData(data)
	if string(msg.GetMessageData()) != "hello" {
		t.Errorf("expected data='hello', got '%s'", string(msg.GetMessageData()))
	}
}

func TestNetMessage_Reset(t *testing.T) {
	msg := &NetMessage{
		MsgId: 100,
		SeqId: 200,
		Data:  []byte("test"),
	}

	msg.Reset()

	if msg.MsgId != 0 {
		t.Errorf("expected MsgId=0 after reset, got %d", msg.MsgId)
	}
	if msg.SeqId != 0 {
		t.Errorf("expected SeqId=0 after reset, got %d", msg.SeqId)
	}
	if msg.Data != nil {
		t.Errorf("expected Data=nil after reset, got %v", msg.Data)
	}
}

// TestNetMessage_Reset_AfterSetDataCopy_NoLeak 回归：Reset 必须释放 ownedBuf，否则先 SetDataCopy 再 Reset 会泄漏。
func TestNetMessage_Reset_AfterSetDataCopy_NoLeak(t *testing.T) {
	data := []byte("payload")
	// 单次流程：SetDataCopy -> Reset，确保 Reset 会清理 Data/ownedBuf 不泄漏。
	msg := GetNetMessage()
	msg.SetDataCopy(data)
	msg.Reset()
	if msg.Data != nil {
		t.Errorf("after Reset: expected Data=nil, got %v", msg.Data)
	}
	// 多次循环：Get -> SetDataCopy -> Reset -> SetDataCopy -> Release
	// 目的：验证不会 double-free、不会 panic，且 Reset 后重新使用是安全的。
	for i := 0; i < 1000; i++ {
		m := GetNetMessage()
		m.SetDataCopy(data)
		m.Reset()
		m.SetDataCopy(data)
		m.Release()
	}
}

func TestNetMessage_Clone_WithData(t *testing.T) {
	original := &NetMessage{
		MsgId: 1001,
		SeqId: 42,
		Data:  []byte("payload"),
	}

	clonedMsg := original.Clone()

	// 验证值相等
	if clonedMsg.MsgId != original.MsgId {
		t.Errorf("expected MsgId=%d, got %d", original.MsgId, clonedMsg.MsgId)
	}
	if clonedMsg.SeqId != original.SeqId {
		t.Errorf("expected SeqId=%d, got %d", original.SeqId, clonedMsg.SeqId)
	}
	if string(clonedMsg.Data) != string(original.Data) {
		t.Errorf("expected Data='%s', got '%s'", string(original.Data), string(clonedMsg.Data))
	}

	// 验证数据独立性（深拷贝）
	original.Data[0] = 'X'
	if clonedMsg.Data[0] == 'X' {
		t.Error("Clone should produce independent copy, but data is shared")
	}
}

func TestNetMessage_Clone_EmptyData(t *testing.T) {
	original := &NetMessage{
		MsgId: 500,
		SeqId: 10,
		Data:  nil,
	}

	clonedMsg := original.Clone()

	if clonedMsg.MsgId != 500 {
		t.Errorf("expected MsgId=500, got %d", clonedMsg.MsgId)
	}
	if clonedMsg.Data != nil {
		t.Error("expected nil Data for clone of empty message")
	}
}

func TestNetMessage_Clone_ZeroLengthData(t *testing.T) {
	original := &NetMessage{
		MsgId: 100,
		Data:  []byte{},
	}

	cloned := original.Clone()
	if cloned.Data != nil {
		t.Error("expected nil Data for clone of zero-length data")
	}
}

// ================================================================
// Message Pool 单元测试
// ================================================================

func TestNetMessage_SetDataCopy_ZeroAlloc(t *testing.T) {
	data := make([]byte, 128)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 100; i++ {
		msg := GetNetMessage()
		msg.SetDataCopy(data)
		msg.Release()
	}
	allocs := testing.AllocsPerRun(1000, func() {
		msg := GetNetMessage()
		msg.SetMsgId(100)
		msg.SetDataCopy(data)
		msg.Release()
	})
	if allocs > 0 {
		t.Errorf("SetDataCopy(128B): got %.0f allocs/op, want 0", allocs)
	}
}

func TestMessagePool_GetPut(t *testing.T) {
	msg := GetNetMessage()
	if msg == nil {
		t.Fatal("GetMessage returned nil")
	}

	// 设置数据后归还
	msg.SetMsgId(100)
	msg.SetMessageData([]byte("test"))
	msg.Release()

	// 重新获取，应该被 Reset 过
	msg2 := GetNetMessage()
	if msg2.GetMsgId() != 0 {
		t.Errorf("expected MsgId=0 after pool recycle, got %d", msg2.GetMsgId())
	}
	if msg2.GetMessageData() != nil {
		t.Errorf("expected nil Data after pool recycle, got %v", msg2.GetMessageData())
	}
	msg2.Release()
}

func TestParseDataPool_GetPut(t *testing.T) {
	pd := GetParseData()
	if pd == nil {
		t.Fatal("GetParseData returned nil")
	}
	if pd.Message == nil {
		t.Error("ParseData.message should not be nil after Get")
	}

	PutParseData(pd)
}

func TestParseDataPool_PutNil(t *testing.T) {
	// PutParseData(nil) 不应 panic
	PutParseData(nil)
}

func TestParseDataPool_Concurrent(t *testing.T) {
	const goroutines = 100
	const iterations = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pd := GetParseData()
				if pd.Message == nil {
					t.Error("message is nil in concurrent Get")
					return
				}
				PutParseData(pd)
			}
		}()
	}
	wg.Wait()
}

func TestNewMessage_Alias(t *testing.T) {
	msg := GetNetMessage()
	if msg == nil {
		t.Fatal("NewMessage returned nil")
	}
	msg.Release()
}

// ================================================================
// RingBuffer Pool 单元测试
// ================================================================

func TestRingBufferPool_GetPut(t *testing.T) {
	rb := GetRingBuffer()
	if rb == nil {
		t.Fatal("GetRingBuffer returned nil")
	}
	if rb.Len() != 0 {
		t.Errorf("expected empty RingBuffer, got Len=%d", rb.Len())
	}

	// 写入一些数据
	rb.Write([]byte("test"))
	PutRingBuffer(rb)

	// 重新获取，应该被 Reset 过（含 Stats 与底层字节清零）
	rb2 := GetRingBuffer()
	if rb2.Len() != 0 {
		t.Errorf("expected empty RingBuffer after pool recycle, got Len=%d", rb2.Len())
	}
	tr, tw := rb2.Stats()
	if tr != 0 || tw != 0 {
		t.Errorf("expected Stats zero after pool Get, got tr=%d tw=%d", tr, tw)
	}
	PutRingBuffer(rb2)
}

func TestRingBufferPool_PutNil(t *testing.T) {
	PutRingBuffer(nil) // 不应 panic
}

func TestRingBufferPool_NonStandardSize(t *testing.T) {
	// 非标准大小的 RingBuffer 不会被归还到池
	rb := NewRingBuffer(RingBufferConfig{Size: 8192})
	PutRingBuffer(rb) // 应该被 GC 而不是归还到池
}

// ================================================================
// BaseSocket 单元测试
// ================================================================

func TestBaseSocket_DefaultConfig(t *testing.T) {
	socket := NewBaseSocket()
	if socket == nil {
		t.Fatal("NewBaseSocket returned nil")
	}
	if socket.config.MaxHeaderLength != DefaultMaxHeaderLength {
		t.Errorf("expected MaxHeaderLength=%d, got %d", DefaultMaxHeaderLength, socket.config.MaxHeaderLength)
	}
	if socket.config.MaxDataLength != DefaultMaxDataLength {
		t.Errorf("expected MaxDataLength=%d, got %d", DefaultMaxDataLength, socket.config.MaxDataLength)
	}
	if socket.config.MaxMsgId != DefaultMaxMsgId {
		t.Errorf("expected MaxMsgId=%d, got %d", DefaultMaxMsgId, socket.config.MaxMsgId)
	}
}

func TestBaseSocket_CustomConfig(t *testing.T) {
	cfg := SocketConfig{
		MaxHeaderLength: 1024,
		MaxDataLength:   2048,
		MaxMsgId:        100,
	}
	socket := NewBaseSocket(cfg)
	if socket.config.MaxMsgId != 100 {
		t.Errorf("expected MaxMsgId=100, got %d", socket.config.MaxMsgId)
	}
}

func TestBaseSocket_ParseFromRingBuffer_SingleMessage(t *testing.T) {
	socket := NewBaseSocket()
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// 构建消息: msgId=1001, seqId=42, data="hello"
	data := []byte("hello")
	msg := makeTestPacket(1001, 42, data)
	rb.Write(msg)

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("ParseFromRingBuffer error: %v", err)
	}
	if !parsed {
		t.Fatal("expected parsed=true")
	}
	if pd.Message.GetMsgId() != 1001 {
		t.Errorf("expected MsgId=1001, got %d", pd.Message.GetMsgId())
	}
	if pd.Message.GetSeqId() != 42 {
		t.Errorf("expected SeqId=42, got %d", pd.Message.GetSeqId())
	}
	if string(pd.Message.GetMessageData()) != "hello" {
		t.Errorf("expected data='hello', got '%s'", string(pd.Message.GetMessageData()))
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_EmptyData(t *testing.T) {
	socket := NewBaseSocket()
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// 消息体为空
	msg := makeTestPacket(500, 0, nil)
	rb.Write(msg)

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("ParseFromRingBuffer error: %v", err)
	}
	if !parsed {
		t.Fatal("expected parsed=true for empty data message")
	}
	if pd.Message.GetMsgId() != 500 {
		t.Errorf("expected MsgId=500, got %d", pd.Message.GetMsgId())
	}
	if pd.Message.GetMessageData() != nil {
		t.Errorf("expected nil data, got %v", pd.Message.GetMessageData())
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_InsufficientHeader(t *testing.T) {
	socket := NewBaseSocket()
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// 只写入 8 字节（header 需要 12）
	rb.Write([]byte{0, 0, 0, 1, 0, 0, 0, 2})

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed {
		t.Error("expected parsed=false for insufficient header")
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_InsufficientBody(t *testing.T) {
	socket := NewBaseSocket()
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// header 声明 dataLen=100，但实际只提供 5 字节
	var header [12]byte
	binary.BigEndian.PutUint32(header[0:4], 1001) // msgId
	binary.BigEndian.PutUint32(header[4:8], 42)   // seqId
	binary.BigEndian.PutUint32(header[8:12], 100) // dataLen=100
	rb.Write(header[:])
	rb.Write([]byte("hello")) // 只有 5 字节

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed {
		t.Error("expected parsed=false for insufficient body")
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_InvalidMsgId(t *testing.T) {
	socket := NewBaseSocket(SocketConfig{
		MaxHeaderLength: DefaultMaxHeaderLength,
		MaxDataLength:   DefaultMaxDataLength,
		MaxMsgId:        100, // 最大消息 ID 为 100
	})
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// msgId=999 超过限制
	msg := makeTestPacket(999, 0, nil)
	rb.Write(msg)

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err == nil {
		t.Error("expected error for invalid msgId")
	}
	if parsed {
		t.Error("expected parsed=false for invalid msgId")
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_InvalidDataLength(t *testing.T) {
	socket := NewBaseSocket(SocketConfig{
		MaxHeaderLength: DefaultMaxHeaderLength,
		MaxDataLength:   10, // 最大数据长度 10
		MaxMsgId:        DefaultMaxMsgId,
	})
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// 声明 dataLen=100 超过限制
	var header [12]byte
	binary.BigEndian.PutUint32(header[0:4], 1)    // msgId
	binary.BigEndian.PutUint32(header[4:8], 0)    // seqId
	binary.BigEndian.PutUint32(header[8:12], 100) // dataLen=100 > MaxDataLength=10
	rb.Write(header[:])

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err == nil {
		t.Error("expected error for oversized data length")
	}
	if parsed {
		t.Error("expected parsed=false for oversized data")
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_MultipleMessages(t *testing.T) {
	socket := NewBaseSocket()
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})

	// 写入 3 条消息
	for i := 0; i < 3; i++ {
		msg := makeTestPacket(uint32(100+i), uint32(i), []byte("msg"))
		rb.Write(msg)
	}

	// 依次解析
	for i := 0; i < 3; i++ {
		pd := GetParseData()
		parsed, err := socket.ParseFromRingBuffer(rb, pd)
		if err != nil {
			t.Fatalf("ParseFromRingBuffer[%d] error: %v", i, err)
		}
		if !parsed {
			t.Fatalf("expected parsed=true for message %d", i)
		}
		if pd.Message.GetMsgId() != int32(100+i) {
			t.Errorf("expected MsgId=%d, got %d", 100+i, pd.Message.GetMsgId())
		}
		PutParseData(pd)
	}

	// 第 4 次应返回 false（没有更多消息）
	pd := GetParseData()
	parsed, _ := socket.ParseFromRingBuffer(rb, pd)
	if parsed {
		t.Error("expected parsed=false after all messages consumed")
	}
	PutParseData(pd)
}

func TestBaseSocket_ParseFromRingBuffer_WrapAround(t *testing.T) {
	socket := NewBaseSocket()
	// 使用小缓冲区，强制数据环绕
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})

	// 写入并消费一些数据，使 write 指针接近缓冲区末尾
	filler := make([]byte, 20)
	rb.Write(filler)
	rb.Discard(20)

	// 此时 write 指针在 offset 20，再写入一个跨边界的消息
	data := []byte("AB")
	msg := makeTestPacket(777, 1, data) // 12 + 2 = 14 字节，从 offset 20 写，跨到 offset 2
	rb.Write(msg)

	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("ParseFromRingBuffer error: %v", err)
	}
	if !parsed {
		t.Fatal("expected parsed=true for wrap-around message")
	}
	if pd.Message.GetMsgId() != 777 {
		t.Errorf("expected MsgId=777, got %d", pd.Message.GetMsgId())
	}
	// 跨边界时数据需要合并拷贝
	if string(pd.Message.GetMessageData()) != "AB" {
		t.Errorf("expected data='AB', got '%s'", string(pd.Message.GetMessageData()))
	}
	PutParseData(pd)
}

func TestBaseSocket_PreparePacket(t *testing.T) {
	socket := NewBaseSocket()
	msg := &NetMessage{
		MsgId: 1001,
		SeqId: 42,
		Data:  []byte("hello"),
	}

	var headerBuf [12]byte
	headerLen, body := socket.PreparePacket(msg, headerBuf[:])

	if headerLen != 12 {
		t.Errorf("expected headerLen=12, got %d", headerLen)
	}

	// 验证 header
	gotMsgId := binary.BigEndian.Uint32(headerBuf[0:4])
	gotSeqId := binary.BigEndian.Uint32(headerBuf[4:8])
	gotDataLen := binary.BigEndian.Uint32(headerBuf[8:12])

	if gotMsgId != 1001 {
		t.Errorf("expected msgId=1001, got %d", gotMsgId)
	}
	if gotSeqId != 42 {
		t.Errorf("expected seqId=42, got %d", gotSeqId)
	}
	if gotDataLen != 5 {
		t.Errorf("expected dataLen=5, got %d", gotDataLen)
	}

	if string(body) != "hello" {
		t.Errorf("expected body='hello', got '%s'", string(body))
	}
}

func TestBaseSocket_PreparePacket_EmptyBody(t *testing.T) {
	socket := NewBaseSocket()
	msg := &NetMessage{MsgId: 100, SeqId: 0}

	var headerBuf [12]byte
	headerLen, body := socket.PreparePacket(msg, headerBuf[:])

	if headerLen != 12 {
		t.Errorf("expected headerLen=12, got %d", headerLen)
	}

	gotDataLen := binary.BigEndian.Uint32(headerBuf[8:12])
	if gotDataLen != 0 {
		t.Errorf("expected dataLen=0, got %d", gotDataLen)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body, got %v", body)
	}
}

func TestBaseSocket_RoundTrip(t *testing.T) {
	socket := NewBaseSocket()

	// 封包
	original := &NetMessage{MsgId: 9999, SeqId: 12345, Data: []byte("round trip test")}
	var headerBuf [12]byte
	headerLen, body := socket.PreparePacket(original, headerBuf[:])

	// 将封包结果写入 RingBuffer
	rb := NewRingBuffer(RingBufferConfig{Size: 4096})
	rb.Write(headerBuf[:headerLen])
	rb.Write(body)

	// 解包
	pd := GetParseData()
	parsed, err := socket.ParseFromRingBuffer(rb, pd)
	if err != nil {
		t.Fatalf("RoundTrip parse error: %v", err)
	}
	if !parsed {
		t.Fatal("expected parsed=true")
	}
	if pd.Message.GetMsgId() != 9999 {
		t.Errorf("expected MsgId=9999, got %d", pd.Message.GetMsgId())
	}
	if pd.Message.GetSeqId() != 12345 {
		t.Errorf("expected SeqId=12345, got %d", pd.Message.GetSeqId())
	}
	if string(pd.Message.GetMessageData()) != "round trip test" {
		t.Errorf("expected data='round trip test', got '%s'", string(pd.Message.GetMessageData()))
	}
	PutParseData(pd)
}

// ================================================================
// AdaptiveWriter 单元测试
// ================================================================

func TestAdaptiveWriter_DirectWrite(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	data := []byte("hello world")
	n, err := aw.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected n=%d, got %d", len(data), n)
	}

	// TierNone 模式下直写
	if aw.GetTier() != ziface.TierNone {
		t.Errorf("expected TierNone, got %d", aw.GetTier())
	}
	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", buf.String())
	}
}

func TestAdaptiveWriter_Available_NoBuffer(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	if aw.Available() != 0 {
		t.Errorf("expected Available=0 for TierNone, got %d", aw.Available())
	}
	if aw.Buffered() != 0 {
		t.Errorf("expected Buffered=0 for TierNone, got %d", aw.Buffered())
	}
}

func TestAdaptiveWriter_Close(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.Write([]byte("data"))
	err := aw.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// 关闭后 tier 应重置
	if aw.GetTier() != ziface.TierNone {
		t.Errorf("expected TierNone after Close, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_DoubleClose(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.Close()
	err := aw.Close()
	if err != nil {
		t.Fatalf("Double close should not error, got %v", err)
	}
}

func TestAdaptiveWriter_Reset(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	aw := NewAdaptiveWriter(&buf1)

	aw.Write([]byte("first"))
	aw.Reset(&buf2)
	aw.Write([]byte("second"))

	if buf1.String() != "first" {
		t.Errorf("expected buf1='first', got '%s'", buf1.String())
	}
	if buf2.String() != "second" {
		t.Errorf("expected buf2='second', got '%s'", buf2.String())
	}
	if aw.GetTier() != ziface.TierNone {
		t.Errorf("expected TierNone after Reset, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_TierUpgrade(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	// 手动设置 lastCheck 为过去，触发 tryAdapt
	aw.lastCheck = time.Now().Add(-3 * time.Second)
	aw.writeCount = freqHigh // 超过高频阈值

	aw.Write([]byte("x")) // 触发 tryAdapt
	if aw.GetTier() != ziface.TierLarge {
		t.Errorf("expected TierLarge after high frequency, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_TierNames(t *testing.T) {
	tests := []struct {
		tier     ziface.BufferTier
		expected ziface.BufferTier
	}{
		{ziface.TierNone, 0},
		{ziface.TierSmall, 1},
		{ziface.TierMedium, 2},
		{ziface.TierLarge, 3},
	}
	for _, tt := range tests {
		if tt.tier != tt.expected {
			t.Errorf("expected %d, got %d", tt.expected, tt.tier)
		}
	}
}

func TestAdaptiveWriter_FlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	err := aw.Flush()
	if err != nil {
		t.Fatalf("Flush on empty writer should not error, got %v", err)
	}
}

// 模拟短写入的 writer
type shortWriter struct {
	maxPerWrite int
	buf         bytes.Buffer
}

func (sw *shortWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > sw.maxPerWrite {
		n = sw.maxPerWrite
	}
	return sw.buf.Write(p[:n])
}

func TestAdaptiveWriter_BufferedWrite(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	// 手动升级到 TierSmall（2KB 缓冲区）
	aw.resizeTier(ziface.TierSmall)
	if aw.GetTier() != ziface.TierSmall {
		t.Fatalf("expected TierSmall, got %d", aw.GetTier())
	}

	// 写入小数据（应缓冲）
	data := []byte("buffered data")
	n, err := aw.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected n=%d, got %d", len(data), n)
	}
	if aw.Buffered() != len(data) {
		t.Errorf("expected Buffered=%d, got %d", len(data), aw.Buffered())
	}
	if buf.Len() != 0 {
		t.Error("expected no output before Flush")
	}

	// Flush 后输出
	err = aw.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	if buf.String() != "buffered data" {
		t.Errorf("expected 'buffered data', got '%s'", buf.String())
	}

	aw.Close()
}

// ================================================================
// BaseServer 单元测试
// ================================================================

func TestBaseServer_New(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	if s == nil {
		t.Fatal("NewBaseServer returned nil")
	}
	if s.addr != ":8888" {
		t.Errorf("expected addr=':8888', got '%s'", s.addr)
	}
	if s.iEncrypt == nil {
		t.Error("expected non-nil encrypt")
	}
}

func TestBaseServer_NextId(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	id1 := s.NextId()
	id2 := s.NextId()
	if id1 >= id2 {
		t.Errorf("expected id1 < id2, got %d >= %d", id1, id2)
	}
}

func TestBaseServer_NextId_Concurrent(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	const goroutines = 100
	const iterations = 100

	ids := make(chan uint64, goroutines*iterations)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ids <- s.NextId()
			}
		}()
	}
	wg.Wait()
	close(ids)

	// 验证唯一性
	seen := make(map[uint64]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id: %d", id)
		}
		seen[id] = true
	}
	if len(seen) != goroutines*iterations {
		t.Errorf("expected %d unique ids, got %d", goroutines*iterations, len(seen))
	}
}

func TestBaseServer_NilHandlersPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when OnAccept is nil")
		}
	}()
	NewBaseServer(":8888", ServerHandlers{OnAccept: nil, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
}

func TestBaseServer_NilReadHandlerPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when OnRead is nil")
		}
	}()
	NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: nil})
}

func TestBaseServer_HandlersCalledOnEvents(t *testing.T) {
	acceptCalled := false
	readCalled := false
	s := NewBaseServer(":8888", ServerHandlers{
		OnAccept: func(ch ziface.IChannel) bool { acceptCalled = true; return true },
		OnRead:   func(ch ziface.IChannel, msg ziface.IWireMessage) { readCalled = true },
	})

	result := s.HandleAccept(nil)
	if !acceptCalled {
		t.Error("OnAccept callback not called")
	}
	if !result {
		t.Error("expected HandleAccept to return true")
	}

	s.HandleRead(nil, nil)
	if !readCalled {
		t.Error("OnRead callback not called")
	}
}

func TestBaseServer_GetChannel_NotFound(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ch := s.GetChannel(9999)
	if ch != nil {
		t.Error("expected nil for non-existent channel")
	}
}

func TestBaseServer_GetChannelByAuthId_NotFound(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ch := s.GetChannelByAuthId(12345)
	if ch != nil {
		t.Error("expected nil for non-existent auth channel")
	}
}

func TestBaseServer_GetEncrypt(t *testing.T) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	s.SetEncrypt(zencrypt.NewSM4Encrypt("aaa"))
	enc := s.GetEncrypt()
	if enc == nil {
		t.Fatal("GetEncrypt returned nil")
	}

	// 默认 encrypt 为 SM4-GCM，验证加解密 round-trip
	data := []byte("test")
	encrypted, err := enc.Encrypt(data)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if string(encrypted) == "test" {
		t.Error("default encrypt should not be passthrough (SM4 enabled)")
	}
	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if string(decrypted) != "test" {
		t.Errorf("round-trip failed: got %q, want %q", decrypted, "test")
	}
}

// ================================================================
// SocketConfig 单元测试
// ================================================================

func TestDefaultSocketConfig(t *testing.T) {
	cfg := DefaultSocketConfig()
	if cfg.MaxHeaderLength != 10*1024 {
		t.Errorf("expected MaxHeaderLength=10240, got %d", cfg.MaxHeaderLength)
	}
	if cfg.MaxDataLength != 1024*1024 {
		t.Errorf("expected MaxDataLength=1048576, got %d", cfg.MaxDataLength)
	}
	if cfg.MaxMsgId != DefaultMaxMsgId {
		t.Errorf("expected MaxMsgId=%d, got %d", DefaultMaxMsgId, cfg.MaxMsgId)
	}
}

// ================================================================
// NetOperator 单元测试
// ================================================================

func TestNetOperator_Fields(t *testing.T) {
	msg := &NetMessage{MsgId: 1}
	op := NetOperator{
		Op:        1,
		ServiceId: 100,
		ChannelId: 200,
		Message:   msg,
		Args:      "test",
	}

	if op.Op != 1 {
		t.Errorf("expected Op=1, got %d", op.Op)
	}
	if op.ServiceId != 100 {
		t.Errorf("expected ServiceId=100, got %d", op.ServiceId)
	}
	if op.ChannelId != 200 {
		t.Errorf("expected ChannelId=200, got %d", op.ChannelId)
	}
	if op.Message.GetMsgId() != 1 {
		t.Errorf("expected MsgId=1, got %d", op.Message.GetMsgId())
	}
}

// ================================================================
// ConnProtocol 单元测试
// ================================================================

func TestConnProtocol_Values(t *testing.T) {
	if TCP != 1 {
		t.Errorf("expected TCP=1, got %d", TCP)
	}
	if KCP != 2 {
		t.Errorf("expected KCP=2, got %d", KCP)
	}
	if WebSocket != 3 {
		t.Errorf("expected WebSocket=3, got %d", WebSocket)
	}
}

// ================================================================
// 辅助函数
// ================================================================

// makeTestPacket 构建协议包 (msgId(4) + seqId(4) + dataLen(4) + data)
func makeTestPacket(msgId, seqId uint32, data []byte) []byte {
	buf := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(buf[0:4], msgId)
	binary.BigEndian.PutUint32(buf[4:8], seqId)
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(data)))
	if len(data) > 0 {
		copy(buf[12:], data)
	}
	return buf
}

// ================================================================
// 基准测试
// ================================================================

func BenchmarkNetMessage_Clone(b *testing.B) {
	msg := &NetMessage{
		MsgId: 1001,
		SeqId: 42,
		Data:  make([]byte, 256),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = msg.Clone()
	}
}

func BenchmarkMessagePool_GetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := GetNetMessage()
		msg.Release()
	}
}

func BenchmarkMessagePool_GetPut_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := GetNetMessage()
			msg.Release()
		}
	})
}

func BenchmarkParseDataPool_GetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pd := GetParseData()
		PutParseData(pd)
	}
}

func BenchmarkBaseSocket_PreparePacket(b *testing.B) {
	socket := NewBaseSocket()
	msg := &NetMessage{
		MsgId: 1001,
		SeqId: 42,
		Data:  make([]byte, 128),
	}
	var headerBuf [12]byte

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		socket.PreparePacket(msg, headerBuf[:])
	}
}

func BenchmarkAdaptiveWriter_DirectWrite(b *testing.B) {
	aw := NewAdaptiveWriter(io.Discard)
	data := make([]byte, 64)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aw.Write(data)
	}
}

func BenchmarkBaseServer_NextId(b *testing.B) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.NextId()
	}
}

func BenchmarkBaseServer_NextId_Parallel(b *testing.B) {
	s := NewBaseServer(":8888", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.NextId()
		}
	})
}
