package zgrace

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Grace struct {
	mu sync.RWMutex
	ch chan os.Signal
	fs []func()
}

func New() *Grace {
	return &Grace{
		ch: make(chan os.Signal, 1), // 建议带缓冲，避免信号丢失
		fs: make([]func(), 0),
	}
}

// Wait 阻塞直到收到退出信号，然后执行所有注册的关闭函数
func (g *Grace) Wait() {
	signal.Notify(g.ch, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	<-g.ch
	g.mu.RLock()
	runFs := make([]func(), len(g.fs))
	copy(runFs, g.fs)
	g.mu.RUnlock()
	for _, f := range runFs {
		f()
	}
}

// Stop 主动触发退出信号（用于测试或内部调用）
func (g *Grace) Stop() {
	g.ch <- &customSignal{}
}

// Register 注册一个关闭时执行的函数
func (g *Grace) Register(f func()) {
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
