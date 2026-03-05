package zgrace

import (
	"os"
	"os/signal"
	"syscall"
)

type Grace struct {
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
	for _, f := range g.fs {
		f()
	}
}

// Stop 主动触发退出信号（用于测试或内部调用）
func (g *Grace) Stop() {
	g.ch <- &customSignal{}
}

// Register 注册一个关闭时执行的函数
func (g *Grace) Register(f func()) {
	g.fs = append(g.fs, f)
}

type customSignal struct {
}

func (c customSignal) String() string {
	return "CustomSignal"
}
func (c customSignal) Signal() {}
