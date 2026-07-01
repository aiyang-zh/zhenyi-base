package znet

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zpool"
)

type errEncrypt struct {
	encErr error
	decErr error
}

func (e errEncrypt) Encrypt([]byte) ([]byte, error) {
	if e.encErr != nil {
		return nil, e.encErr
	}
	return []byte("enc"), nil
}

func (e errEncrypt) Decrypt([]byte) ([]byte, error) {
	if e.decErr != nil {
		return nil, e.decErr
	}
	return []byte("dec"), nil
}

type timeoutWriteConn struct {
	net.Conn
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func (c timeoutWriteConn) Write(b []byte) (int, error) {
	return 0, &net.OpError{Op: "write", Err: timeoutErr{}}
}

type failReadConn struct {
	net.Conn
	err error
}

func (c failReadConn) Read([]byte) (int, error) {
	return 0, c.err
}

func writeWireMessage(conn net.Conn, msg *NetMessage) error {
	socket := NewBaseSocket()
	var headerBuf [13]byte
	headerLen, body := socket.PreparePacket(msg, headerBuf[:])
	if _, err := conn.Write(headerBuf[:headerLen]); err != nil {
		return err
	}
	if len(body) > 0 {
		_, err := conn.Write(body)
		return err
	}
	return nil
}

func readOneWirePacket(conn net.Conn) (msgId int32, body []byte, err error) {
	hdr := make([]byte, 12)
	if _, err = io.ReadFull(conn, hdr); err != nil {
		return 0, nil, err
	}
	msgId = int32(binary.BigEndian.Uint32(hdr[0:4]))
	dataLen := binary.BigEndian.Uint32(hdr[8:12])
	if dataLen == 0 {
		return msgId, nil, nil
	}
	body = make([]byte, dataLen)
	_, err = io.ReadFull(conn, body)
	return msgId, body, err
}

func saveSendLoopTuning(t *testing.T) func() {
	t.Helper()
	old := GetSendLoopTuning()
	return func() { SetSendLoopTuning(old) }
}

func TestBaseClient_GetConn_And_LoadConn(t *testing.T) {
	clientConn, _ := net.Pipe()
	defer clientConn.Close()

	client := NewBaseClient()
	client.SetConn(clientConn)
	if client.GetConn() != clientConn {
		t.Fatal("GetConn should return active connection")
	}
	_ = client.Close()
	if client.GetConn() != nil {
		t.Fatal("GetConn should be nil after Close")
	}
}

func TestBaseClient_Close_Idempotent_AndOwnedBuffers(t *testing.T) {
	client := NewBaseClient()
	buf := zpool.GetBytesBuffer(4)
	client.parseData.OwnedBuffers = append(client.parseData.OwnedBuffers, buf)

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if len(client.parseData.OwnedBuffers) != 0 {
		t.Fatalf("expected OwnedBuffers cleared")
	}
}

func TestBaseClient_SetConn_SyncDoesNotStartRunSend(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	client := NewBaseClient()
	client.SetConn(clientConn)

	msg := GetNetMessage()
	msg.SetMsgId(1)
	msg.SetMessageData([]byte("sync"))
	client.SendMsgAsync(msg) // sync mode: dropped

	time.Sleep(20 * time.Millisecond)
	serverConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	if _, err := serverConn.Read(make([]byte, 64)); err == nil {
		t.Fatal("sync client should not run async send loop")
	}
	_ = client.Close()
}

func TestBaseClient_SendMsg_SyncAndAsync(t *testing.T) {
	t.Run("sync", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer serverConn.Close()

		client := NewBaseClient()
		client.SetConn(clientConn)
		defer client.Close()

		done := make(chan struct{})
		go func() {
			msgId, body, err := readOneWirePacket(serverConn)
			if err != nil {
				t.Errorf("read: %v", err)
			} else if msgId != 7 || string(body) != "sync" {
				t.Errorf("got msgId=%d body=%q", msgId, body)
			}
			close(done)
		}()

		in := &NetMessage{MsgId: 7, SeqId: 1, Data: []byte("sync")}
		client.SendMsg(in)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting sync SendMsg")
		}
	})

	t.Run("async", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer serverConn.Close()

		client := NewBaseClient(WithAsyncMode())
		client.SetConn(clientConn)
		defer client.Close()

		in := GetNetMessage()
		in.SetMsgId(8)
		in.SetMessageData([]byte("async"))
		client.SendMsg(in)
		in.Release()

		msgId, body, err := readOneWirePacket(serverConn)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msgId != 8 || string(body) != "async" {
			t.Fatalf("got msgId=%d body=%q", msgId, body)
		}
	})
}

func TestBaseClient_SendMsg_EdgeCases(t *testing.T) {
	client := NewBaseClient()
	client.SendMsg(nil)

	client.Close()
	msg := GetNetMessage()
	client.SendMsg(msg)
	msg.Release()

	client2 := NewBaseClient()
	client2.SendMsg(&NetMessage{MsgId: 1, Data: []byte("x")})
	_ = client2.Close()
}

func TestBaseClient_SendMsgAsync_AsyncOnly(t *testing.T) {
	client := NewBaseClient()
	msg := GetNetMessage()
	client.SendMsgAsync(msg)

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	asyncClient := NewBaseClient(WithAsyncMode())
	asyncClient.SetConn(clientConn)
	defer asyncClient.Close()

	out := GetNetMessage()
	out.SetMsgId(3)
	out.SetMessageData([]byte("a"))
	asyncClient.SendMsgAsync(out)

	msgId, _, err := readOneWirePacket(serverConn)
	if err != nil || msgId != 3 {
		t.Fatalf("async enqueue write: msgId=%d err=%v", msgId, err)
	}

	asyncClient.Close()
	late := GetNetMessage()
	asyncClient.SendMsgAsync(late)
}

func TestBaseClient_SendMsgAsync_EnqueueFailAfterStop(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)

	msg := GetNetMessage()
	msg.SetMsgId(9)
	client.mailBoxQueue.StopEnqueue()
	client.SendMsgAsync(msg)
	_ = client.Close()
}

func TestBaseClient_Async_BatchSend_WithEncrypt(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	go drainWirePackets(serverConn)

	client := NewBaseClient(WithAsyncMode())
	client.SetEncrypt(zencrypt.NewBaseEncrypt())
	client.SetConn(clientConn)
	defer client.Close()

	for i := 0; i < 5; i++ {
		m := GetNetMessage()
		m.SetMsgId(int32(100 + i))
		m.SetMessageData([]byte("batch"))
		client.SendMsgAsync(m)
	}
	time.Sleep(80 * time.Millisecond)
	_ = client.Close()
}

func drainWirePackets(conn net.Conn) {
	for {
		if _, _, err := readOneWirePacket(conn); err != nil {
			return
		}
	}
}

func TestBaseClient_Request_Success_AndErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer serverConn.Close()

		client := NewBaseClient()
		client.SetEncrypt(zencrypt.NewBaseEncrypt())
		client.SetConn(clientConn)
		defer client.Close()

		go func() {
			if _, _, err := readOneWirePacket(serverConn); err != nil {
				return
			}
			_ = writeWireMessage(serverConn, &NetMessage{MsgId: 2, SeqId: 1, Data: []byte("pong")})
		}()

		resp, err := client.Request(&NetMessage{MsgId: 1, SeqId: 1, Data: []byte("ping")})
		if err != nil {
			t.Fatalf("Request: %v", err)
		}
		if resp.GetMsgId() != 2 || string(resp.GetMessageData()) != "pong" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("decrypt error", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer serverConn.Close()

		client := NewBaseClient()
		client.SetEncrypt(errEncrypt{decErr: errors.New("decrypt fail")})
		client.SetConn(clientConn)
		defer client.Close()

		go func() {
			_, _, _ = readOneWirePacket(serverConn)
			_ = writeWireMessage(serverConn, &NetMessage{MsgId: 2, Data: []byte("x")})
		}()

		_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("ping")})
		if err == nil || !strings.Contains(err.Error(), "decrypt fail") {
			t.Fatalf("expected decrypt error, got %v", err)
		}
	})

	t.Run("header eof", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		client := NewBaseClient()
		client.SetConn(clientConn)
		go func() {
			_, _, _ = readOneWirePacket(serverConn)
			_ = serverConn.Close()
		}()
		_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
		if err != io.EOF {
			t.Fatalf("expected EOF, got %v", err)
		}
		_ = client.Close()
	})

	t.Run("not connected", func(t *testing.T) {
		client := NewBaseClient()
		_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
		if err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("expected not connected, got %v", err)
		}
	})

	t.Run("closed", func(t *testing.T) {
		client := NewBaseClient()
		_ = client.Close()
		_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
		if err != io.EOF {
			t.Fatalf("expected EOF, got %v", err)
		}
	})
}

func TestBaseClient_writeImmediate_Errors(t *testing.T) {
	t.Run("encrypt error", func(t *testing.T) {
		clientConn, _ := net.Pipe()
		client := NewBaseClient()
		client.SetEncrypt(errEncrypt{encErr: errors.New("enc fail")})
		client.SetConn(clientConn)
		client.writeImmediate(&NetMessage{MsgId: 1, Data: []byte("x")}, clientConn)
		_ = client.Close()
	})

	t.Run("write error", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		_ = serverConn.Close()
		client := NewBaseClient()
		client.SetConn(clientConn)
		client.writeImmediate(&NetMessage{MsgId: 1, Data: []byte("x")}, clientConn)
		_ = client.Close()
	})
}

func TestBaseClient_sendBatchMsg_Paths(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		client := NewBaseClient(WithAsyncMode())
		client.sendBatchMsg(nil)
	})

	t.Run("encrypt fail skip", func(t *testing.T) {
		clientConn, _ := net.Pipe()
		client := NewBaseClient(WithAsyncMode())
		client.SetEncrypt(errEncrypt{encErr: errors.New("enc")})
		client.activeConn.Store(clientConn)
		msg := GetNetMessage()
		msg.SetMsgId(1)
		msg.SetMessageData([]byte("x"))
		client.sendBatchMsg([]ziface.IMessage{msg})
		msg.Release()
		_ = clientConn.Close()
	})

	t.Run("write timeout", func(t *testing.T) {
		c1, c2 := net.Pipe()
		client := NewBaseClient(WithAsyncMode())
		client.activeConn.Store(timeoutWriteConn{Conn: c2})
		msg := GetNetMessage()
		msg.SetMsgId(1)
		msg.SetMessageData([]byte("x"))
		client.sendBatchMsg([]ziface.IMessage{msg})
		msg.Release()
		_ = c1.Close()
	})

	t.Run("normal close write", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		_ = serverConn.Close()
		client := NewBaseClient(WithAsyncMode())
		client.activeConn.Store(clientConn)
		msg := GetNetMessage()
		msg.SetMsgId(1)
		msg.SetMessageData([]byte("x"))
		client.sendBatchMsg([]ziface.IMessage{msg})
		msg.Release()
		_ = clientConn.Close()
	})

	t.Run("closed client", func(t *testing.T) {
		client := NewBaseClient(WithAsyncMode())
		msg := GetNetMessage()
		client.state.Store(false)
		client.sendBatchMsg([]ziface.IMessage{msg})
		msg.Release()
	})

	t.Run("grow headers and bufs", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer serverConn.Close()
		go func() {
			for {
				if _, _, err := readOneWirePacket(serverConn); err != nil {
					return
				}
			}
		}()
		client := NewBaseClient(WithAsyncMode())
		client.headersBuf = nil
		client.writeBufs = nil
		client.activeConn.Store(clientConn)
		msgs := make([]ziface.IMessage, 0, 3)
		for i := 0; i < 3; i++ {
			m := GetNetMessage()
			m.SetMsgId(int32(i))
			m.SetMessageData([]byte("grow"))
			msgs = append(msgs, m)
		}
		client.sendBatchMsg(msgs)
		for _, m := range msgs {
			m.Release()
		}
		_ = clientConn.Close()
	})
}

func TestBaseClient_runSend_IdleShrink(t *testing.T) {
	restore := saveSendLoopTuning(t)
	defer restore()
	SetSendLoopTuning(SendLoopTuning{
		BatchMin:        1,
		BatchMax:        4,
		BatchTargetMean: time.Millisecond,
		MaxBatchLimit:   4,
		BackoffFirst:    1,
		BackoffSecond:   1,
		BackoffSleep:    time.Microsecond,
		IdleShrinkAfter: 1,
		IdleShrinkEvery: time.Nanosecond,
	})

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)
	time.Sleep(50 * time.Millisecond)
	_ = client.Close()
}

func TestBaseClient_Async_CloseDrainsQueue(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	var drained atomic.Int32
	go func() {
		for {
			if _, _, err := readOneWirePacket(serverConn); err != nil {
				return
			}
			drained.Add(1)
		}
	}()

	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)

	const n = 20
	for i := 0; i < n; i++ {
		m := GetNetMessage()
		m.SetMsgId(int32(i))
		m.SetMessageData([]byte("q"))
		client.SendMsgAsync(m)
	}
	time.Sleep(150 * time.Millisecond)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if drained.Load() == 0 {
		t.Fatal("expected drained messages before Close completed")
	}
}

func TestBaseClient_Async_ConcurrentSendMsg(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	go drainWirePackets(serverConn)

	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)

	var sent atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			m := GetNetMessage()
			m.SetMsgId(int32(id))
			m.SetMessageData([]byte("c"))
			client.SendMsgAsync(m)
			sent.Add(1)
		}(i)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	_ = client.Close()
	if sent.Load() != 8 {
		t.Fatalf("expected 8 sends, got %d", sent.Load())
	}
}

func TestBaseClient_read_DecryptError_AndReadError(t *testing.T) {
	t.Run("decrypt error continues", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		client := NewBaseClient()
		client.SetEncrypt(errEncrypt{decErr: errors.New("dec")})
		client.SetConn(clientConn)

		go func() {
			_ = writeWireMessage(serverConn, &NetMessage{MsgId: 1, Data: []byte("bad")})
			_ = serverConn.Close()
		}()

		if client.read() == 0 {
			client.read()
		}
		_ = client.Close()
	})

	t.Run("non op read error", func(t *testing.T) {
		c1, c2 := net.Pipe()
		client := NewBaseClient()
		client.activeConn.Store(failReadConn{Conn: c2, err: errors.New("read boom")})
		if client.read() == 0 {
			t.Fatal("expected read to exit with error")
		}
		_ = c1.Close()
		_ = client.Close()
	})

	t.Run("op error read", func(t *testing.T) {
		c1, c2 := net.Pipe()
		client := NewBaseClient()
		client.activeConn.Store(failReadConn{Conn: c2, err: &net.OpError{Op: "read", Err: errors.New("weird")}})
		if client.read() == 0 {
			t.Fatal("expected read to exit")
		}
		_ = c1.Close()
		_ = client.Close()
	})
}

func TestBaseClient_read_NormalCloseOnRead(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := NewBaseClient()
	client.SetConn(clientConn)

	go func() {
		_ = serverConn.Close()
	}()
	if client.read() == 0 {
		t.Fatal("expected exit on closed connection")
	}
	_ = client.Close()
}

type errWriteConn struct {
	net.Conn
	err error
}

func (c errWriteConn) Write([]byte) (int, error) {
	return 0, c.err
}

func TestBaseClient_Close_ClosesConnAfterStateFlip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := NewBaseClient(WithAsyncMode())
	client.SetConn(clientConn)

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, serverConn)
		close(done)
	}()

	_ = client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server side should observe closed connection")
	}
}

func TestBaseClient_sendBatchMsg_WriteError(t *testing.T) {
	client := NewBaseClient(WithAsyncMode())
	c1, c2 := net.Pipe()
	client.activeConn.Store(errWriteConn{Conn: c2, err: errors.New("write boom")})
	msg := GetNetMessage()
	msg.SetMsgId(1)
	msg.SetMessageData([]byte("x"))
	client.sendBatchMsg([]ziface.IMessage{msg})
	msg.Release()
	_ = c1.Close()
}

func TestBaseClient_Request_BodyReadEOF(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := NewBaseClient()
	client.SetConn(clientConn)
	defer client.Close()

	go func() {
		_, _, _ = readOneWirePacket(serverConn)
		hdr := []byte{0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 5}
		_, _ = serverConn.Write(hdr)
		_ = serverConn.Close()
	}()

	_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
	if err != io.EOF {
		t.Fatalf("expected EOF reading body, got %v", err)
	}
}

func TestBaseClient_Request_RejectsMinInt32MsgId(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	client := NewBaseClient()
	client.SetConn(clientConn)

	go func() {
		reqHdr := make([]byte, 12)
		if _, err := io.ReadFull(serverConn, reqHdr); err != nil {
			return
		}
		dataLen := binary.BigEndian.Uint32(reqHdr[8:12])
		if dataLen > 0 {
			body := make([]byte, dataLen)
			if _, err := io.ReadFull(serverConn, body); err != nil {
				return
			}
		}
		resp := make([]byte, 12)
		resp[0] = 0x80 // big-endian int32 0x80000000（math.MinInt32）
		_, _ = serverConn.Write(resp)
	}()

	_, err := client.Request(&NetMessage{MsgId: 1, Data: []byte("x")})
	if err == nil {
		t.Fatal("expected msgId out of range error for MinInt32")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseClient_read_NilBuffer(t *testing.T) {
	client := NewBaseClient()
	clientConn, _ := net.Pipe()
	client.activeConn.Store(clientConn)
	client.readBuffer = nil
	if client.read() == 0 {
		t.Fatal("expected exit when readBuffer nil")
	}
}

func TestBaseClient_writeImmediate_Closed(t *testing.T) {
	client := NewBaseClient()
	client.state.Store(false)
	client.writeImmediate(&NetMessage{MsgId: 1, Data: []byte("x")}, nil)
}

func TestBaseClient_StartSend_Once(t *testing.T) {
	client := NewBaseClient(WithAsyncMode())
	client.StartSend()
	client.StartSend()
	_ = client.Close()
}
