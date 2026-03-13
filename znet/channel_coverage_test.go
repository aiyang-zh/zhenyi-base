package znet

import (
	"context"
	"encoding/binary"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockLimiter implements iface.ILimit for rate limiting tests
type mockLimiter struct {
	allow bool
}

func (m *mockLimiter) Allow() bool {
	return m.allow
}

// testServer wraps BaseServer to implement IServer (adds Server/Close)
type testServer struct {
	*BaseServer
}

func (s *testServer) Server(ctx context.Context) {}
func (s *testServer) Close()                     {}

// newTestChannel creates a BaseChannel with net.Pipe for testing
func newTestChannel(t *testing.T) (*BaseChannel, net.Conn, *BaseServer) {
	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	ts := &testServer{BaseServer: bs}
	ch, serverConn := newTestChannelWithServer(t, ts)
	return ch, serverConn, bs
}

// newTestChannelWithServer creates a BaseChannel with the given server
func newTestChannelWithServer(t *testing.T, srv *testServer) (*BaseChannel, net.Conn) {
	clientConn, serverConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), clientConn, srv)
	return ch, serverConn
}

// ================================================================
// BaseChannel tests
// ================================================================

func TestBaseChannel_NewBaseChannel(t *testing.T) {
	ch, serverConn, bs := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	if ch == nil {
		t.Fatal("NewBaseChannel returned nil")
	}
	if ch.GetChannelId() == 0 {
		t.Error("channelId should be non-zero")
	}
	if ch.GetAddr() == "" {
		t.Error("addr should be non-empty")
	}
	_ = bs
}

func TestBaseChannel_IsOpen(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	if !ch.IsOpen() {
		t.Error("new channel should be open")
	}
	ch.Close()
	if ch.IsOpen() {
		t.Error("closed channel should not be open")
	}
}

func TestBaseChannel_GetChannelId(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	id := ch.GetChannelId()
	if id == 0 {
		t.Error("channelId should not be 0")
	}
}

func TestBaseChannel_GetAddr(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	addr := ch.GetAddr()
	if addr == "" {
		t.Error("GetAddr should return non-empty string")
	}
	if !strings.Contains(addr, "pipe") {
		t.Logf("net.Pipe addr format: %s", addr)
	}
}

func TestBaseChannel_GetAuthId_SetAuthId(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	if ch.GetAuthId() != 0 {
		t.Errorf("initial authId should be 0, got %d", ch.GetAuthId())
	}
	ch.SetAuthId(12345)
	if ch.GetAuthId() != 12345 {
		t.Errorf("SetAuthId/GetAuthId: expected 12345, got %d", ch.GetAuthId())
	}
}

func TestBaseChannel_GetRpcId(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	id1 := ch.GetRpcId()
	id2 := ch.GetRpcId()
	id3 := ch.GetRpcId()
	if id1 >= id2 || id2 >= id3 {
		t.Errorf("GetRpcId should auto-increment: %d, %d, %d", id1, id2, id3)
	}
}

func TestBaseChannel_SetLimit_Allow(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	// nil rate = always allow
	if !ch.Allow() {
		t.Error("Allow() with nil rate should return true")
	}

	// set limiter that allows
	ch.SetLimit(&mockLimiter{allow: true})
	if !ch.Allow() {
		t.Error("Allow() with allowing limiter should return true")
	}

	// set limiter that denies
	ch.SetLimit(&mockLimiter{allow: false})
	if ch.Allow() {
		t.Error("Allow() with denying limiter should return false")
	}
}

func TestBaseChannel_UpdateLastRecTime_Check(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	// Check when within timeout
	if !ch.Check() {
		t.Error("Check() should return true when within heartbeat timeout")
	}
	ch.UpdateLastRecTime()
	if !ch.Check() {
		t.Error("Check() after UpdateLastRecTime should return true")
	}
}

func TestBaseChannel_SetCloseCall(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	closed := false
	ch.SetCloseCall(func(ziface.IChannel) {
		closed = true
	})
	ch.Close()
	if !closed {
		t.Error("SetCloseCall callback should be invoked on Close")
	}
}

func TestBaseChannel_GetWriterTier_GetBuffered_Flush(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	if ch.GetWriterTier() != ziface.TierNone {
		t.Errorf("GetWriterTier: expected TierNone, got %d", ch.GetWriterTier())
	}
	if ch.GetBuffered() != 0 {
		t.Errorf("GetBuffered: expected 0, got %d", ch.GetBuffered())
	}
	if err := ch.Flush(); err != nil {
		t.Errorf("Flush: expected nil, got %v", err)
	}
}

func TestBaseChannel_Send_EnqueuesMessage(t *testing.T) {
	ch, serverConn, bs := newTestChannel(t)
	defer serverConn.Close()

	bs.AddChannel(ch)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch.StartSend(ctx)
	defer ch.Close()

	msg := GetNetMessage()
	msg.MsgId = 100
	msg.SeqId = 1
	msg.Data = []byte("hello")

	ch.Send(msg)

	// Give runSend time to process
	time.Sleep(50 * time.Millisecond)
	cancel()
}

func TestBaseChannel_Send_WhenClosed_ReleasesMessage(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.Close()
	msg := GetNetMessage()
	msg.MsgId = 1
	msg.SeqId = 0
	msg.Data = []byte("x")
	ch.Send(msg)
}

func TestBaseChannel_RecordRecv_GetReadBufferStats(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	ch.RecordRecv(100)
	lenRb, capRb, totalRead, totalWrite := ch.GetReadBufferStats()
	if capRb == 0 {
		t.Error("readBuffer cap should be non-zero")
	}
	_ = lenRb
	_ = totalRead
	_ = totalWrite
}

func TestBaseChannel_Close_Idempotent(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.Close()
	ch.Close()
	ch.Close()
	// Multiple Close should not panic
}

func TestBaseChannel_WriteBuffers_WhenClosed(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.Close()
	err := ch.writeBuffers(net.Buffers{[]byte("x")})
	if err == nil {
		t.Error("writeBuffers on closed channel should return error")
	}
}

func TestBaseChannel_SendBatchMsg_Empty(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer ch.Close()
	defer serverConn.Close()

	ch.SendBatchMsg(nil)
	ch.SendBatchMsg([]ziface.IMessage{})
}

func TestBaseChannel_SendBatchMsg_SingleMessage(t *testing.T) {
	// Use TCP (has kernel buffer) to avoid net.Pipe deadlock
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	acceptDone := make(chan net.Conn, 1)
	go func() {
		conn, _ := ln.Accept()
		acceptDone <- conn
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Skipf("dial: %v", err)
	}
	defer client.Close()

	serverConn := <-acceptDone
	if serverConn == nil {
		t.Fatal("accept failed")
	}
	defer serverConn.Close()

	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	ts := &testServer{BaseServer: bs}
	ch := NewBaseChannel(ts.NextId(), client, ts)
	defer ch.Close()
	bs.AddChannel(ch)

	msg := &NetMessage{}
	msg.MsgId = 1
	msg.SeqId = 42
	msg.Data = []byte("hello")

	ch.SendBatchMsg([]ziface.IMessage{msg})

	buf := make([]byte, 1024)
	serverConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n < 12 {
		t.Errorf("expected >= 12 bytes, got %d", n)
	}
}

func TestBaseChannel_Start_ReadLoopExits(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	bs := NewBaseServer("test:0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ts := &testServer{BaseServer: bs}
	ch := NewBaseChannel(ts.NextId(), clientConn, ts)
	bs.AddChannel(ch)

	done := make(chan struct{})
	go func() {
		ch.Start()
		close(done)
	}()

	// Close the server side to make read return
	serverConn.Close()
	select {
	case <-done:
		// Start() exited
	case <-time.After(2 * time.Second):
		t.Error("Start() should exit when remote closes")
	}
}

// ================================================================
// isNormalCloseError coverage (via error paths - method is private)
// ================================================================

func TestBaseChannel_IsNormalCloseError_UseOfClosed(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	ts := &testServer{BaseServer: bs}

	ch := NewBaseChannel(ts.NextId(), clientConn, ts)
	bs.AddChannel(ch)
	ch.SetCloseCall(func(ziface.IChannel) {})

	// Close server side first - client read will get "use of closed network connection"
	serverConn.Close()
	clientConn.Close()
	ch.Close()
}

// ================================================================
// BaseServer additional tests
// ================================================================

func TestBaseServer_SetHeartbeatTimeout(t *testing.T) {
	s := NewBaseServer(":0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	s.SetHeartbeatTimeout(5 * time.Second)
	s.SetHeartbeatTimeout(0)
}

func TestBaseServer_SetMaxConnections_AcceptAllowed(t *testing.T) {
	bs := NewBaseServer(":0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ts := &testServer{BaseServer: bs}

	if !bs.AcceptAllowed() {
		t.Error("AcceptAllowed with maxConns=0 should be true")
	}

	bs.SetMaxConnections(2)
	ch1, conn1 := newTestChannelWithServer(t, ts)
	defer conn1.Close()
	ch2, conn2 := newTestChannelWithServer(t, ts)
	defer conn2.Close()

	bs.AddChannel(ch1)
	if !bs.AcceptAllowed() {
		t.Error("AcceptAllowed with 1 conn and max 2 should be true")
	}
	bs.AddChannel(ch2)
	if bs.AcceptAllowed() {
		t.Error("AcceptAllowed with 2 connections and max 2 should be false (at capacity)")
	}
	ch1.Close()
	ch2.Close()

	bs2 := NewBaseServer(":0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ts2 := &testServer{BaseServer: bs2}
	bs2.SetMaxConnections(1)
	ch3, conn3 := newTestChannelWithServer(t, ts2)
	defer conn3.Close()
	bs2.AddChannel(ch3)
	if bs2.AcceptAllowed() {
		t.Error("AcceptAllowed with 1 conn and max 1 should be false")
	}
	ch3.Close()
}

func TestBaseServer_AddChannel_SetChannelAuth_GetChannelByAuthId(t *testing.T) {
	s := NewBaseServer(":0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ch, conn, _ := newTestChannel(t)
	defer conn.Close()
	defer ch.Close()

	s.AddChannel(ch)
	cid := ch.GetChannelId()

	if s.GetChannel(cid) != ch {
		t.Error("GetChannel should return added channel")
	}

	s.SetChannelAuth(cid, 999)
	if ch.GetAuthId() != 999 {
		t.Errorf("SetChannelAuth should update channel authId, got %d", ch.GetAuthId())
	}
	if s.GetChannelByAuthId(999) != ch {
		t.Error("GetChannelByAuthId should return channel")
	}
	if s.GetChannelByAuthId(0) != nil {
		t.Error("GetChannelByAuthId(0) should return nil")
	}
}

func TestBaseServer_RemoveChannel_OnClose(t *testing.T) {
	ch, conn, bs := newTestChannel(t)
	defer conn.Close()

	bs.AddChannel(ch)
	cid := ch.GetChannelId()
	ch.SetAuthId(100)
	bs.SetChannelAuth(cid, 100)

	if bs.GetChannel(cid) == nil {
		t.Fatal("channel should exist before close")
	}
	ch.Close()
	if bs.GetChannel(cid) != nil {
		t.Error("GetChannel should return nil after channel Close (removeChannel)")
	}
	if bs.GetChannelByAuthId(100) != nil {
		t.Error("GetChannelByAuthId should return nil after channel Close")
	}
}

// ================================================================
// RingBuffer additional tests (ReadByte, PeekByte, PeekBytes, Stats)
// ================================================================

func TestRingBuffer_ReadByte(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})
	defer PutRingBuffer(rb)

	rb.Write([]byte("AB"))
	b, err := rb.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 'A' {
		t.Errorf("ReadByte: expected 'A', got %q", b)
	}
	b, err = rb.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte 2: %v", err)
	}
	if b != 'B' {
		t.Errorf("ReadByte 2: expected 'B', got %q", b)
	}
	_, err = rb.ReadByte()
	if err != ErrBufferEmpty {
		t.Errorf("ReadByte on empty: expected ErrBufferEmpty, got %v", err)
	}
}

func TestRingBuffer_PeekByte(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})
	defer PutRingBuffer(rb)

	rb.Write([]byte("XYZ"))
	b, err := rb.PeekByte(0)
	if err != nil {
		t.Fatalf("PeekByte(0): %v", err)
	}
	if b != 'X' {
		t.Errorf("PeekByte(0): expected 'X', got %q", b)
	}
	b, err = rb.PeekByte(2)
	if err != nil {
		t.Fatalf("PeekByte(2): %v", err)
	}
	if b != 'Z' {
		t.Errorf("PeekByte(2): expected 'Z', got %q", b)
	}
	if rb.Len() != 3 {
		t.Error("PeekByte should not consume data")
	}
	_, err = rb.PeekByte(10)
	if err != ErrInsufficientData {
		t.Errorf("PeekByte out of range: expected ErrInsufficientData, got %v", err)
	}
}

func TestRingBuffer_PeekBytes(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})
	defer PutRingBuffer(rb)

	rb.Write([]byte("hello"))
	data, err := rb.PeekBytes(0, 5)
	if err != nil {
		t.Fatalf("PeekBytes: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("PeekBytes: expected 'hello', got %q", string(data))
	}
	if rb.Len() != 5 {
		t.Error("PeekBytes should not consume")
	}
	_, err = rb.PeekBytes(0, 10)
	if err != ErrInsufficientData {
		t.Errorf("PeekBytes insufficient: expected ErrInsufficientData, got %v", err)
	}
}

func TestRingBuffer_PeekBytes_WrapAround(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 16, MaxSize: 16})
	defer PutRingBuffer(rb)

	rb.Write([]byte("12345678"))
	rb.Discard(6)
	rb.Write([]byte("ABCDEFGH"))

	data, err := rb.PeekBytes(0, 10)
	if err != nil {
		t.Fatalf("PeekBytes wrap: %v", err)
	}
	expected := "78ABCDEFGH"
	if string(data) != expected {
		t.Errorf("PeekBytes wrap: expected %q, got %q", expected, string(data))
	}
}

func TestRingBuffer_Stats(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32})
	defer PutRingBuffer(rb)

	rb.Write([]byte("abcdef"))
	rb.Discard(3)
	totalRead, totalWrite := rb.Stats()
	if totalWrite != 6 {
		t.Errorf("Stats totalWrite: expected 6, got %d", totalWrite)
	}
	if totalRead != 3 {
		t.Errorf("Stats totalRead: expected 3, got %d", totalRead)
	}
}

// ================================================================
// BaseSocket HeaderLen with different protocol versions
// ================================================================

func TestBaseSocket_HeaderLen_ProtocolVersion(t *testing.T) {
	// Default (v0)
	s0 := NewBaseSocket()
	if s0.HeaderLen() != 12 {
		t.Errorf("v0 HeaderLen: expected 12, got %d", s0.HeaderLen())
	}

	// v1
	s1 := NewBaseSocket(SocketConfig{ProtocolVersion: 1})
	if s1.HeaderLen() != 13 {
		t.Errorf("v1 HeaderLen: expected 13, got %d", s1.HeaderLen())
	}
}

func TestBaseSocket_PreparePacket_ProtocolVersion1(t *testing.T) {
	s := NewBaseSocket(SocketConfig{ProtocolVersion: 1})
	msg := &NetMessage{MsgId: 100, SeqId: 42, Data: []byte("test")}
	var headerBuf [13]byte
	hdrLen, body := s.PreparePacket(msg, headerBuf[:])
	if hdrLen != 13 {
		t.Errorf("PreparePacket v1: expected hdrLen 13, got %d", hdrLen)
	}
	if headerBuf[0] != 1 {
		t.Errorf("PreparePacket v1: version byte should be 1, got %d", headerBuf[0])
	}
	gotMsgId := binary.BigEndian.Uint32(headerBuf[1:5])
	if gotMsgId != 100 {
		t.Errorf("PreparePacket v1 msgId: expected 100, got %d", gotMsgId)
	}
	if string(body) != "test" {
		t.Errorf("PreparePacket v1 body: expected 'test', got %q", string(body))
	}
}

// ================================================================
// Ensure iface.ILimit is satisfied
// ================================================================

var _ ziface.ILimit = (*mockLimiter)(nil)

// ================================================================
// Concurrent Send and Close
// ================================================================

func TestBaseChannel_ConcurrentSendClose(t *testing.T) {
	ch, serverConn, bs := newTestChannel(t)
	defer serverConn.Close()

	bs.AddChannel(ch)
	ctx, cancel := context.WithCancel(context.Background())
	ch.StartSend(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := GetNetMessage()
			msg.MsgId = int32(idx)
			msg.SeqId = uint32(idx)
			msg.Data = []byte("x")
			ch.Send(msg)
		}(i)
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	ch.Close()
	wg.Wait()
}

func TestBaseChannel_Send_NotOpen(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.state.Store(false)

	msg := GetNetMessage()
	msg.MsgId = 1
	ch.Send(msg)
}

func TestBaseChannel_Send_SyncMode_DropsMessage(t *testing.T) {
	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
		SyncMode: true,
	})
	ts := &testServer{BaseServer: bs}
	ch, serverConn := newTestChannelWithServer(t, ts)
	defer serverConn.Close()
	defer ch.Close()

	msg := GetNetMessage()
	msg.MsgId = 1
	msg.Data = []byte("x")
	// sync 模式下 Send 不 panic，打日志并丢弃消息（保证进程稳定）
	ch.Send(msg)
	// 不 panic 即通过；消息已被 Release，无需再 defer Release
}

func TestBaseChannel_GetReadBufferStats_NilBuffer(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.readBuffer = nil
	l, c, tr, tw := ch.GetReadBufferStats()
	if l != 0 || c != 0 || tr != 0 || tw != 0 {
		t.Fatal("nil readBuffer should return all zeros")
	}
}

func TestBaseChannel_Check_WithTimeout_Expired(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.SetHeartbeatTimeout(100 * time.Millisecond)
	ch.UpdateLastRecTime()
	time.Sleep(150 * time.Millisecond)

	if ch.Check() {
		t.Fatal("Check should return false when heartbeat expired")
	}
}

func TestBaseChannel_resetReadDeadline_WithTimeout(t *testing.T) {
	ch, serverConn, _ := newTestChannel(t)
	defer serverConn.Close()

	ch.SetHeartbeatTimeout(5 * time.Second)
	ch.resetReadDeadline()
}

func TestBaseChannel_Close_ConnAlreadyClosed(t *testing.T) {
	bs := NewBaseServer("test:0", ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	srv := &testServer{BaseServer: bs}
	clientConn, serverConn := net.Pipe()

	ch := NewBaseChannel(srv.NextId(), clientConn, srv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch.StartSend(ctx)

	serverConn.Close()
	clientConn.Close()
	time.Sleep(50 * time.Millisecond)

	ch.Close()
}
