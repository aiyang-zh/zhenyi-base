package znet

import (
	"bytes"
	"io"
	"testing"
)

func TestRingBuffer_Basic(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 测试初始状态
	if rb.Len() != 0 {
		t.Errorf("expected Len() = 0, got %d", rb.Len())
	}
	if rb.Cap() != 16 {
		t.Errorf("expected Cap() = 16, got %d", rb.Cap())
	}
	if !rb.IsEmpty() {
		t.Error("expected IsEmpty() = true")
	}

	// 写入数据
	data := []byte("hello")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 {
		t.Errorf("expected Write() = 5, got %d", n)
	}
	if rb.Len() != 5 {
		t.Errorf("expected Len() = 5, got %d", rb.Len())
	}

	// Peek 零拷贝读取
	peeked, err := rb.Peek(5)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if string(peeked) != "hello" {
		t.Errorf("expected Peek() = 'hello', got '%s'", string(peeked))
	}

	// Peek 后长度不变
	if rb.Len() != 5 {
		t.Errorf("expected Len() = 5 after Peek, got %d", rb.Len())
	}

	// Discard 丢弃数据
	err = rb.Discard(5)
	if err != nil {
		t.Fatalf("Discard failed: %v", err)
	}
	if rb.Len() != 0 {
		t.Errorf("expected Len() = 0 after Discard, got %d", rb.Len())
	}
}

func TestRingBuffer_WrapAround(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 写入 10 字节
	data1 := []byte("0123456789")
	rb.Write(data1)

	// 丢弃 8 字节
	rb.Discard(8)

	// 再写入 10 字节（会环绕）
	data2 := []byte("ABCDEFGHIJ")
	n, err := rb.Write(data2)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 10 {
		t.Errorf("expected Write() = 10, got %d", n)
	}

	// 验证可读长度
	if rb.Len() != 12 { // 2 + 10
		t.Errorf("expected Len() = 12, got %d", rb.Len())
	}

	// 读取验证
	result := make([]byte, 12)
	err = rb.ReadFull(result)
	if err != nil {
		t.Fatalf("ReadFull failed: %v", err)
	}
	expected := "89ABCDEFGHIJ"
	if string(result) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(result))
	}
}

func TestRingBuffer_ReadUint32BE(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 写入大端序 uint32: 0x12345678
	data := []byte{0x12, 0x34, 0x56, 0x78}
	rb.Write(data)

	v, err := rb.ReadUint32BE()
	if err != nil {
		t.Fatalf("ReadUint32BE failed: %v", err)
	}
	if v != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08x", v)
	}
}

func TestRingBuffer_PeekUint32BE(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 写入数据
	data := []byte{0x00, 0x00, 0x00, 0x10, 0xAA, 0xBB, 0xCC, 0xDD}
	rb.Write(data)

	// Peek 第一个 uint32
	v1, _ := rb.PeekUint32BE(0)
	if v1 != 16 {
		t.Errorf("expected 16, got %d", v1)
	}

	// Peek 第二个 uint32
	v2, _ := rb.PeekUint32BE(4)
	if v2 != 0xAABBCCDD {
		t.Errorf("expected 0xAABBCCDD, got 0x%08x", v2)
	}

	// 长度不变
	if rb.Len() != 8 {
		t.Errorf("expected Len() = 8, got %d", rb.Len())
	}
}

func TestRingBuffer_PeekHeader12(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	data := []byte{
		0x00, 0x00, 0x03, 0xE9, // msgId = 1001
		0x00, 0x00, 0x00, 0x2A, // seqId = 42
		0x00, 0x00, 0x00, 0x80, // dataLen = 128
	}
	rb.Write(data)

	a, b, c := rb.PeekHeader12(0)
	if a != 1001 {
		t.Errorf("expected msgId=1001, got %d", a)
	}
	if b != 42 {
		t.Errorf("expected seqId=42, got %d", b)
	}
	if c != 128 {
		t.Errorf("expected dataLen=128, got %d", c)
	}

	if rb.Len() != 12 {
		t.Errorf("PeekHeader12 should not consume data, Len=%d", rb.Len())
	}
}

func TestRingBuffer_PeekHeader12_WrapAround(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	rb.Write(make([]byte, 10))
	rb.Discard(10)

	header := []byte{
		0x00, 0x00, 0x03, 0xE9,
		0x00, 0x00, 0x00, 0x2A,
		0x00, 0x00, 0x00, 0x80,
	}
	rb.Write(header)

	a, b, c := rb.PeekHeader12(0)
	if a != 1001 || b != 42 || c != 128 {
		t.Errorf("WrapAround: expected (1001,42,128), got (%d,%d,%d)", a, b, c)
	}
}

func TestRingBuffer_WriteFromReader(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64})

	reader := bytes.NewReader([]byte("hello world"))
	n, err := rb.WriteFromReader(reader, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("WriteFromReader failed: %v", err)
	}
	if n != 11 {
		t.Errorf("expected 11 bytes, got %d", n)
	}

	peeked, _ := rb.Peek(11)
	if string(peeked) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(peeked))
	}
}

func TestRingBuffer_Full(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 8, MaxSize: 8})

	// 写满
	data := []byte("12345678")
	n, _ := rb.Write(data)
	if n != 8 {
		t.Errorf("expected 8, got %d", n)
	}
	if !rb.IsFull() {
		t.Error("expected IsFull() = true")
	}

	// 再写应该失败
	_, err := rb.Write([]byte("x"))
	if err != ErrBufferFull {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}

	// 丢弃一些数据后可以继续写
	rb.Discard(4)
	n, err = rb.Write([]byte("ABCD"))
	if err != nil {
		t.Fatalf("Write after Discard failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4, got %d", n)
	}
}

func TestRingBuffer_PeekAll(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 写入数据使其环绕
	rb.Write([]byte("12345678"))
	rb.Discard(6)
	rb.Write([]byte("ABCDEFGH"))

	first, second := rb.PeekAll()

	// 第一段: "78"
	// 第二段: "ABCDEFGH"
	total := string(first) + string(second)
	if total != "78ABCDEFGH" {
		t.Errorf("expected '78ABCDEFGH', got '%s'", total)
	}
}

func TestRingBuffer_Grow(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 8, MaxSize: 32})

	// 写满初始缓冲区
	rb.Write([]byte("12345678"))
	if rb.Cap() != 8 {
		t.Errorf("expected Cap() = 8, got %d", rb.Cap())
	}

	// 尝试写入更多数据，触发扩容
	n, err := rb.Write([]byte("ABCD"))
	if err != nil {
		t.Fatalf("Write after Grow failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes written, got %d", n)
	}
	if rb.Cap() != 16 {
		t.Errorf("expected Cap() = 16 after Grow, got %d", rb.Cap())
	}

	// 验证数据完整性
	result := make([]byte, 12)
	rb.ReadFull(result)
	if string(result) != "12345678ABCD" {
		t.Errorf("expected '12345678ABCD', got '%s'", string(result))
	}
}

func TestRingBuffer_GrowMaxLimit(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 8, MaxSize: 16})

	// 写入超过最大限制的数据
	rb.Write([]byte("12345678"))
	rb.Write([]byte("ABCDEFGH")) // 扩容到 16

	// 再写应该失败（已达最大限制）
	n, err := rb.Write([]byte("XYZ"))
	if err != ErrBufferFull {
		t.Errorf("expected ErrBufferFull, got %v (n=%d)", err, n)
	}
}

func TestRingBuffer_PeekTwoSlices(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 写入数据使其环绕
	rb.Write([]byte("12345678"))
	rb.Discard(6)
	rb.Write([]byte("ABCDEFGH"))

	// 使用 PeekTwoSlices 读取跨边界数据
	first, second, err := rb.PeekTwoSlices(0, 10)
	if err != nil {
		t.Fatalf("PeekTwoSlices failed: %v", err)
	}

	total := string(first) + string(second)
	if total != "78ABCDEFGH" {
		t.Errorf("expected '78ABCDEFGH', got '%s'", total)
	}
}

func TestRingBuffer_IoReader(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64})
	rb.Write([]byte("hello world"))

	// 使用 io.Reader 接口读取
	buf := make([]byte, 5)
	n, err := rb.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 || string(buf) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(buf[:n]))
	}

	// 继续读取
	n, _ = rb.Read(buf)
	if string(buf[:n]) != " worl" {
		t.Errorf("expected ' worl', got '%s'", string(buf[:n]))
	}
}

func TestRingBuffer_Uint64Overflow(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16})

	// 模拟接近溢出的情况（通过直接设置指针）
	rb.read = ^uint64(0) - 100 // 接近 uint64 最大值
	rb.write = ^uint64(0) - 100

	// 写入和读取应该仍然正常工作
	rb.Write([]byte("test"))
	if rb.Len() != 4 {
		t.Errorf("expected Len() = 4, got %d", rb.Len())
	}

	result := make([]byte, 4)
	rb.ReadFull(result)
	if string(result) != "test" {
		t.Errorf("expected 'test', got '%s'", string(result))
	}
}

// ================================================================
// 基准测试
// ================================================================

func BenchmarkRingBuffer_Write(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(data)
		rb.Discard(1024)
	}
}

func BenchmarkRingBuffer_Peek(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	rb.Write(make([]byte, 32*1024))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Peek(1024)
	}
}

func BenchmarkRingBuffer_ReadUint32BE(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write([]byte{0x12, 0x34, 0x56, 0x78})
		rb.ReadUint32BE()
	}
}

func BenchmarkRingBuffer_PeekHeader12(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	rb.Write(make([]byte, 32*1024))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.PeekHeader12(0)
	}
}

func BenchmarkRingBuffer_PeekHeader12_vs_3xPeekUint32(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	rb.Write(make([]byte, 32*1024))

	b.Run("3xPeekUint32BE", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rb.PeekUint32BE(0)
			rb.PeekUint32BE(4)
			rb.PeekUint32BE(8)
		}
	})

	b.Run("PeekHeader12", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rb.PeekHeader12(0)
		}
	})
}

func BenchmarkRingBuffer_WriteFromReader(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	data := make([]byte, 4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		rb.WriteFromReader(reader, 0)
		rb.Reset()
	}
}

// 对比：传统方式 vs Ring Buffer
func BenchmarkTraditional_ReadCopy(b *testing.B) {
	data := make([]byte, 1024)
	reader := bytes.NewReader(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(data)
		buf := make([]byte, 1024) // 每次分配
		reader.Read(buf)
	}
}

func BenchmarkRingBuffer_ZeroCopyRead(b *testing.B) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64 * 1024})
	rb.Write(make([]byte, 32*1024))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Peek(1024) // 零拷贝
		rb.Discard(1024)
		rb.Write(make([]byte, 1024))
	}
}
