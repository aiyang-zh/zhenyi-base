package znet

// 回归测试：关服清理、auth 映射、协议校验、reactor 读/共享写关闭等行为契约。
import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
)

func newRegressionServer(t *testing.T) *testServer {
	t.Helper()
	return &testServer{BaseServer: NewBaseServer("127.0.0.1:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})}
}

// 无 SetListener（如部分 WebSocket 路径）时 BaseClose 仍须关闭全部 channel。
func TestBaseClose_ClosesChannelsWithoutListener(t *testing.T) {
	srv := newRegressionServer(t)
	clientConn, srvConn := net.Pipe()
	defer clientConn.Close()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)
	srv.AddChannel(ch)

	if srv.listener.Load() != nil {
		t.Fatal("expected no listener before BaseClose")
	}

	srv.BaseClose()
	if !ch.isClose.Load() {
		t.Fatal("BaseClose must close all channels even when listener is nil")
	}
}

// 拒绝连接或显式 Close 须归还 readBuffer 与 mailBoxQueue，不得仅关闭底层 conn。
func TestRejectedChannel_CloseReleasesResources(t *testing.T) {
	srv := newRegressionServer(t)
	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)

	ch.Close()
	if !ch.isClose.Load() {
		t.Fatal("Close must mark channel closed")
	}
	if ch.readBuffer != nil {
		t.Fatal("Close must release readBuffer to pool (readBuffer should be nil)")
	}
	if ch.mailBoxQueue != nil && !ch.mailBoxQueue.Empty() {
		t.Fatal("unexpected: mailBoxQueue should be closable after Close")
	}
}

// 仅关闭底层 conn 不得释放 channel readBuffer（与完整 Close 对比）。
func TestRejectedChannel_ConnCloseOnlyLeaksReadBuffer(t *testing.T) {
	srv := newRegressionServer(t)
	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)

	if err := ch.conn.Close(); err != nil {
		t.Fatalf("conn.Close: %v", err)
	}
	if ch.readBuffer == nil {
		t.Fatal("conn.Close alone must not release readBuffer")
	}
	if ch.isClose.Load() {
		t.Fatal("conn.Close alone must not close channel")
	}
}

// authId 冲突绑定时，先关闭的连接不得删除仍存活连接的 auth 映射。
func TestSetChannelAuth_ConflictSurvivorKeepsMapping(t *testing.T) {
	srv := newRegressionServer(t)

	c1, s1 := net.Pipe()
	defer c1.Close()
	chA := NewBaseChannel(srv.NextId(), s1, srv)
	srv.AddChannel(chA)

	c2, s2 := net.Pipe()
	defer c2.Close()
	chB := NewBaseChannel(srv.NextId(), s2, srv)
	srv.AddChannel(chB)

	const authID uint64 = 9001
	srv.SetChannelAuth(chA.GetChannelId(), authID)
	srv.SetChannelAuth(chB.GetChannelId(), authID)

	if got := srv.GetChannelByAuthId(authID); got == nil || got.GetChannelId() != chB.GetChannelId() {
		t.Fatalf("auth map should point to channel B, got %v", got)
	}
	if chA.GetAuthId() != 0 {
		t.Fatalf("channel A authId should be cleared after B took authId, got %d", chA.GetAuthId())
	}

	chA.Close()
	if got := srv.GetChannelByAuthId(authID); got == nil || got.GetChannelId() != chB.GetChannelId() {
		t.Fatalf("after A closes, auth map must still resolve to B, got %v", got)
	}
}

// SetChannelAuth(channelId, 0) 须解绑 authChannels 中仍指向该 channel 的条目。
func TestSetChannelAuth_ClearAuthIdRemovesMapping(t *testing.T) {
	srv := newRegressionServer(t)

	_, s1 := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), s1, srv)
	srv.AddChannel(ch)

	const authID uint64 = 42
	srv.SetChannelAuth(ch.GetChannelId(), authID)
	if got := srv.GetChannelByAuthId(authID); got == nil || got.GetChannelId() != ch.GetChannelId() {
		t.Fatalf("expected auth map to resolve to channel, got %v", got)
	}

	srv.SetChannelAuth(ch.GetChannelId(), 0)
	if ch.GetAuthId() != 0 {
		t.Fatalf("channel authId should be 0, got %d", ch.GetAuthId())
	}
	if got := srv.GetChannelByAuthId(authID); got != nil {
		t.Fatalf("auth map should be cleared, got %v", got)
	}
}

// v1 协议下 writeImmediate 须使用足够大的 header 缓冲，不得 panic。
func TestBaseClient_writeImmediate_ProtocolV1(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	cfg := DefaultSocketConfig()
	cfg.ProtocolVersion = 1
	client := NewBaseClient(WithSocketConfig(cfg))
	client.SetConn(clientConn)
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		hdr := make([]byte, headerSizeV1)
		if _, err := io.ReadFull(serverConn, hdr); err != nil {
			t.Errorf("server read header: %v", err)
			return
		}
		if hdr[0] != 1 {
			t.Errorf("expected version byte 1, got %d", hdr[0])
		}
		dataLen := binary.BigEndian.Uint32(hdr[9:13])
		if dataLen > 0 {
			body := make([]byte, dataLen)
			if _, err := io.ReadFull(serverConn, body); err != nil {
				t.Errorf("server read body: %v", err)
			}
		}
	}()

	client.writeImmediate(&NetMessage{MsgId: 42, SeqId: 7, Data: []byte("hi")}, clientConn)
	<-done
}

// Request 须按协议 HeaderLen 读写（含 v1 版本字节）。
func TestBaseClient_Request_ProtocolV1(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	cfg := DefaultSocketConfig()
	cfg.ProtocolVersion = 1
	client := NewBaseClient(WithSocketConfig(cfg))
	client.SetConn(clientConn)
	defer client.Close()

	go func() {
		reqHdr := make([]byte, headerSizeV1)
		if _, err := io.ReadFull(serverConn, reqHdr); err != nil {
			return
		}
		reqLen := binary.BigEndian.Uint32(reqHdr[9:13])
		if reqLen > 0 {
			discard := make([]byte, reqLen)
			_, _ = io.ReadFull(serverConn, discard)
		}

		resp := make([]byte, headerSizeV1)
		resp[0] = 1
		binary.BigEndian.PutUint32(resp[1:5], 99)
		binary.BigEndian.PutUint32(resp[5:9], 1)
		binary.BigEndian.PutUint32(resp[9:13], 3)
		_, _ = serverConn.Write(resp)
		_, _ = serverConn.Write([]byte("ack"))
	}()

	msg, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("q")})
	if err != nil {
		t.Fatalf("Request v1: %v", err)
	}
	if msg.GetMsgId() != 99 {
		t.Fatalf("expected msgId 99, got %d", msg.GetMsgId())
	}
	if string(msg.GetMessageData()) != "ack" {
		t.Fatalf("expected body ack, got %q", msg.GetMessageData())
	}
}

// Request 须尊重 SocketConfig.MaxDataLength，超长 body 返回 validation 错误。
func TestBaseClient_Request_RespectsSocketMaxDataLength(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	const maxBody = 64
	cfg := DefaultSocketConfig()
	cfg.MaxDataLength = maxBody
	client := NewBaseClient(WithSocketConfig(cfg))
	client.SetConn(clientConn)
	defer client.Close()

	go func() {
		reqHdr := make([]byte, headerSizeV0)
		_, _ = io.ReadFull(serverConn, reqHdr)
		reqLen := binary.BigEndian.Uint32(reqHdr[8:12])
		if reqLen > 0 {
			discard := make([]byte, reqLen)
			_, _ = io.ReadFull(serverConn, discard)
		}

		hdr := make([]byte, headerSizeV0)
		binary.BigEndian.PutUint32(hdr[8:12], maxBody+1)
		_, _ = serverConn.Write(hdr)
	}()

	_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
	if err == nil {
		t.Fatal("expected error for oversized response body")
	}
	if !zerrs.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// reactor 读路径 WriteToReadBuffer 须更新 lastRecTime（与自旋读路径一致）。
func TestWriteToReadBuffer_UpdatesLastRecTime(t *testing.T) {
	srv := newRegressionServer(t)
	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)
	ch.SetHeartbeatTimeout(time.Second)

	before := atomic.LoadInt64(&ch.lastRecTime)
	time.Sleep(2 * time.Millisecond)

	if _, err := ch.WriteToReadBuffer([]byte{1, 2, 3}); err != nil {
		t.Fatalf("WriteToReadBuffer: %v", err)
	}
	after := atomic.LoadInt64(&ch.lastRecTime)
	if after <= before {
		t.Fatal("WriteToReadBuffer must update lastRecTime for reactor heartbeat")
	}
}

// WriteToReadBuffer 缓冲满且无法扩容时须返回错误，不得静默丢数据。
func TestWriteToReadBuffer_ReturnsErrWhenBufferFull(t *testing.T) {
	srv := newRegressionServer(t)
	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)
	ch.readBuffer = NewRingBuffer(RingBufferConfig{Size: 16, MaxSize: 16})

	junk := make([]byte, 16)
	for i := range junk {
		junk[i] = 0xff
	}
	if _, err := ch.WriteToReadBuffer(junk); err != nil {
		t.Fatalf("first write should fill buffer: %v", err)
	}
	if _, err := ch.WriteToReadBuffer([]byte{1, 2, 3}); err == nil {
		t.Fatal("WriteToReadBuffer must return error when buffer cannot accept data")
	}
}

// BaseClose 在共享写模式下并行等待 drain，墙钟非逐连接串行累加 SharedSendCloseTimeout。
func TestBaseClose_SharedSend_DoesNotBlockPerChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv := newRegressionServer(t)
	srv.SetSharedSendWorkerMode(true)

	const nCh = 4
	channels := make([]*BaseChannel, nCh)
	for i := 0; i < nCh; i++ {
		_, srvConn := net.Pipe()
		ch := NewBaseChannel(srv.NextId(), srvConn, srv)
		if !srv.BindSharedSendHook(ctx, ch) {
			t.Fatal("BindSharedSendHook failed")
		}
		srv.AddChannel(ch)
		m := GetNetMessage()
		m.SetMsgId(1)
		m.SetMessageData([]byte("payload"))
		ch.Send(m)
		channels[i] = ch
	}

	start := time.Now()
	srv.BaseClose()
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("BaseClose blocked too long with shared-send channels: %v", elapsed)
	}
	for _, ch := range channels {
		if !ch.isClose.Load() {
			t.Fatal("BaseClose must mark shared-send channels closed")
		}
	}
}

// BaseClose 在共享写 drain 全局超时后须 stopSharedSendWorkers，避免 worker goroutine 残留。
func TestBaseClose_SharedSendDrainTimeout_StopsWorkers(t *testing.T) {
	defer saveSendLoopTuning(t)()
	SetSendLoopTuning(SendLoopTuning{SharedSendCloseTimeout: 5 * time.Millisecond})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srv := newRegressionServer(t)
	srv.SetSharedSendWorkerMode(true)
	srv.ensureSharedSendWorkers(ctx)

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)
	// 故意不处理 close hook，模拟 worker 无法 ack drain。
	ch.SetSharedSendHook(func(*BaseChannel) {})
	srv.AddChannel(ch)

	start := time.Now()
	srv.BaseClose()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("BaseClose should return after global drain timeout, took %v", elapsed)
	}

	srv.sharedSendWorkersMu.Lock()
	stillRunning := srv.sharedSendWorkersCancel != nil
	srv.sharedSendWorkersMu.Unlock()
	if stillRunning {
		t.Fatal("BaseClose must stop shared send workers after drain timeout")
	}
	if !ch.isClose.Load() {
		t.Fatal("BaseClose must mark channel closed even when drain ack times out")
	}
}

// 环缓冲已满且无法扩容时，部分写入须返回 io.ErrShortWrite。
func TestRingBuffer_Write_PartialWhenFull_ReturnsShortWrite(t *testing.T) {
	rb := NewRingBuffer(RingBufferConfig{Size: 32, MaxSize: 32})
	if _, err := rb.Write(make([]byte, 30)); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	n, err := rb.Write(make([]byte, 10))
	if n != 2 {
		t.Fatalf("expected 2 bytes written, got %d", n)
	}
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected io.ErrShortWrite on partial write, got %v", err)
	}
}

// AddChannel 在 maxConn 边界拒绝直连入队（未经 HandleAccept 占位时仍原子递增校验）。
func TestAddChannel_EnforcesMaxConnAtIncrement(t *testing.T) {
	srv := newRegressionServer(t)
	srv.SetMaxConnections(1)
	var metrics regressionMetrics
	srv.SetMetrics(&metrics)

	c1, s1 := net.Pipe()
	defer c1.Close()
	ch1 := NewBaseChannel(srv.NextId(), s1, srv)
	srv.AddChannel(ch1)

	c2, s2 := net.Pipe()
	defer c2.Close()
	ch2 := NewBaseChannel(srv.NextId(), s2, srv)
	srv.AddChannel(ch2)

	if srv.connCount.Load() > 1 {
		t.Fatalf("connCount must not exceed maxConn, got %d", srv.connCount.Load())
	}
	if !ch2.isClose.Load() {
		t.Fatal("overflow channel must be closed when maxConn exceeded at AddChannel")
	}
	if metrics.rejected.Load() != 1 {
		t.Fatalf("ConnRejectedInc expected 1, got %d", metrics.rejected.Load())
	}
}

// HandleAccept 在 maxConn 下先占位再 OnAccept，满员时不得调用 OnAccept。
func TestHandleAccept_MaxConn_ReservesBeforeOnAccept(t *testing.T) {
	srv := newRegressionServer(t)
	srv.SetMaxConnections(1)
	var metrics regressionMetrics
	srv.SetMetrics(&metrics)
	var onAcceptCalls atomic.Int32
	srv.handlers.OnAccept = func(ziface.IChannel) bool {
		onAcceptCalls.Add(1)
		return true
	}

	c1, s1 := net.Pipe()
	defer c1.Close()
	ch1 := NewBaseChannel(srv.NextId(), s1, srv)
	if !srv.HandleAccept(ch1) {
		t.Fatal("first HandleAccept should succeed")
	}
	srv.AddChannel(ch1)

	c2, s2 := net.Pipe()
	defer c2.Close()
	ch2 := NewBaseChannel(srv.NextId(), s2, srv)
	if srv.HandleAccept(ch2) {
		t.Fatal("second HandleAccept must fail when at maxConn")
	}
	if onAcceptCalls.Load() != 1 {
		t.Fatalf("OnAccept must not run when maxConn full, got %d calls", onAcceptCalls.Load())
	}
	if metrics.rejected.Load() != 1 {
		t.Fatalf("ConnRejectedInc expected 1, got %d", metrics.rejected.Load())
	}
	if srv.connCount.Load() != 1 {
		t.Fatalf("connCount must stay 1, got %d", srv.connCount.Load())
	}
	ch2.CloseFromSharedSendPath()
}

type errDecryptOnly struct{}

func (errDecryptOnly) Encrypt(data []byte) ([]byte, error) { return data, nil }
func (errDecryptOnly) Decrypt([]byte) ([]byte, error) {
	return nil, errors.New("decrypt failed")
}

// 服务端读路径解密失败须断链（与 Parse 错误一致）。
func TestChannel_DecryptError_ClosesConnection(t *testing.T) {
	srv := newRegressionServer(t)
	srv.SetEncrypt(errDecryptOnly{})

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)

	// 最小合法 v1 包：msgId=1, seqId=0, len=3, data=加密载荷（Decrypt 会失败）
	hdr := make([]byte, 13)
	binary.BigEndian.PutUint32(hdr[0:4], 1)
	binary.BigEndian.PutUint32(hdr[4:8], 0)
	binary.BigEndian.PutUint32(hdr[8:12], 3)
	hdr[12] = 0
	pkt := append(hdr, []byte("bad")...)
	if _, err := ch.WriteToReadBuffer(pkt); err != nil {
		t.Fatalf("WriteToReadBuffer: %v", err)
	}
	if !ch.ParseAndDispatch() {
		t.Fatal("ParseAndDispatch must return true on decrypt error")
	}
}

// SetDisconnectOnDecryptError(false) 时解密失败不断链。
func TestChannel_DecryptError_LegacyDrop(t *testing.T) {
	srv := newRegressionServer(t)
	srv.SetDisconnectOnDecryptError(false)
	srv.SetEncrypt(errDecryptOnly{})

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(srv.NextId(), srvConn, srv)

	hdr := make([]byte, 13)
	binary.BigEndian.PutUint32(hdr[0:4], 1)
	binary.BigEndian.PutUint32(hdr[8:12], 3)
	pkt := append(hdr, []byte("bad")...)
	if _, err := ch.WriteToReadBuffer(pkt); err != nil {
		t.Fatalf("WriteToReadBuffer: %v", err)
	}
	if ch.ParseAndDispatch() {
		t.Fatal("legacy mode must not close on decrypt error")
	}
	if ch.isClose.Load() {
		t.Fatal("channel must stay open in legacy decrypt mode")
	}
}

// HandleAccept 成功但未 AddChannel 不得泄漏 connCount。
func TestHandleAccept_WithoutAddChannel_NoConnCountLeak(t *testing.T) {
	srv := newRegressionServer(t)
	srv.SetMaxConnections(2)
	srv.handlers.OnAccept = func(ziface.IChannel) bool { return true }

	c1, s1 := net.Pipe()
	defer c1.Close()
	ch1 := NewBaseChannel(srv.NextId(), s1, srv)
	if !srv.HandleAccept(ch1) {
		t.Fatal("HandleAccept should succeed")
	}
	// 故意不 AddChannel
	if srv.connCount.Load() != 0 {
		t.Fatalf("connCount must stay 0 without AddChannel, got %d", srv.connCount.Load())
	}
}

type regressionMetrics struct {
	rejected atomic.Int32
}

func (m *regressionMetrics) ConnInc()         {}
func (m *regressionMetrics) ConnDec()         {}
func (m *regressionMetrics) ConnRejectedInc() { m.rejected.Add(1) }

// 共享写 worker 与 Close 并发时不得与 mailBoxQueue.Close 竞态（-race）。
func TestSharedSend_CloseConcurrentWithWorker_NoRace(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	clientConn, srvConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })

	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}

	const rounds = 40
	for round := 0; round < rounds; round++ {
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 8; j++ {
					m := GetNetMessage()
					m.SetMsgId(1)
					m.SetMessageData([]byte("payload"))
					ch.Send(m)
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			ch.Close()
		}()
		wg.Wait()

		if ch.isClose.Load() {
			clientConn2, srvConn2 := net.Pipe()
			t.Cleanup(func() { _ = clientConn2.Close() })
			ch = NewBaseChannel(ts.NextId(), srvConn2, ts)
			if !ts.BindSharedSendHook(ctx, ch) {
				t.Fatal("BindSharedSendHook failed on reopen")
			}
		}
	}
}

// CloseFromReactor 不得同步等待共享写 worker 排空。
func TestSharedSend_CloseFromReactor_ReturnsBeforeWorkerDrain(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	clientConn, srvConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })

	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}

	for i := 0; i < 64; i++ {
		m := GetNetMessage()
		m.SetMsgId(1)
		m.SetMessageData([]byte("payload"))
		ch.Send(m)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		ch.CloseFromReactor()
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CloseFromReactor blocked waiting for shared send worker")
	}
	if !ch.isClose.Load() {
		t.Fatal("CloseFromReactor must mark channel closed")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ch.sharedSendCloseAck:
			return
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("shared send worker did not ack mailbox drain after CloseFromReactor")
}

// worker 已在本 goroutine 完成 drain 后，Close 不得再等待 ack。
func TestSharedSend_CloseAfterInlineDrain_NoWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}

	m := GetNetMessage()
	m.SetMsgId(1)
	m.SetMessageData([]byte("x"))
	ch.Send(m)

	ch.sharedSendDrainAndCloseMailbox(make([]ziface.IMessage, MaxBatchLimit))
	ch.sharedSendCloseDone.Store(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ch.Close()
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Close blocked after sharedSendCloseDone was set")
	}
}

// 读栈活跃时 CloseFromReactor 须延后归还 readBuffer，直至 EndReactorRead。
func TestReactorRead_DeferredReadBufferRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}

	ch.BeginReactorRead()
	ch.CloseFromReactor()
	if ch.readBuffer == nil {
		t.Fatal("readBuffer must not be released while reactor read stack is active")
	}
	ch.EndReactorRead()
	if ch.readBuffer != nil {
		t.Fatal("readBuffer must be released after EndReactorRead")
	}
}

// 发送队列 overflow 断链须走 CloseFromReactor，不得阻塞 reactor 读栈。
func TestSharedSend_OverflowCloseUsesCloseFromReactor(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	_, srvConn := net.Pipe()
	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}
	ch.SetSendQueueOverflowHook(func(*BaseChannel, int64, int64) SendQueueOverflowAction {
		return OverflowCloseChannel
	})

	ch.onSendQueueOverflow(9000, 8192)
	if !ch.closeReactorAsync.Load() {
		t.Fatal("overflow close on shared send path must use CloseFromReactor")
	}
	if !ch.isClose.Load() {
		t.Fatal("overflow close must mark channel closed")
	}
}

// 读栈内 Close()（非 CloseFromReactor）在共享写路径须同样异步 drain。
func TestClose_FromReactorReadStack_AsyncSharedSend(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ts := newRegressionServer(t)
	ts.SetSharedSendWorkerMode(true)

	clientConn, srvConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })

	ch := NewBaseChannel(ts.NextId(), srvConn, ts)
	if !ts.BindSharedSendHook(ctx, ch) {
		t.Fatal("BindSharedSendHook failed")
	}

	for i := 0; i < 32; i++ {
		m := GetNetMessage()
		m.SetMsgId(1)
		m.SetMessageData([]byte("payload"))
		ch.Send(m)
	}

	ch.BeginReactorRead()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ch.Close()
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Close() inside reactor read stack blocked on shared send worker")
	}
	if !ch.closeReactorAsync.Load() {
		t.Fatal("Close() inside reactor read stack must async shared send drain")
	}
	ch.EndReactorRead()
}
