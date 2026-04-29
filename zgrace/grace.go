package zgrace

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Grace struct {
	mu sync.RWMutex
	ch chan os.Signal
	fs []func(context.Context)
	// ctx 为执行关闭回调时传入的上下文；未设置时在 Wait 中使用 context.Background。
	ctx        context.Context
	notifyOnce sync.Once
}

func New() *Grace {
	return &Grace{
		ch: make(chan os.Signal, 1), // 建议带缓冲，避免信号丢失
		fs: make([]func(context.Context), 0),
	}
}

// SetContext 设置在触发关闭时传给各回调的 context（如带超时的派生 context）。
// 传入 nil 表示清除，Wait 时将使用 context.Background。
func (g *Grace) SetContext(ctx context.Context) {
	g.mu.Lock()
	g.ctx = ctx
	g.mu.Unlock()
}

// EnableSignalNotify enables process signal forwarding into Grace channel once.
// EnableSignalNotify 启用进程信号到 Grace 通道的转发（幂等，只注册一次）。
func (g *Grace) EnableSignalNotify() {
	g.notifyOnce.Do(func() {
		signal.Notify(g.ch, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	})
}

// Wait 阻塞直到收到退出信号，然后按注册顺序执行关闭函数。
// 每个回调接收 SetContext 设置的上下文（若未设置则为 Background）。
// 单个回调若 panic，会被捕获，后续回调仍会继续执行。
func (g *Grace) Wait() {
	g.EnableSignalNotify()
	<-g.ch
	g.mu.RLock()
	runFs := make([]func(context.Context), len(g.fs))
	copy(runFs, g.fs)
	runCtx := g.ctx
	g.mu.RUnlock()
	if runCtx == nil {
		runCtx = context.Background()
	}
	for _, f := range runFs {
		func() {
			defer func() { _ = recover() }()
			f(runCtx)
		}()
	}
}

// Stop 主动触发退出信号（用于测试或内部调用）
func (g *Grace) Stop() {
	select {
	case g.ch <- &customSignal{}:
	default:
		// already stopped / signal pending
	}
}

// Register 注册关闭时执行的函数，参数为本次停机使用的 context。
func (g *Grace) Register(f func(context.Context)) {
	g.mu.Lock()
	g.fs = append(g.fs, f)
	g.mu.Unlock()
}

type customSignal struct {
}

func (c customSignal) String() string {
	return "CustomSignal"
}
func (c customSignal) Signal() {}
