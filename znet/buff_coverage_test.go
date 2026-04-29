package znet

import (
	"bytes"
	"errors"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi-base/ztime"
)

// ================================================================
// AdaptiveWriter (buff.go) tests
// ================================================================

func TestAdaptiveWriter_TierNone_DirectWrite(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	n, err := aw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
	if aw.GetTier() != ziface.TierNone {
		t.Errorf("expected TierNone, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_UpgradeTier(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.writeCount = 600
	aw.lastCheck = time.Now().Add(-3 * time.Second)

	aw.Write([]byte("x"))

	if aw.GetTier() != ziface.TierLarge {
		t.Fatalf("expected TierLarge, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_DowngradeTier(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.tier = ziface.TierLarge
	aw.poolBuf = zpool.GetBytesBuffer(16384)
	aw.n = 0
	aw.writeCount = 5
	aw.lastCheck = time.Now().Add(-15 * time.Second)
	aw.lastWrite = time.Now().Add(-35 * time.Second)

	aw.tryAdapt(ztime.ServerNow())

	if aw.GetTier() >= ziface.TierLarge {
		t.Fatalf("expected tier to downgrade from TierLarge, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_UpgradeTier_Medium(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.writeCount = 200
	aw.lastCheck = time.Now().Add(-3 * time.Second)

	aw.Write([]byte("x"))

	if aw.GetTier() != ziface.TierMedium {
		t.Fatalf("expected TierMedium, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_UpgradeTier_Small(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.writeCount = 30
	aw.lastCheck = time.Now().Add(-3 * time.Second)

	aw.Write([]byte("x"))

	if aw.GetTier() != ziface.TierSmall {
		t.Fatalf("expected TierSmall, got %d", aw.GetTier())
	}
}

func TestAdaptiveWriter_ResizeTier_WithPendingFlush(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	copy(aw.poolBuf.B, []byte("data"))
	aw.n = 4

	aw.resizeTier(ziface.TierMedium)

	if aw.GetTier() != ziface.TierMedium {
		t.Fatalf("expected TierMedium, got %d", aw.GetTier())
	}
	if buf.String() != "data" {
		t.Fatalf("expected pending data flushed, got %q", buf.String())
	}
}

func TestAdaptiveWriter_ResizeTier_FlushError(t *testing.T) {
	errW := &errWriter{err: errors.New("flush fail")}
	aw := NewAdaptiveWriter(errW)
	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	aw.poolBuf.B[0] = 'x'
	aw.n = 1

	aw.resizeTier(ziface.TierLarge)

	if aw.GetTier() != ziface.TierSmall {
		t.Fatal("resize should abort on flush error, tier should stay TierSmall")
	}
}

func TestAdaptiveWriter_Write_WithErrorState(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)
	aw.err = errors.New("prior error")

	n, err := aw.Write([]byte("test"))
	if err == nil {
		t.Fatal("expected error from prior state")
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes, got %d", n)
	}
}

func TestAdaptiveWriter_FlushInternal_PartialWriteError(t *testing.T) {
	pw := &partialWriter{failAfter: 3}
	aw := NewAdaptiveWriter(pw)
	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	copy(aw.poolBuf.B, []byte("hello world"))
	aw.n = 11

	err := aw.Flush()
	if err == nil {
		t.Fatal("expected error after partial write")
	}
	if aw.n == 0 {
		t.Fatal("should have remaining unflushed bytes")
	}
}

type partialWriter struct {
	failAfter int
	written   int
}

func (pw *partialWriter) Write(p []byte) (n int, err error) {
	remaining := pw.failAfter - pw.written
	if remaining <= 0 {
		return 0, errors.New("write limit reached")
	}
	if len(p) > remaining {
		pw.written += remaining
		return remaining, errors.New("partial write error")
	}
	pw.written += len(p)
	return len(p), nil
}

func TestAdaptiveWriter_FlushInternal_Error(t *testing.T) {
	errW := &errWriter{err: errors.New("write error")}
	aw := NewAdaptiveWriter(errW)
	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	aw.poolBuf.B[0] = 'x'
	aw.n = 1

	err := aw.Flush()
	if err == nil {
		t.Fatal("expected flush error")
	}
}

func TestAdaptiveWriter_FlushInternal_ShortWrite(t *testing.T) {
	zeroW := &zeroReturnWriter{}
	aw := NewAdaptiveWriter(zeroW)
	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	aw.poolBuf.B[0] = 'x'
	aw.n = 1

	err := aw.Flush()
	if err != io.ErrShortWrite {
		t.Errorf("expected ErrShortWrite, got %v", err)
	}
}

type errWriter struct{ err error }

func (e *errWriter) Write(p []byte) (n int, err error) {
	return 0, e.err
}

// zeroReturnWriter returns (0, nil) to trigger ErrShortWrite in flushInternal
type zeroReturnWriter struct{}

func (z *zeroReturnWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

func TestAdaptiveWriter_Write_BufferFull_Flush(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(64)
	aw.writeCount = 0
	aw.lastCheck = time.Now().Add(-3 * time.Second)

	data := make([]byte, 128)
	for i := range data {
		data[i] = byte('a' + i%26)
	}

	n, err := aw.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 128 {
		t.Errorf("expected 128 bytes written, got %d", n)
	}
	err = aw.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if buf.Len() != 128 {
		t.Errorf("expected 128 bytes in buffer, got %d", buf.Len())
	}
}

func TestAdaptiveWriter_Close_WithPendingData(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAdaptiveWriter(&buf)

	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	copy(aw.poolBuf.B, []byte("pending"))
	aw.n = 7

	err := aw.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if buf.String() != "pending" {
		t.Errorf("expected 'pending' flushed, got %q", buf.String())
	}
}

func TestAdaptiveWriter_Reset_WithBuffer(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	aw := NewAdaptiveWriter(&buf1)

	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	aw.n = 5

	aw.Reset(&buf2)

	if aw.poolBuf != nil {
		t.Error("Reset should release poolBuf")
	}
	if aw.tier != ziface.TierNone {
		t.Errorf("expected TierNone, got %d", aw.tier)
	}
	if aw.n != 0 {
		t.Errorf("expected n=0, got %d", aw.n)
	}
}

func TestAdaptiveWriter_Available_WithBuffer(t *testing.T) {
	aw := NewAdaptiveWriter(&bytes.Buffer{})
	aw.tier = ziface.TierSmall
	aw.poolBuf = zpool.GetBytesBuffer(2048)
	aw.n = 100

	avail := aw.Available()
	if avail != 1948 {
		t.Errorf("expected Available 1948, got %d", avail)
	}
}

// ================================================================
// RingBuffer tests
// ================================================================

func TestRingBuffer_PeekTwoSlices_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	data := []byte("AABBCCDDEE1234")
	rb.Write(data)

	first, second, err := rb.PeekTwoSlices(0, 14)
	if err != nil {
		t.Fatalf("PeekTwoSlices: %v", err)
	}
	total := len(first) + len(second)
	if total != 14 {
		t.Fatalf("expected 14 bytes total, got %d", total)
	}
	if second == nil {
		t.Fatal("expected second slice for wraparound data")
	}
	combined := append(first, second...)
	if string(combined) != string(data) {
		t.Errorf("expected %q, got %q", data, combined)
	}
}

func TestRingBuffer_Read_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	rb.Write([]byte("HELLO_WORLD!"))

	out := make([]byte, 12)
	n, err := rb.Read(out)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 12 {
		t.Errorf("expected 12 bytes, got %d", n)
	}
	if string(out[:n]) != "HELLO_WORLD!" {
		t.Errorf("expected HELLO_WORLD!, got %q", string(out[:n]))
	}
}

func TestRingBuffer_PeekBytes_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	rb.Write([]byte("AABBCCDDEE12"))

	data, err := rb.PeekBytes(0, 12)
	if err != nil {
		t.Fatalf("PeekBytes: %v", err)
	}
	if len(data) != 12 {
		t.Errorf("expected 12 bytes, got %d", len(data))
	}
	if string(data) != "AABBCCDDEE12" {
		t.Errorf("expected AABBCCDDEE12, got %q", data)
	}
}

func TestRingBuffer_PeekUint32BE_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 30)
	rb.Write(filler)
	rb.Discard(30)
	rb.Write([]byte{0x12, 0x34, 0x56, 0x78})

	v, err := rb.PeekUint32BE(0)
	if err != nil {
		t.Fatalf("PeekUint32BE: %v", err)
	}
	if v != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08x", v)
	}
}

func TestRingBuffer_ReadUint32BE_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 30)
	rb.Write(filler)
	rb.Discard(30)
	rb.Write([]byte{0xAB, 0xCD, 0xEF, 0x01})

	v, err := rb.ReadUint32BE()
	if err != nil {
		t.Fatalf("ReadUint32BE: %v", err)
	}
	if v != 0xABCDEF01 {
		t.Errorf("expected 0xABCDEF01, got 0x%08x", v)
	}
}

func TestRingBuffer_ReadFull_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	rb.Write([]byte("WRAP_DATA!"))

	out := make([]byte, 10)
	err := rb.ReadFull(out)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(out) != "WRAP_DATA!" {
		t.Errorf("expected WRAP_DATA!, got %q", out)
	}
}

func TestRingBuffer_Write_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	data := []byte("WRAP_WRITE_TEST!")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 16 {
		t.Errorf("expected 16 bytes, got %d", n)
	}
	out := make([]byte, 16)
	rb.ReadFull(out)
	if string(out) != "WRAP_WRITE_TEST!" {
		t.Errorf("expected WRAP_WRITE_TEST!, got %q", out)
	}
}

func TestRingBuffer_WriteFromReader_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)

	r := bytes.NewReader([]byte("WRAP_READER!"))
	n, err := rb.WriteFromReader(r, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("WriteFromReader: %v", err)
	}
	if n != 12 {
		t.Errorf("expected 12 bytes, got %d", n)
	}
}

func TestRingBuffer_Peek_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	rb.Write([]byte("PEEKWRAP1234"))

	data, err := rb.Peek(12)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(data) != 4 {
		t.Errorf("Peek should return first segment (4 bytes before boundary), got %d", len(data))
	}
}

func TestRingBuffer_PeekAll_ActualWraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 28)
	rb.Write(filler)
	rb.Discard(28)
	rb.Write([]byte("WRAP_ALL_TEST"))

	first, second := rb.PeekAll()
	total := len(first) + len(second)
	if total != 13 {
		t.Errorf("expected 13 bytes, got %d", total)
	}
	if second == nil {
		t.Fatal("expected two slices for wrapped data")
	}
}

func TestRingBuffer_Grow_AtMaxSize(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	data := make([]byte, 32)
	rb.Write(data)

	ok := rb.Grow(64)
	if ok {
		t.Fatal("Grow should fail when at MaxSize")
	}
}

func TestRingBuffer_Discard_MoreThanAvailable(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})
	rb.Write([]byte("abc"))
	err := rb.Discard(10)
	if err == nil {
		t.Fatal("expected error when discarding more than available")
	}
}

func TestRingBuffer_Write_PartialWhenFull(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	rb.Write(make([]byte, 30))

	n, err := rb.Write(make([]byte, 10))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 bytes (remaining space), got %d", n)
	}
}

func TestRingBuffer_PeekHeader12_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	filler := make([]byte, 26)
	rb.Write(filler)
	rb.Discard(26)
	header := []byte{0, 0, 0, 100, 0, 0, 0, 1, 0, 0, 0, 5}
	rb.Write(header)

	a, b, c := rb.PeekHeader12(0)
	if a != 100 || b != 1 || c != 5 {
		t.Errorf("expected (100, 1, 5), got (%d, %d, %d)", a, b, c)
	}
}

func TestRingBuffer_WriteFromReader_Partial(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 64})

	r := &limitReader{data: []byte("hello world"), limit: 5}
	n, err := rb.WriteFromReader(r, 10)
	if err != nil && err != io.EOF {
		t.Fatalf("WriteFromReader: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes, got %d", n)
	}
	if rb.Len() != 5 {
		t.Errorf("expected Len 5, got %d", rb.Len())
	}
}

type limitReader struct {
	data  []byte
	limit int
}

func (l *limitReader) Read(p []byte) (n int, err error) {
	if l.limit <= 0 {
		return 0, io.EOF
	}
	toRead := l.limit
	if toRead > len(p) {
		toRead = len(p)
	}
	if toRead > len(l.data) {
		toRead = len(l.data)
	}
	copy(p, l.data[:toRead])
	l.data = l.data[toRead:]
	l.limit -= toRead
	return toRead, nil
}

func TestRingBuffer_Peek_SplitData(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})

	rb.Write([]byte("abcdef"))
	rb.Discard(3)
	rb.Write([]byte("GHIJKL"))

	data, err := rb.Peek(10)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(data) < 9 {
		t.Errorf("expected at least 9 bytes, got %d", len(data))
	}
}

func TestRingBuffer_PeekAll_Wraparound(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})

	rb.Write([]byte("aaa"))
	rb.Discard(2)
	rb.Write([]byte("bbb"))

	first, second := rb.PeekAll()
	total := len(first) + len(second)
	if total != 4 {
		t.Errorf("expected 4 bytes total, got %d", total)
	}
}

// ================================================================
// BaseClient read() tests
// ================================================================

func TestBaseClient_ReadLoop_FullMessage(t *testing.T) {
	client := NewBaseClient()
	client.SetEncrypt(zencrypt.NewBaseEncrypt())
	serverConn, clientConn := net.Pipe()
	client.SetConn(clientConn)

	var received []*NetMessage
	var mu sync.Mutex
	client.SetReadCall(func(msg ziface.IWireMessage) {
		mu.Lock()
		received = append(received, msg.(*NetMessage).Clone())
		mu.Unlock()
	})

	msg := GetNetMessage()
	msg.SetMsgId(100)
	msg.SetSeqId(1)
	msg.SetMessageData([]byte("hello"))

	socket := NewBaseSocket()
	var headerBuf [12]byte
	headerLen, body := socket.PreparePacket(msg, headerBuf[:])

	go func() {
		serverConn.Write(headerBuf[:headerLen])
		if len(body) > 0 {
			serverConn.Write(body)
		}
		time.Sleep(100 * time.Millisecond)
		serverConn.Close()
	}()

	for {
		n := client.read()
		if n != 0 {
			break
		}
	}

	msg.Release()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0].GetMsgId() != 100 {
		t.Errorf("expected msgId 100, got %d", received[0].GetMsgId())
	}
}

func TestBaseClient_Read_WithoutAsyncMode_Panics(t *testing.T) {
	client := NewBaseClient()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling Read() without WithAsyncMode")
		}
	}()
	client.Read()
}

func TestBaseClient_Request_WithAsyncMode_ReturnsError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)
	defer client.Close()

	_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
	if err == nil {
		t.Fatal("expected error when calling Request() with WithAsyncMode")
	}
	if !strings.Contains(err.Error(), "WithAsyncMode") {
		t.Errorf("expected error about WithAsyncMode, got %v", err)
	}
}

func TestBaseClient_Request_UsesDefaultMaxDataLength(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewBaseClient()
	client.SetConn(clientConn)
	defer client.Close()

	go func() {
		// Drain one client request packet first (header 12B + body 1B),
		// otherwise both ends may block on net.Pipe writes.
		reqBuf := make([]byte, 13)
		_, _ = io.ReadFull(serverConn, reqBuf)

		// msgId=1, seqId=1, dataLen=DefaultMaxDataLength+1
		header := []byte{
			0x00, 0x00, 0x00, 0x01,
			0x00, 0x00, 0x00, 0x01,
			0x00, 0x10, 0x00, 0x01,
		}
		_, _ = serverConn.Write(header)
		_ = serverConn.Close()
	}()

	_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
	if !errors.Is(err, ErrBufferFull) {
		t.Fatalf("expected ErrBufferFull when dataLen > DefaultMaxDataLength, got %v", err)
	}
}

func TestBaseClient_Read_BufferFull_SinglePacketTooBig(t *testing.T) {
	client := NewBaseClient()
	serverConn, clientConn := net.Pipe()
	client.SetConn(clientConn)
	client.SetReadCall(func(ziface.IWireMessage) {})

	rb := client.readBuffer
	for rb.Free() > 0 {
		rb.Write([]byte{0x01})
	}

	go func() {
		malformed := make([]byte, 12)
		malformed[8] = 0xFF
		malformed[9] = 0xFF
		malformed[10] = 0xFF
		malformed[11] = 0xFF
		serverConn.Write(malformed)
		time.Sleep(50 * time.Millisecond)
		serverConn.Close()
	}()

	n := client.read()
	if n != 1 {
		t.Errorf("expected read to return 1 (exit), got %d", n)
	}
	clientConn.Close()
}

func TestBaseClient_Read_MultipleMessages(t *testing.T) {
	client := NewBaseClient()
	client.SetEncrypt(zencrypt.NewBaseEncrypt())
	serverConn, clientConn := net.Pipe()
	client.SetConn(clientConn)

	var received []*NetMessage
	var mu sync.Mutex
	client.SetReadCall(func(msg ziface.IWireMessage) {
		mu.Lock()
		received = append(received, msg.(*NetMessage).Clone())
		mu.Unlock()
	})

	packets := [][]byte{
		makeTestPacket(100, 1, []byte("a")),
		makeTestPacket(101, 2, []byte("b")),
		makeTestPacket(102, 3, []byte("c")),
	}

	go func() {
		for _, p := range packets {
			serverConn.Write(p)
		}
		time.Sleep(50 * time.Millisecond)
		serverConn.Close()
	}()

	for {
		n := client.read()
		if n != 0 {
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(received))
	}
}

func TestBaseClient_Read_ParseError(t *testing.T) {
	client := NewBaseClient()
	serverConn, clientConn := net.Pipe()
	client.SetConn(clientConn)
	client.SetReadCall(func(ziface.IWireMessage) {})

	go func() {
		// Send malformed packet: dataLen=0xFFFFFFFF exceeds MaxDataLength
		malformed := make([]byte, 12)
		malformed[8] = 0xFF
		malformed[9] = 0xFF
		malformed[10] = 0xFF
		malformed[11] = 0xFF
		serverConn.Write(malformed)
		time.Sleep(50 * time.Millisecond)
		serverConn.Close()
	}()

	n := client.read()
	if n != 1 {
		t.Errorf("expected read to return 1 on parse error, got %d", n)
	}
	clientConn.Close()
}

func TestBaseClient_Read_ClosedClient(t *testing.T) {
	client := NewBaseClient()
	client.Close()

	n := client.read()
	if n != 1 {
		t.Errorf("expected read to return 1 when closed, got %d", n)
	}
}

func TestBaseClient_isNormalCloseError(t *testing.T) {
	client := NewBaseClient()

	tests := []struct {
		err    error
		expect bool
	}{
		{nil, false},
		{errors.New("other"), false},
		{errors.New("use of closed network connection"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("forcibly closed by the remote host"), true},
	}
	for _, tt := range tests {
		got := client.isNormalCloseError(tt.err)
		if got != tt.expect {
			t.Errorf("isNormalCloseError(%v) = %v, want %v", tt.err, got, tt.expect)
		}
	}
}

// ================================================================
// ParseFromRingBuffer (socket.go) tests
// ================================================================

func TestParseFromRingBuffer_InsufficientHeader(t *testing.T) {
	rb := GetRingBuffer()
	defer PutRingBuffer(rb)

	rb.Write([]byte{0, 0, 0}) // only 3 bytes

	socket := NewBaseSocket()
	parseData := GetParseData()
	defer PutParseData(parseData)

	parsed, err := socket.ParseFromRingBuffer(rb, parseData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed {
		t.Error("expected parsed=false (insufficient header)")
	}
}

func TestParseFromRingBuffer_PartialBody(t *testing.T) {
	rb := GetRingBuffer()
	defer PutRingBuffer(rb)

	// Full 12-byte header (msgId=1, seqId=1, dataLen=5) + only 1 byte of body
	header := makeTestPacket(1, 1, []byte("12345"))
	rb.Write(header[:13])

	sock := NewBaseSocket()
	parseData := GetParseData()
	defer PutParseData(parseData)

	parsed, err := sock.ParseFromRingBuffer(rb, parseData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed {
		t.Error("expected parsed=false (partial body)")
	}
}

func TestParseFromRingBuffer_ProtocolVersionMismatch(t *testing.T) {
	rb := GetRingBuffer()
	defer PutRingBuffer(rb)

	sock := NewBaseSocket(SocketConfig{ProtocolVersion: 1})
	// Write v0 packet (no version byte) - first byte will be 0, not 1
	rb.Write(makeTestPacket(1, 1, []byte("x")))

	parseData := GetParseData()
	defer PutParseData(parseData)

	parsed, err := sock.ParseFromRingBuffer(rb, parseData)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	if parsed {
		t.Error("expected parsed=false")
	}
}

func TestParseFromRingBuffer_ProtocolV1(t *testing.T) {
	rb := GetRingBuffer()
	defer PutRingBuffer(rb)

	sock := NewBaseSocket(SocketConfig{
		MaxHeaderLength: DefaultMaxHeaderLength,
		MaxDataLength:   DefaultMaxDataLength,
		MaxMsgId:        DefaultMaxMsgId,
		ProtocolVersion: 1,
	})

	// V1: version(1) + msgId(4) + seqId(4) + dataLen(4) + data
	// Header is at offset 1: msgId big-endian at [1:5], seqId at [5:9], dataLen at [9:13]
	packet := make([]byte, 14)
	packet[0] = 1 // version
	packet[1] = 0 // msgId 100 = 0x00000064
	packet[2] = 0
	packet[3] = 0
	packet[4] = 100
	packet[5] = 0 // seqId 1
	packet[6] = 0
	packet[7] = 0
	packet[8] = 1
	packet[9] = 0 // dataLen = 1
	packet[10] = 0
	packet[11] = 0
	packet[12] = 1
	packet[13] = 'x' // body
	rb.Write(packet)

	parseData := GetParseData()
	defer PutParseData(parseData)

	parsed, err := sock.ParseFromRingBuffer(rb, parseData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed {
		t.Error("expected parsed=true")
	}
	if parseData.Message.GetMsgId() != 100 {
		t.Errorf("expected msgId 100, got %d", parseData.Message.GetMsgId())
	}
}
