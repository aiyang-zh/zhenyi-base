package zbatch

// import (
// 	"fmt"
// 	"time"
// )

// 这个文件展示了如何在 Channel 和 Actor 中使用 AdaptiveBatcher

// ==================== Channel 中的使用示例 ====================

/*
// 在 BaseChannel 结构体中添加字段
type BaseChannel struct {
	// ... 其他字段
	batcher *batch.AdaptiveBatcher
}

// 在 NewBaseChannel 中初始化
func NewBaseChannel(...) *BaseChannel {
	c := &BaseChannel{
		// ... 其他初始化
		batcher: batch.NewAdaptiveBatcher(batch.Config{
			TargetP99:         5 * time.Millisecond,  // 目标延迟
			MinBatch:          20,
			MaxBatch:          200,
			InitialBatch:      128,
			WindowSize:        20,
			EmptyThreshold:    50,
			OverloadThreshold: 10000,
		}),
	}
	return c
}

// 在 runSend 方法中使用
func (c *BaseChannel) runSend(ctx context.Context) {
	defer c.sendDone.Done()

	// 使用自适应批量大小
	var readBatch []*model.Message

	for {
		// ✅ 获取当前推荐的批量大小
		queueLen := atomic.LoadInt64(&c.msgCount)
		batchSize := c.batcher.GetBatchSize(queueLen)

		// 确保 readBatch 容量足够
		if cap(readBatch) < batchSize {
			readBatch = make([]*model.Message, batchSize)
		} else {
			readBatch = readBatch[:batchSize]
		}

		// ✅ 记录开始时间
		startTime := time.Now()

		// 批量取消息
		n := c.mailBoxQueue.DequeueBatch(readBatch)
		if n == 0 {
			// 队列空，等待
			select {
			case <-ctx.Done():
				return
			case <-c.writeSignal:
				continue
			}
		}

		// 处理消息
		atomic.AddInt64(&c.msgCount, int64(-n))
		c.SendBatchMsg(readBatch[:n])

		// ✅ 记录本次批处理的延迟
		elapsed := time.Since(startTime)
		c.batcher.RecordLatency(elapsed)
	}
}
*/

// ==================== Actor 中的使用示例 ====================

/*
// 在 Actor 结构体中添加字段
type Actor struct {
	// ... 其他字段
	batcher *batch.AdaptiveBatcher
}

// 在 NewActor 中初始化
func NewActor(config ActorConfig) *Actor {
	a := &Actor{
		// ... 其他初始化
		batcher: batch.NewAdaptiveBatcher(batch.Config{
			TargetP99:         10 * time.Millisecond, // Actor 延迟目标可以稍高
			MinBatch:          10,
			MaxBatch:          200,
			InitialBatch:      50,
			WindowSize:        30,
			EmptyThreshold:    20,
			OverloadThreshold: 5000,
		}),
	}
	return a
}

// 在 Run 方法中使用（顺序模式）
func (a *Actor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// ✅ 获取当前推荐的批量大小
		queueLen := int64(a.mailBoxQueue.Len())
		batchSize := a.batcher.GetBatchSize(queueLen)

		// ✅ 批量处理消息
		startTime := time.Now()
		processedCount := a.processBatch(batchSize)

		if processedCount > 0 {
			// ✅ 记录延迟
			elapsed := time.Since(startTime)
			a.batcher.RecordLatency(elapsed)
		}
	}
}

// 批量处理消息
func (a *Actor) processBatch(maxCount int) int {
	count := 0
	for i := 0; i < maxCount; i++ {
		msg, ok := a.mailBoxQueue.TryDequeue()
		if !ok {
			break
		}
		a.handleMessage(msg)
		count++
	}
	return count
}
*/

// ==================== 监控示例 ====================

// ExampleMonitoring 展示如何监控 AdaptiveBatcher
// func ExampleMonitoring() {
// 	batcher := NewAdaptiveBatcher(DefaultConfig())

// 	// 定期打印统计信息
// 	go func() {
// 		ticker := time.NewTicker(10 * time.Second)
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			stats := batcher.GetStats()
// 			fmt.Printf("📊 [Adaptive Batcher] "+
// 				"Batch=%d, P99=%v, Avg=%v, Overload=%d\n",
// 				stats.CurrentBatch,
// 				stats.P99Latency,
// 				stats.AvgLatency,
// 				stats.ConsecutiveOverload,
// 			)
// 		}
// 	}()
// }

// // ==================== 高级用法：不同场景的配置 ====================

// // ConfigForChannel 适用于 Channel 的配置（网络 I/O 场景）
// func ConfigForChannel() Config {
// 	return Config{
// 		TargetP99:         5 * time.Millisecond,  // 网络场景要求低延迟
// 		MinBatch:          20,                     // 最小批次稍大
// 		MaxBatch:          200,                    // 最大批次
// 		InitialBatch:      128,                    // 初始较大批次
// 		WindowSize:        20,                     // 较小窗口，快速响应
// 		EmptyThreshold:    50,
// 		OverloadThreshold: 10000,
// 	}
// }

// // ConfigForActor 适用于 Actor 的配置（业务逻辑场景）
// func ConfigForActor() Config {
// 	return Config{
// 		TargetP99:         10 * time.Millisecond, // 业务逻辑可以容忍稍高延迟
// 		MinBatch:          10,                     // 最小批次较小
// 		MaxBatch:          200,
// 		InitialBatch:      50,                     // 初始中等批次
// 		WindowSize:        30,                     // 较大窗口，更平滑
// 		EmptyThreshold:    20,
// 		OverloadThreshold: 5000,
// 	}
// }

// // ConfigForHighThroughput 高吞吐量场景配置
// func ConfigForHighThroughput() Config {
// 	return Config{
// 		TargetP99:         20 * time.Millisecond, // 容忍更高延迟
// 		MinBatch:          50,                     // 更大的最小批次
// 		MaxBatch:          500,                    // 更大的最大批次
// 		InitialBatch:      200,                    // 更大的初始批次
// 		WindowSize:        50,                     // 更大窗口
// 		EmptyThreshold:    100,
// 		OverloadThreshold: 50000,
// 	}
// }

// // ConfigForLowLatency 低延迟场景配置
// func ConfigForLowLatency() Config {
// 	return Config{
// 		TargetP99:         2 * time.Millisecond,  // 极低延迟要求
// 		MinBatch:          5,                      // 极小最小批次
// 		MaxBatch:          50,                     // 较小最大批次
// 		InitialBatch:      20,                     // 较小初始批次
// 		WindowSize:        10,                     // 极快响应
// 		EmptyThreshold:    10,
// 		OverloadThreshold: 1000,
// 	}
// }
