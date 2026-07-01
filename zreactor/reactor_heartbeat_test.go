//go:build linux || darwin

package zreactor

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type heartbeatTestChannel struct {
	id       uint64
	expireAt atomic.Int64
	closed   atomic.Bool
}

func (c *heartbeatTestChannel) WriteToReadBuffer(p []byte) (int, error) {
	return len(p), nil
}

func (c *heartbeatTestChannel) ParseAndDispatch() bool {
	return false
}

func (c *heartbeatTestChannel) GetChannelId() uint64 { return c.id }

func (c *heartbeatTestChannel) Close() {
	c.closed.Store(true)
}

func (c *heartbeatTestChannel) Check() bool {
	return time.Now().UnixMilli() < c.expireAt.Load()
}

func TestServe_HeartbeatPollClosesIdleConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var chPtr atomic.Pointer[heartbeatTestChannel]
	acceptFn := func(conn net.Conn) (ReactorChannel, bool) {
		ch := &heartbeatTestChannel{id: 1}
		ch.expireAt.Store(time.Now().Add(30 * time.Millisecond).UnixMilli())
		chPtr.Store(ch)
		return ch, true
	}

	serveDone := make(chan struct{})
	go func() {
		_ = ServeWithConfig(ctx, ln, acceptFn, nil, &ServeConfig{
			HeartbeatPollMs: 50,
			BatchRead:       true,
		})
		close(serveDone)
	}()

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ch := chPtr.Load(); ch != nil && ch.closed.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("idle connection should be closed by heartbeat poll")
}
