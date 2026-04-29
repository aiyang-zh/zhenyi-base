package zgrace

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Grace 单元测试
// ============================================================

func TestGrace_New(t *testing.T) {
	m := New()
	if m.ch == nil {
		t.Error("channel should be initialized")
	}
	if m.fs == nil {
		t.Error("function slice should be initialized")
	}
}

func TestGrace_Register(t *testing.T) {
	m := New()
	called := false
	m.Register(func(ctx context.Context) {
		_ = ctx
		called = true
	})

	if len(m.fs) != 1 {
		t.Errorf("expected 1 registered func, got %d", len(m.fs))
	}

	m.fs[0](context.Background())
	if !called {
		t.Error("registered function should be callable")
	}
}

func TestGrace_RegisterMultiple(t *testing.T) {
	m := New()
	var count int32

	m.Register(func(ctx context.Context) { _ = ctx; atomic.AddInt32(&count, 1) })
	m.Register(func(ctx context.Context) { _ = ctx; atomic.AddInt32(&count, 1) })
	m.Register(func(ctx context.Context) { _ = ctx; atomic.AddInt32(&count, 1) })

	if len(m.fs) != 3 {
		t.Errorf("expected 3, got %d", len(m.fs))
	}
}

func TestGrace_SetContextPassedToHooks(t *testing.T) {
	m := New()
	type key struct{}
	k := key{}
	base := context.WithValue(context.Background(), k, "v")
	m.SetContext(base)
	var got string
	m.Register(func(ctx context.Context) {
		if v, ok := ctx.Value(k).(string); ok {
			got = v
		}
	})

	done := make(chan struct{})
	go func() {
		m.Wait()
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	m.Stop()
	select {
	case <-done:
		if got != "v" {
			t.Fatalf("hook ctx: want v, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestGrace_PanicInHookContinues(t *testing.T) {
	m := New()
	var order []int
	m.Register(func(ctx context.Context) {
		_ = ctx
		order = append(order, 1)
		panic("first")
	})
	m.Register(func(ctx context.Context) {
		_ = ctx
		order = append(order, 2)
	})

	done := make(chan struct{})
	go func() {
		m.Wait()
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	m.Stop()
	select {
	case <-done:
		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Fatalf("expected [1,2] after recover, got %v", order)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestGrace_StopTriggersClose(t *testing.T) {
	m := New()
	var called int32

	m.Register(func(ctx context.Context) {
		_ = ctx
		atomic.StoreInt32(&called, 1)
	})

	done := make(chan struct{})
	go func() {
		m.Wait()
		close(done)
	}()

	// 给 Close goroutine 一点启动时间
	time.Sleep(50 * time.Millisecond)

	// 发送关闭信号
	m.Stop()

	select {
	case <-done:
		if atomic.LoadInt32(&called) != 1 {
			t.Error("registered close function should have been called")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not complete after SendClose")
	}
}

func TestGrace_Stop_NonBlockingWhenBufferFull(t *testing.T) {
	m := New()

	// Fill the signal buffer.
	m.Stop()

	done := make(chan struct{})
	go func() {
		m.Stop() // must not block even if nobody is consuming g.ch
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop blocked when channel buffer was full")
	}
}

func TestGrace_ExecutionOrder(t *testing.T) {
	m := New()
	var order []int

	m.Register(func(ctx context.Context) { _ = ctx; order = append(order, 1) })
	m.Register(func(ctx context.Context) { _ = ctx; order = append(order, 2) })
	m.Register(func(ctx context.Context) { _ = ctx; order = append(order, 3) })

	done := make(chan struct{})
	go func() {
		m.Wait()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	m.Stop()

	select {
	case <-done:
		if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("expected [1,2,3], got %v", order)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestCustomSignal_String(t *testing.T) {
	s := customSignal{}
	if s.String() != "CustomSignal" {
		t.Errorf("expected 'CustomSignal', got '%s'", s.String())
	}
}

func TestCustomSignal_Signal(t *testing.T) {
	// Signal() 是空方法，只要不 panic 即可
	s := customSignal{}
	s.Signal()
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkGrace_Register(b *testing.B) {
	m := New()
	f := func(context.Context) {}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.Register(f)
	}
}
