package zlog

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

// ✅ 优化：使用 *pool.Buffer 存入 sync.Pool 避免 []byte → interface{} 的 24B 分配
// pool.GetBytesBuffer 支持 64B - 64KB 的分桶策略，覆盖绝大多数日志场景
// 超过 64KB 的日志会直接分配（极少见）

// asyncWriteSyncer 异步写入器（优化版）
type asyncWriteSyncer struct {
	ws       zapcore.WriteSyncer
	ch       chan *zpool.Buffer
	flushCh  chan chan error // Sync 请求通道
	stopChan chan struct{}
	wg       sync.WaitGroup
	err      atomic.Value
	once     sync.Once

	droppedCount atomic.Uint64
	writtenCount atomic.Uint64
	errorCount   atomic.Uint64
}

func newAsyncWriteSyncer(ws zapcore.WriteSyncer, channelSize int) *asyncWriteSyncer {
	aws := &asyncWriteSyncer{
		ws:       ws,
		ch:       make(chan *zpool.Buffer, channelSize),
		flushCh:  make(chan chan error, 1),
		stopChan: make(chan struct{}),
	}
	aws.wg.Add(1)
	go aws.run()
	return aws
}

func (a *asyncWriteSyncer) run() {
	defer a.wg.Done()

	writeBuf := func(buf *zpool.Buffer) {
		n, err := a.ws.Write(buf.B)
		if err != nil {
			a.err.Store(err)
			a.errorCount.Add(1)
		} else if n == len(buf.B) {
			a.writtenCount.Add(1)
		}
		buf.Release()
	}

	for {
		select {
		case buf := <-a.ch:
			writeBuf(buf)

		case done := <-a.flushCh:
			for len(a.ch) > 0 {
				writeBuf(<-a.ch)
			}
			done <- a.ws.Sync()

		case <-a.stopChan:
			for {
				select {
				case buf := <-a.ch:
					writeBuf(buf)
				default:
					return
				}
			}
		}
	}
}

func (a *asyncWriteSyncer) Write(p []byte) (n int, err error) {
	// 检查后台写入是否有错误（可选：根据业务决定是否要检查）
	// if lastErr := a.err.Load(); lastErr != nil {
	// 	return 0, lastErr.(error)
	// }

	// ✅ 使用全局 buffer 池（*Buffer 指针，Put 时零分配）
	buf := zpool.GetBytesBuffer(len(p))
	buf.B = append(buf.B[:0], p...) // 确保从 0 开始追加

	select {
	case a.ch <- buf:
		return len(p), nil
	default:
		// channel 满了，丢弃日志
		a.droppedCount.Add(1)
		buf.Release() // 归还到池

		// 根据业务需求选择：
		// 方案1: 返回成功（推荐，避免影响业务）
		return len(p), nil

		// 方案2: 返回错误（让调用方感知）
		// return 0, fmt.Errorf("async log buffer full")
	}
}

// Sync 实现同步刷新（等待 run goroutine 处理完所有 pending buffer 后再 Sync）
func (a *asyncWriteSyncer) Sync() error {
	done := make(chan error, 1)
	select {
	case a.flushCh <- done:
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			return fmt.Errorf("sync timeout, %d logs still pending", len(a.ch))
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("sync timeout, flush channel blocked")
	}
}

// Close 优雅关闭
func (a *asyncWriteSyncer) Close() error {
	var closeErr error
	a.once.Do(func() {
		close(a.stopChan)
		a.wg.Wait() // 等待后台协程处理完剩余日志

		// 最后 sync 一次
		if err := a.ws.Sync(); err != nil {
			closeErr = err
		}

		// 关闭底层的 writer
		if closer, ok := a.ws.(io.Closer); ok {
			if err := closer.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}

		// 返回最后记录的错误
		if lastErr := a.err.Load(); lastErr != nil && closeErr == nil {
			closeErr = lastErr.(error)
		}
	})
	return closeErr
}

// Stats 返回监控统计信息
func (a *asyncWriteSyncer) Stats() (written, dropped, errors uint64, queueLen int) {
	return a.writtenCount.Load(),
		a.droppedCount.Load(),
		a.errorCount.Load(),
		len(a.ch)
}

// GetDroppedCount 获取丢弃的日志数（用于监控告警）
func (a *asyncWriteSyncer) GetDroppedCount() uint64 {
	return a.droppedCount.Load()
}

// GetPoolStats 获取全局 buffer 池的统计信息（用于性能分析）
func (a *asyncWriteSyncer) GetPoolStats() zpool.BufferPoolStats {
	return zpool.GetStats()
}
