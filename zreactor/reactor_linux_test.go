//go:build linux

// 本文件测试需 Linux 环境（epoll、TCP）；压力/高连接数稳定性建议在 Linux 上单独跑 benchmark 或集成测试。
package zreactor

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

func TestServe_RequiresTCPListener(t *testing.T) {
	ctx := context.Background()
	err := Serve(ctx, &fakeListener{}, func(net.Conn) (ReactorChannel, bool) { return nil, false }, nil)
	if err == nil {
		t.Fatal("expected errNotTCPListener")
	}
	if !zerrs.IsValidation(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

type fakeListener struct{}

func (fakeListener) Accept() (net.Conn, error) { return nil, nil }
func (fakeListener) Addr() net.Addr            { return nil }
func (fakeListener) Close() error              { return nil }

func TestServe_AcceptOneThenShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no tcp listen:", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var acceptCount, closeCount int
	var mu sync.Mutex
	metrics := &Metrics{
		OnAccept: func() { mu.Lock(); acceptCount++; mu.Unlock() },
		OnClose:  func() { mu.Lock(); closeCount++; mu.Unlock() },
	}
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ln, func(conn net.Conn) (ReactorChannel, bool) {
			// 接受并加入 reactor，OnAccept 会在加入 fdMap 后调用
			return &fakeChannel{conn: conn}, true
		}, metrics)
	}()
	time.Sleep(10 * time.Millisecond)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close() // 客户端关闭后，reactor 会 closeConn 并调用 OnClose
	time.Sleep(50 * time.Millisecond)
	cancel()
	err = <-done
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	ac := acceptCount
	mu.Unlock()
	if ac < 1 {
		t.Errorf("expected at least 1 accept, got %d", ac)
	}
}

// TestServeWithConfig_BatchRead 覆盖批量读路径：BatchRead=true 时接受一连接后关闭，无 panic。
func TestServeWithConfig_BatchRead(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no tcp listen:", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := &ServeConfig{BatchRead: true}
	done := make(chan error, 1)
	go func() {
		done <- ServeWithConfig(ctx, ln, func(conn net.Conn) (ReactorChannel, bool) {
			conn.Close()
			return nil, false
		}, nil, cfg)
	}()
	time.Sleep(10 * time.Millisecond)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// TestServeWithConfig_MaxConns 连接数达到 MaxConns 时不再接受新连接（不调用 accept，客户端会阻塞直至超时或服务端关闭）。
func TestServeWithConfig_MaxConns(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no tcp listen:", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var acceptCount int
	var mu sync.Mutex
	metrics := &Metrics{
		OnAccept: func() { mu.Lock(); acceptCount++; mu.Unlock() },
	}
	cfg := &ServeConfig{MaxConns: 1, BatchRead: false}
	done := make(chan error, 1)
	go func() {
		done <- ServeWithConfig(ctx, ln, func(conn net.Conn) (ReactorChannel, bool) {
			// 接受但不关闭连接，占满 MaxConns=1；OnAccept 会递增 acceptCount
			ch := &fakeChannel{conn: conn}
			return ch, true
		}, metrics, cfg)
	}()
	time.Sleep(10 * time.Millisecond)
	conn1, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	ac := acceptCount
	mu.Unlock()
	if ac != 1 {
		t.Errorf("expected 1 accept after first dial, got %d", ac)
	}
	cancel()
	_ = <-done
}

type fakeChannel struct {
	conn net.Conn
}

func (f *fakeChannel) WriteToReadBuffer(p []byte) (n int, err error) { return len(p), nil }
func (f *fakeChannel) ParseAndDispatch() bool                        { return false }
func (f *fakeChannel) GetChannelId() uint64                          { return 0 }
func (f *fakeChannel) Close()                                        { /* reactor 已关闭 conn，避免重复关闭 */ }

// panicChannel 用于测试 ParseAndDispatch panic 时 reactor 恢复并关闭该连接，不拖垮进程。
type panicChannel struct {
	conn net.Conn
	id   uint64
}

func (p *panicChannel) WriteToReadBuffer(b []byte) (n int, err error) { return len(b), nil }
func (p *panicChannel) ParseAndDispatch() bool {
	panic("test panic recovery")
}
func (p *panicChannel) GetChannelId() uint64 { return p.id }
func (p *panicChannel) Close()               {}

// TestParseAndDispatch_PanicRecoveryClosesConn 验证单连接 ParseAndDispatch panic 时，
// reactor 恢复后关闭该连接并调用 OnClose，进程不退出。
func TestParseAndDispatch_PanicRecoveryClosesConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no tcp listen:", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var closeCount int
	var mu sync.Mutex
	metrics := &Metrics{
		OnClose: func() { mu.Lock(); closeCount++; mu.Unlock() },
	}

	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ln, func(conn net.Conn) (ReactorChannel, bool) {
			return &panicChannel{conn: conn, id: 1}, true
		}, metrics)
	}()
	time.Sleep(20 * time.Millisecond)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Write([]byte("x"))
	_ = conn.Close()

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	cc := closeCount
	mu.Unlock()
	if cc < 1 {
		t.Errorf("expected OnClose >= 1 after ParseAndDispatch panic, got %d", cc)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
