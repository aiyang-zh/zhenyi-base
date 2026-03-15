package ztcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
)

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

// waitForListener waits for the TServer to bind (up to ~1s)
func waitForListener(server *Server, t *testing.T) string {
	t.Helper()
	for i := 0; i < 50; i++ {
		if server.GetListener() != nil {
			return server.GetAddr()
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server failed to bind listener")
	return ""
}

func TestTServer_TClient_FullIntegration(t *testing.T) {
	var receivedMsgs []*znet.NetMessage
	var mu sync.Mutex
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ch ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, msg ziface.IWireMessage) {
			mu.Lock()
			receivedMsgs = append(receivedMsgs, msg.(*znet.NetMessage).Clone())
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	addr := waitForListener(server, t)
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("NewTClient: %v", err)
	}
	defer client.Close()

	var clientReceived []*znet.NetMessage
	client.SetReadCall(func(msg ziface.IWireMessage) {
		mu.Lock()
		clientReceived = append(clientReceived, msg.(*znet.NetMessage).Clone())
		mu.Unlock()
	})
	client.Read()

	time.Sleep(100 * time.Millisecond)

	msg := znet.GetNetMessage()
	defer msg.Release()
	msg.SetMsgId(100)
	msg.SetSeqId(1)
	msg.SetMessageData([]byte("hello"))
	client.SendMsg(msg)

	time.Sleep(200 * time.Millisecond)
	server.Close()
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(receivedMsgs) != 1 {
		t.Errorf("expected 1 message on server, got %d", len(receivedMsgs))
	}
	if len(receivedMsgs) > 0 {
		if receivedMsgs[0].GetMsgId() != 100 {
			t.Errorf("expected msgId 100, got %d", receivedMsgs[0].GetMsgId())
		}
		if string(receivedMsgs[0].GetMessageData()) != "hello" {
			t.Errorf("expected data 'hello', got %q", string(receivedMsgs[0].GetMessageData()))
		}
	}
}

func TestTChannel_DirectTCPConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	var gotMsg atomic.Bool
	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, msg ziface.IWireMessage) {
			if msg.GetMsgId() == 100 && string(msg.GetMessageData()) == "direct" {
				gotMsg.Store(true)
			}
		},
	})
	server.SetEncrypt(zencrypt.NewBaseEncrypt())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	acceptDone := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		acceptDone <- conn
	}()

	clientConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	serverConn := <-acceptDone
	if serverConn == nil {
		t.Fatal("accept failed")
	}

	channelId := server.NextId()
	ch := NewChannel(channelId, serverConn, server)
	server.AddChannel(ch)
	go func() {
		ch.StartSend(ctx)
		ch.Start()
	}()
	defer ch.Close()

	packet := makeTestPacket(100, 1, []byte("direct"))
	_, err = clientConn.Write(packet)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if !gotMsg.Load() {
		t.Error("server did not receive message")
	}
}

func TestBaseClient_FullOperations(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	serverReceivedCh := make(chan []byte, 1)
	go func() {
		conn, _ := ln.Accept()
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		serverReceivedCh <- buf[:n]
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	client := znet.NewBaseClient(znet.WithAsyncMode())
	client.SetConn(conn)
	if !client.IsOpen() {
		t.Error("IsOpen should be true")
	}
	if client.GetConn() != conn {
		t.Error("GetConn should return set conn")
	}

	client.SetReadCall(func(ziface.IWireMessage) {})
	client.Read()

	msg := znet.GetNetMessage()
	msg.SetMsgId(200)
	msg.SetSeqId(2)
	msg.SetMessageData([]byte("baseclient"))
	client.SendMsg(msg)

	time.Sleep(150 * time.Millisecond)
	client.Close()

	if client.IsOpen() {
		t.Error("IsOpen should be false after Close")
	}

	serverReceived := <-serverReceivedCh
	if len(serverReceived) < 12 {
		t.Errorf("expected server to receive packet, got %d bytes", len(serverReceived))
	}
	msgId := binary.BigEndian.Uint32(serverReceived[0:4])
	if msgId != 200 {
		t.Errorf("expected msgId 200, got %d", msgId)
	}
}

func TestTServer_OnAcceptReject(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return false },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	addr := waitForListener(server, t)
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	server.Close()
	cancel()
}

func TestTServer_ChannelHeartbeatTimeout(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	server.SetHeartbeatTimeout(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	addr := waitForListener(server, t)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	conn.Close()
	server.Close()
	cancel()
}

func TestTChannel_CloseCallback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})

	var closeCalled atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		conn, _ := ln.Accept()
		ch := NewChannel(server.NextId(), conn, server)
		ch.SetCloseCall(func(ziface.IChannel) { closeCalled.Store(true) })
		server.AddChannel(ch)
		ch.StartSend(ctx)
		ch.Start()
	}()

	conn, _ := net.Dial("tcp", ln.Addr().String())
	conn.Close()

	time.Sleep(200 * time.Millisecond)
	if !closeCalled.Load() {
		t.Error("SetCloseCall callback should be invoked on disconnect")
	}
}

func TestTServer_MultipleChannels(t *testing.T) {
	var count int32
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ch ziface.IChannel, msg ziface.IWireMessage) { atomic.AddInt32(&count, 1) },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	addr := waitForListener(server, t)
	time.Sleep(50 * time.Millisecond)

	c1, err := NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("client1: %v", err)
	}
	defer c1.Close()
	c1.SetReadCall(func(ziface.IWireMessage) {})
	c1.Read()

	c2, err := NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("client2: %v", err)
	}
	defer c2.Close()
	c2.SetReadCall(func(ziface.IWireMessage) {})
	c2.Read()

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		msg := znet.GetNetMessage()
		msg.SetMsgId(int32(100 + i))
		msg.SetSeqId(uint32(i))
		msg.SetMessageData([]byte("hi"))
		c1.SendMsg(msg)
		c2.SendMsg(msg)
	}

	time.Sleep(300 * time.Millisecond)
	server.Close()
	cancel()

	if atomic.LoadInt32(&count) < 6 {
		t.Errorf("expected at least 6 messages, got %d", atomic.LoadInt32(&count))
	}
}

func TestParseData_ResetForReuse_ViaChannelRead(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	var msgs []int32
	var mu sync.Mutex
	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, msg ziface.IWireMessage) {
			mu.Lock()
			msgs = append(msgs, msg.GetMsgId())
			mu.Unlock()
		},
	})
	server.SetEncrypt(zencrypt.NewBaseEncrypt())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		conn, _ := ln.Accept()
		ch := NewChannel(server.NextId(), conn, server)
		server.AddChannel(ch)
		ch.StartSend(ctx)
		ch.Start()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	for i := 0; i < 5; i++ {
		conn.Write(makeTestPacket(uint32(100+i), uint32(i), []byte("x")))
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	n := len(msgs)
	mu.Unlock()
	if n != 5 {
		t.Errorf("expected 5 messages (resetForReuse exercised), got %d", n)
	}
}

func TestTChannel_WriteBuffers_SendBatchMsg(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chReady := make(chan ziface.IChannel, 1)
	go func() {
		conn, _ := ln.Accept()
		ch := NewChannel(server.NextId(), conn, server)
		server.AddChannel(ch)
		chReady <- ch
		ch.StartSend(ctx)
		ch.Start()
	}()

	clientConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	ch := <-chReady
	if ch == nil {
		t.Fatal("channel not ready")
	}

	msg := znet.GetNetMessage()
	defer msg.Release()
	msg.MsgId = 1
	msg.SeqId = 10
	msg.Data = []byte("batch")
	ch.(*Channel).SendBatchMsg([]ziface.IMessage{msg})

	buf := make([]byte, 256)
	clientConn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n < 12 {
		t.Errorf("expected >= 12 bytes, got %d", n)
	}
	cancel()
}

func TestTServer_SetMaxConnections_Reject(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	server.SetMaxConnections(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	addr := waitForListener(server, t)
	time.Sleep(50 * time.Millisecond)

	c1, err := NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("client1: %v", err)
	}
	defer c1.Close()
	c1.SetReadCall(func(ziface.IWireMessage) {})
	c1.Read()

	time.Sleep(50 * time.Millisecond)

	c2, err := NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("client2: %v", err)
	}
	if c2 != nil {
		c2.Close()
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	server.Close()
	cancel()
}

func TestTServer_SetChannelAuth_GetChannelByAuthId_RealChannel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		conn, _ := ln.Accept()
		ch := NewChannel(server.NextId(), conn, server)
		server.AddChannel(ch)
		server.SetChannelAuth(ch.GetChannelId(), 7777)
		ch.StartSend(ctx)
		ch.Start()
	}()

	_, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	ch := server.GetChannelByAuthId(7777)
	if ch == nil {
		t.Error("GetChannelByAuthId(7777) should return channel")
	}
	if ch != nil && ch.GetAuthId() != 7777 {
		t.Errorf("GetAuthId: expected 7777, got %d", ch.GetAuthId())
	}
	cancel()
}

func TestBaseClient_SendMsg_WhenClosed(t *testing.T) {
	client := znet.NewBaseClient()
	client.Close()

	msg := znet.GetNetMessage()
	msg.SetMsgId(1)
	msg.SetSeqId(1)
	msg.SetMessageData([]byte("x"))
	client.SendMsg(msg)
	msg.Release()
}

func TestBaseClient_SetEncrypt(t *testing.T) {
	client := znet.NewBaseClient()
	client.SetEncrypt(nil)
	_ = client
}

func TestBaseClient_Read_ConnectionClosed(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := znet.NewBaseClient(znet.WithAsyncMode())
	client.SetConn(clientConn)
	client.SetReadCall(func(ziface.IWireMessage) {})

	go func() {
		client.Read()
	}()

	time.Sleep(30 * time.Millisecond)
	serverConn.Close()
	clientConn.Close()

	time.Sleep(100 * time.Millisecond)
}

func TestNewTClient_InvalidAddress(t *testing.T) {
	_, err := NewClient("127.0.0.1:37923")
	if err == nil {
		t.Error("NewTClient to unused port should fail")
	}
}

func TestBaseServer_SetChannelAuth_NonExistentChannel(t *testing.T) {
	s := znet.NewBaseServer(":0", znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	s.SetChannelAuth(99999, 1)
	if s.GetChannelByAuthId(1) != nil {
		t.Error("should be nil when channel does not exist")
	}
}

func TestTServer_Listen_ContextDone(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ctx, cancel := context.WithCancel(context.Background())
	server.Server(ctx)
	waitForListener(server, t)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	server.Close()
}

func TestTChannel_MalformedPacket_ParseError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	defer ln.Close()

	server := NewServer(ln.Addr().String(), znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chReady := make(chan ziface.IChannel, 1)
	go func() {
		conn, _ := ln.Accept()
		ch := NewChannel(server.NextId(), conn, server)
		ch.SetCloseCall(func(ziface.IChannel) {})
		server.AddChannel(ch)
		chReady <- ch
		ch.StartSend(ctx)
		ch.Start()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	<-chReady
	malformed := make([]byte, 12)
	binary.BigEndian.PutUint32(malformed[0:4], 1)
	binary.BigEndian.PutUint32(malformed[4:8], 0)
	binary.BigEndian.PutUint32(malformed[8:12], 0xFFFFFFFF)
	conn.Write(malformed)

	time.Sleep(150 * time.Millisecond)
}

func TestTServer_Close_Idempotent(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ctx, cancel := context.WithCancel(context.Background())
	server.Server(ctx)
	waitForListener(server, t)
	time.Sleep(20 * time.Millisecond)

	server.Close()
	server.Close()
	server.Close()
	cancel()
}

// generateSelfSignedCert 生成自签名 RSA/ECDSA 证书用于测试
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

// NewStandardTLSConfigFromPEM 从 PEM 字节创建标准 TLS 配置。
func NewStandardTLSConfigFromPEM(certPEM, keyPEM []byte) (*ziface.TLSConfig, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.New("tls: failed to parse certificate: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeStandard,
		StdConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}, nil
}
func TestTServer_WithTLS(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)
	tlsCfg, err := NewStandardTLSConfigFromPEM(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	type msgSnapshot struct {
		msgId int32
		data  []byte
	}
	received := make(chan msgSnapshot, 1)
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ch ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, msg ziface.IWireMessage) {
			snap := msgSnapshot{
				msgId: msg.GetMsgId(),
				data:  append([]byte(nil), msg.GetMessageData()...),
			}
			received <- snap
		},
	})
	server.SetTLSConfig(tlsCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	time.Sleep(100 * time.Millisecond)

	addr := server.GetAddr()

	clientCfg := znet.NewClientStandardTLSConfig()
	conn, err := znet.DialTLS("tcp", addr, clientCfg)
	if err != nil {
		t.Fatalf("DialTLS to TServer failed: %v", err)
	}
	defer conn.Close()

	client := znet.NewBaseClient()
	client.SetConn(conn)
	client.SetReadCall(func(ziface.IWireMessage) {})

	msg := znet.GetNetMessage()
	msg.SetMsgId(1)
	msg.SetMessageData([]byte("tls test"))
	client.SendMsg(msg)
	msg.Release()

	select {
	case got := <-received:
		if got.msgId != 1 {
			t.Fatalf("msgId: got %d, want 1", got.msgId)
		}
		if string(got.data) != "tls test" {
			t.Fatalf("data: got %q, want %q", got.data, "tls test")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message over TLS")
	}
}
