package zbatch

import (
	"time"
)

// AdaptiveBatcher 自适应批量处理器（非线程安全，适用于单 Goroutine）。
//
// 设计理念：
// - 适用于 Channel/Actor 的单 Goroutine 场景（如 runSend、Run）
// - 去掉锁和 atomic 操作，降低开销
// - 使用滚动和优化，避免每次循环求和
// - 使用简单的平均延迟代替 P99 排序，性能更好
// - 动态步长调整，平滑过渡
//
// 性能指标：
// - GetBatchSize: ~5-10ns/op
// - RecordLatency: ~2-3ns/op (O(1))
// - calculateAvgLatency: ~1-2ns/op (O(1))
type AdaptiveBatcher struct {
	config Config

	currentBatch int

	// 环形缓冲区 & 滚动和
	latencies    []time.Duration
	totalLatency time.Duration // ✅ 滚动和，避免每次循环求和
	idx          int
	count        int // 记录缓冲区填了多少数据
}

// Config 为 AdaptiveBatcher 的配置项。
type Config struct {
	// 延迟目标
	TargetP99 time.Duration // 目标延迟（如 5ms）

	// 批量大小范围
	MinBatch int // 最小批量（默认 10）
	MaxBatch int // 最大批量（默认 200）

	// 初始批量
	InitialBatch int // 初始批量大小（默认 50）

	// 统计窗口
	WindowSize int // 延迟统计窗口大小（默认 20）

	// 队列阈值（用于快速判断）
	EmptyThreshold    int64 // 空闲阈值（默认 50）
	OverloadThreshold int64 // 严重超载阈值（默认 5000）
}

// DefaultConfig 返回一份适用于通用网络场景的默认配置。
func DefaultConfig() Config {
	return Config{
		TargetP99:         5 * time.Millisecond,
		MinBatch:          10,
		MaxBatch:          200,
		InitialBatch:      50,
		WindowSize:        20,
		EmptyThreshold:    50,
		OverloadThreshold: 5000,
	}
}

// NewAdaptiveBatcher 创建自适应批量处理器。
//
// 注意：此版本**非线程安全**，适用于单 Goroutine 场景（如 Channel.runSend、Actor.Run）。
func NewAdaptiveBatcher(config Config) *AdaptiveBatcher {
	// 参数校验和默认值
	if config.MinBatch <= 0 {
		config.MinBatch = 10
	}
	if config.MaxBatch <= 0 {
		config.MaxBatch = 200
	}
	if config.InitialBatch <= 0 {
		config.InitialBatch = 50
	}
	if config.WindowSize <= 0 {
		config.WindowSize = 20
	}
	if config.TargetP99 <= 0 {
		config.TargetP99 = 5 * time.Millisecond
	}
	if config.EmptyThreshold <= 0 {
		config.EmptyThreshold = 50
	}
	if config.OverloadThreshold <= 0 {
		config.OverloadThreshold = 5000
	}

	return &AdaptiveBatcher{
		config:       config,
		currentBatch: config.InitialBatch,
		latencies:    make([]time.Duration, config.WindowSize),
	}
}

// GetBatchSize 根据当前队列长度与历史延迟，返回推荐的批量大小。
//
// queueLen 通常为当前待处理任务数。
// 性能：O(1)，无锁，无内存分配。
func (ab *AdaptiveBatcher) GetBatchSize(queueLen int64) int {
	// 1. 优先策略：队列积压策略
	// 如果队列很长，说明生产速度 > 消费速度
	// 此时应最大化吞吐量，使用 MaxBatch 减少系统调用次数
	if queueLen > ab.config.OverloadThreshold {
		ab.currentBatch = ab.config.MaxBatch
		return ab.config.MaxBatch
	}

	// 2. 优先策略：队列空闲策略
	// 如果队列几乎没了，没必要非凑够大 Batch，快速发出去降低延迟
	if queueLen < ab.config.EmptyThreshold {
		target := ab.config.MinBatch
		// 平滑过渡：避免突然从 200 降到 10
		if ab.currentBatch > target {
			ab.currentBatch = (ab.currentBatch + target) / 2
		} else {
			ab.currentBatch = target
		}
		return ab.currentBatch
	}

	// 3. 延迟反馈策略（当队列长度适中时）
	if ab.count > 0 {
		avg := ab.calculateAvgLatency()

		if avg > ab.config.TargetP99 {
			// 延迟太高，说明单次 Batch 处理太慢（可能是加密耗时或网络阻塞）
			// 适当减小 Batch，让数据包更碎片化地流出，避免 HoL 阻塞
			step := (ab.currentBatch - ab.config.MinBatch) / 4 // 动态步长
			if step < 1 {
				step = 1
			}
			ab.currentBatch -= step
		} else if avg < ab.config.TargetP99/2 {
			// 延迟很低，说明系统很闲或者处理很快
			// 尝试增大 Batch 以提升吞吐量上限
			step := (ab.config.MaxBatch - ab.currentBatch) / 4
			if step < 1 {
				step = 1
			}
			ab.currentBatch += step
		}
	}

	// 4. 边界限制
	if ab.currentBatch < ab.config.MinBatch {
		ab.currentBatch = ab.config.MinBatch
	}
	if ab.currentBatch > ab.config.MaxBatch {
		ab.currentBatch = ab.config.MaxBatch
	}

	return ab.currentBatch
}

// RecordLatency 记录一次批处理的延迟。
//
// 算法：
//  1. 减去即将被覆盖的旧值（从 totalLatency 中）
//  2. 将新值加入 totalLatency
//  3. 更新环形缓冲区
//
// 性能：O(1)，无锁，无内存分配。
func (ab *AdaptiveBatcher) RecordLatency(d time.Duration) {
	// ✅ 滚动和优化：先减去旧值，再加上新值
	oldVal := ab.latencies[ab.idx]
	ab.totalLatency = ab.totalLatency - oldVal + d
	ab.latencies[ab.idx] = d

	ab.idx = (ab.idx + 1) % len(ab.latencies)
	if ab.count < len(ab.latencies) {
		ab.count++
	}
}

// GetCurrentBatch 获取当前批量大小（只读）。
func (ab *AdaptiveBatcher) GetCurrentBatch() int {
	return ab.currentBatch
}

// GetAvgLatency 获取最近窗口内的平均延迟（用于监控）。
func (ab *AdaptiveBatcher) GetAvgLatency() time.Duration {
	if ab.count == 0 {
		return 0
	}
	return ab.calculateAvgLatency()
}

// calculateAvgLatency 计算平均延迟，无内存分配。
//
// 性能：O(1)，直接使用滚动和，避免循环。
func (ab *AdaptiveBatcher) calculateAvgLatency() time.Duration {
	// ✅ O(1) 计算，直接使用滚动和
	return ab.totalLatency / time.Duration(ab.count)
}

// Reset 重置内部状态（主要用于测试场景）。
func (ab *AdaptiveBatcher) Reset() {
	ab.currentBatch = ab.config.InitialBatch
	ab.totalLatency = 0
	ab.idx = 0
	ab.count = 0
	for i := range ab.latencies {
		ab.latencies[i] = 0
	}
}
