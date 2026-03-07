package zws

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
)

func wsFreePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestWServer_WClient_FullIntegration(t *testing.T) {
	addr := wsFreePort(t)

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
	time.Sleep(200 * time.Millisecond)

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

func TestWClient_ConnectInvalidAddr(t *testing.T) {
	_, err := NewClient("127.0.0.1:37923")
	if err == nil {
		t.Error("NewClient to unused port should fail")
	}
}

func TestWServer_Close_Idempotent(t *testing.T) {
	addr := wsFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)
	time.Sleep(200 * time.Millisecond)

	server.Close()
	server.Close()
	server.Close()
	cancel()
}

func TestWServer_OnAcceptReject(t *testing.T) {
	addr := wsFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return false }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)
	time.Sleep(200 * time.Millisecond)

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

func TestWServer_StartAndClose(t *testing.T) {
	addr := wsFreePort(t)

	server := NewServer(addr, znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Server(ctx)

	time.Sleep(200 * time.Millisecond)

	server.Close()
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestWServer_Client_SendReceive(t *testing.T) {
	addr := wsFreePort(t)

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

	time.Sleep(200 * time.Millisecond)

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
	msg.SetMsgId(200)
	msg.SetSeqId(2)
	msg.SetMessageData([]byte("ws-hello"))
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
		if received[0].GetMsgId() != 200 {
			t.Errorf("expected msgId 200, got %d", received[0].GetMsgId())
		}
		if string(received[0].GetMessageData()) != "ws-hello" {
			t.Errorf("expected data 'ws-hello', got %q", string(received[0].GetMessageData()))
		}
	}
}

func TestWServer_CheckOrigin(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{OnAccept: func(ziface.IChannel) bool { return true }, OnRead: func(ziface.IChannel, ziface.IWireMessage) {}})
	if !server.CheckOrigin(nil) {
		t.Error("CheckOrigin should return true")
	}
}

func TestWClient_InvalidAddress(t *testing.T) {
	// Use invalid hostname - DNS resolution should fail
	_, err := NewClient("nonexistent-host-xyz.invalid:19998")
	if err == nil {
		t.Error("NewClient to invalid hostname should fail")
	}
}
