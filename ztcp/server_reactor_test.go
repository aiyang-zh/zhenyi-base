//go:build linux || darwin

package ztcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
)

func TestTServer_Reactor_HeartbeatTimeout(t *testing.T) {
	server := NewServer("127.0.0.1:0", znet.ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead:   func(ziface.IChannel, ziface.IWireMessage) {},
	})
	server.SetHeartbeatTimeout(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ServerReactor(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := waitForListener(server, t)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := conn.Read(make([]byte, 1))
		if err != nil {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("reactor path should close idle connection on heartbeat timeout")
}
