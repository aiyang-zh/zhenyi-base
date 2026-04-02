//go:build darwin

package zreactor

import (
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testReactorChannel struct {
	id       uint64
	mu       sync.Mutex
	buf      bytes.Buffer
	parsedCh chan []byte
	closed   atomic.Bool
}

func (c *testReactorChannel) WriteToReadBuffer(p []byte) (n int, err error) {
	c.mu.Lock()
	_, _ = c.buf.Write(p)
	c.mu.Unlock()
	return len(p), nil
}

func (c *testReactorChannel) ParseAndDispatch() bool {
	c.mu.Lock()
	b := c.buf.Bytes()
	// Trigger close once we have enough data.
	// (Test only cares that ParseAndDispatch is called with bytes from syscall.Read.)
	if len(b) >= 5 {
		out := append([]byte(nil), b...)
		// Non-blocking send to avoid goroutine leak.
		select {
		case c.parsedCh <- out:
		default:
		}
		c.mu.Unlock()
		return true
	}
	c.mu.Unlock()
	return false
}

func (c *testReactorChannel) GetChannelId() uint64 { return c.id }

func (c *testReactorChannel) Close() {
	c.closed.Store(true)
}

func TestServe_DarwinKqueue_ReadAndDispatch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	parsed := make(chan []byte, 1)

	acceptFn := func(conn net.Conn) (ReactorChannel, bool) {
		ch := &testReactorChannel{
			id:       1,
			parsedCh: parsed,
		}
		return ch, true
	}

	serveDone := make(chan struct{})
	go func() {
		_ = Serve(ctx, ln, acceptFn, &Metrics{
			OnClose: nil,
		})
		close(serveDone)
	}()

	// Client: connect and send data.
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_, _ = c.Write([]byte("hello"))
	_ = c.Close()

	select {
	case got := <-parsed:
		if len(got) < 5 {
			t.Fatalf("unexpected parsed bytes len=%d: %q", len(got), string(got))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ParseAndDispatch")
	}

	cancel()
	select {
	case <-serveDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Serve to stop")
	}
}
