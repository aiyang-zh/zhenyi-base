package zkcp

import (
	"context"
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/znet"
	"net"
	"sync"
	"testing"
	"time"
)

func kcpFreePort(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("ListenPacket: %v", err)
	}
	addr := pc.LocalAddr().String()
	pc.Close()
	return addr
}

func waitForKListener(server *Server, t *testing.T) string {
	t.Helper()
	for i := 0; i < 50; i++ {
		if server.GetListener() != nil {
			return server.GetAddr()
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("KCP server failed to bind listener")
	return ""
}

func TestKServer_KClient_FullIntegration(t *testing.T) {
	addr := kcpFreePort(t)

	var receivedMsgs []*znet.NetMessage
	var mu sync.Mutex
	server := NewServer(addr, znet.ServerHandlers{
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

	_ = waitForKListener(server, t)
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
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

	time.Sleep(300 * time.Millisecond)
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

func TestKClient_ConnectInvalidAddr(t *testing.T) {
	// Use invalid hostname - KCP/UDP dial fails on DNS resolution
	_, err := NewClient("nonexistent-host.invalid:19999")
	if err == nil {
		t.Error("NewClient to invalid hostname should fail")
	}
}

func TestKServer_Close(t *testing.T) {
	addr := kcpFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	_ = waitForKListener(server, t)
	time.Sleep(50 * time.Millisecond)

	server.Close()
	cancel()
	time.Sleep(50 * time.Millisecond)
}
func TestKCPServer_StartAndClose(t *testing.T) {
	addr := kcpFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	_ = waitForKListener(server, t)
	time.Sleep(50 * time.Millisecond)

	server.Close()
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestKCPServer_Client_SendReceive(t *testing.T) {
	addr := kcpFreePort(t)

	var received []*znet.NetMessage
	var mu sync.Mutex
	server := NewServer(addr, znet.ServerHandlers{
		OnAccept: func(ch ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, msg ziface.IWireMessage) {
			mu.Lock()
			received = append(received, msg.(*znet.NetMessage).Clone())
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	_ = waitForKListener(server, t)
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	client.SetReadCall(func(ziface.IWireMessage) {})
	client.Read()

	time.Sleep(100 * time.Millisecond)

	msg := znet.GetNetMessage()
	defer msg.Release()
	msg.SetMsgId(100)
	msg.SetSeqId(1)
	msg.SetMessageData([]byte("hello"))
	client.SendMsg(msg)

	time.Sleep(300 * time.Millisecond)
	server.Close()
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Errorf("expected 1 message on server, got %d", len(received))
	}
	if len(received) > 0 {
		if received[0].GetMsgId() != 100 {
			t.Errorf("expected msgId 100, got %d", received[0].GetMsgId())
		}
		if string(received[0].GetMessageData()) != "hello" {
			t.Errorf("expected data 'hello', got %q", string(received[0].GetMessageData()))
		}
	}
}

func TestKCPServer_OnAcceptReject(t *testing.T) {
	addr := kcpFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return false }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	_ = waitForKListener(server, t)
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(addr)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client != nil {
		client.Close()
	}
	time.Sleep(100 * time.Millisecond)

	server.Close()
	cancel()
}

func TestKCPServer_Close_Idempotent(t *testing.T) {
	addr := kcpFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	ctx, cancel := context.WithCancel(context.Background())
	server.Server(ctx)

	_ = waitForKListener(server, t)
	time.Sleep(20 * time.Millisecond)

	server.Close()
	server.Close()
	server.Close()
	cancel()
}

func TestKCPClient_InvalidAddress(t *testing.T) {
	// Use invalid hostname - DNS resolution should fail for KCP/UDP dial
	_, err := NewClient("nonexistent-host-xyz.invalid:19999")
	if err == nil {
		t.Error("NewClient to invalid hostname should fail")
	}
}
