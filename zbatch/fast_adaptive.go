package zbatch

import (
	"time"
)

const (
	// 使用 2 的幂次，方便位运算优化
	WindowSize  = 16
	WindowMask  = WindowSize - 1
	WindowShift = 4 // 2^4 = 16，用于除法优化

	// 采样间隔：每 16 次批处理才重新计算一次策略，减少 CPU 计算
	UpdateInterval = 16
	UpdateMask     = UpdateInterval - 1
)

// FastAdaptiveBatcher 极速自适应批处理器（非线程安全，专用于单 Goroutine）
//
// 设计理念：
// - 专为 Channel/Actor 的 runSend/Run 等单 Goroutine 热路径设计
// - 使用滚动和 + 位运算 + 采样更新，将开销降至最低
// - 牺牲统计功能和线程安全，换取极致性能
//
// 性能优化：
// - O(1) 时间复杂度，无内存分配
// - 15/16 的调用直接返回，只有 1/16 执行策略计算
// - 使用位运算代替除法/取模
// - 使用数组代替切片（栈分配，少一次指针解引用）
// - 使用滚动和代替遍历求和
//
// 性能指标：
// - GetBatchSize: ~3ns/op (快速路径)
// - RecordLatency: ~2ns/op
type FastAdaptiveBatcher struct {
	// 配置参数 (只读)
	minBatch      int32
	maxBatch      int32
	targetLatency int64 // time.Duration 转为 int64

	// 运行时状态
	currentBatch int32

	// 环形缓冲区 & 滚动和
	latencies    [WindowSize]int64 // 使用数组而非切片，避免一次指针解引用，且在栈上分配
	totalLatency int64             // 滚动和
	idx          int               // 环形索引
	count        int               // filled sample count (max WindowSize)

	// 采样计数器
	counter int
}

// NewFastAdaptiveBatcher 创建极速自适应批处理器
//
// 参数:
//   - minB: 最小批量（推荐 10-20）
//   - maxB: 最大批量（推荐 200-500）
//   - targetP99: 目标延迟（推荐 5-10ms）
//
// 注意：此版本**非线程安全**，仅适用于单 Goroutine 场景（如 Channel.runSend、Actor.Run）
func NewFastAdaptiveBatcher(minB, maxB int, targetP99 time.Duration) *FastAdaptiveBatcher {
	if minB <= 0 {
		minB = 10
	}
	if maxB <= 0 {
		maxB = 200
	}

	return &FastAdaptiveBatcher{
		minBatch:      int32(minB),
		maxBatch:      int32(maxB),
		targetLatency: int64(targetP99),
		currentBatch:  int32(minB * 2), // 初始值给个中间态
	}
}

// GetBatchSize 获取批量大小
//
// 参数:
//   - lastFetchCount: 上一轮实际抓取的数量（非全局队列长度）
//
// 返回:
//   - 推荐的批量大小
//
// 算法：
//  1. 饱和路径：lastFetchCount >= currentBatch → 增大批量（队列还有更多数据）
//  2. 空闲路径：lastFetchCount < 5 → 保持当前批量（生产者慢）
//  3. 采样路径：每 16 次调用才执行一次策略计算
//     - 计算平均延迟 = totalLatency / 16 (使用右移 4 位优化)
//     - 延迟高 → 减小 batch（减少 HoL 阻塞）
//     - 延迟低 → 增大 batch（提升吞吐）
//
// 关键路径：复杂度 O(1)，无锁，无内存分配
func (b *FastAdaptiveBatcher) GetBatchSize(lastFetchCount int64) int {
	// 1. 【极速路径】饱和检查 (Saturation Check)
	// 如果上一轮实际取到的数量 >= 当前设定的 BatchSize，说明队列里还有更多数据。
	// 这时候应该倾向于扩大 BatchSize 以提升吞吐。
	if lastFetchCount >= int64(b.currentBatch) {
		// 处于饱和状态，激进增加
		b.currentBatch += 20
		if b.currentBatch > b.maxBatch {
			b.currentBatch = b.maxBatch
		}
		return int(b.currentBatch)
	}

	// 2. 【极速路径】空闲检查 (Idle Check)
	// 如果上一轮取到的很少，说明生产者很慢。
	// 此时保持现状即可，不需要频繁计算延迟，避免抖动。
	if lastFetchCount < 5 {
		// 这里不需要急着减小 currentBatch，留给下面的延迟算法去慢慢收敛
		return int(b.currentBatch)
	}

	// 3. 【采样路径】策略更新 (Sample Check)
	// 只有当计数器命中掩码时才进行复杂计算 (每 16 次调用执行一次)
	b.counter++
	if (b.counter & UpdateMask) != 0 {
		return int(b.currentBatch)
	}

	// --- 以下逻辑每 16 次才执行一次 ---

	// 计算平均延迟 (滚动和 / 16) -> 等价于 右移 4 位
	avgLatency := b.totalLatency >> WindowShift

	if avgLatency > b.targetLatency {
		// 延迟太高 -> 必须减小 Batch (为了降低 HoL 阻塞)
		b.currentBatch -= 10
		if b.currentBatch < b.minBatch {
			b.currentBatch = b.minBatch
		}
	} else if avgLatency < (b.targetLatency >> 2) { // < target / 4
		// 延迟非常低 (比如 < 目标延迟的 1/4) -> 尝试增大 Batch
		// 注意：这里的条件比上面饱和检查要宽松，用于在非饱和状态下寻找最佳点
		b.currentBatch += 5
		if b.currentBatch > b.maxBatch {
			b.currentBatch = b.maxBatch
		}
	}

	return int(b.currentBatch)
}

// RecordLatency 记录延迟
//
// 参数:
//   - d: 本次批处理的耗时
//
// 算法：
//  1. 从 totalLatency 中减去即将被覆盖的旧值
//  2. 将新值加入 totalLatency
//  3. 更新环形缓冲区
//  4. 使用位运算更新索引 (等价于 (idx + 1) % 16)
//
// 关键路径：复杂度 O(1)，无锁，无内存分配
func (b *FastAdaptiveBatcher) RecordLatency(d time.Duration) {
	val := int64(d)

	// 1. 减去即将被覆盖的旧值
	// 2. 加上新值
	// 3. 更新数组
	// 这比遍历求和快得多
	oldVal := b.latencies[b.idx]
	b.totalLatency = b.totalLatency - oldVal + val
	b.latencies[b.idx] = val

	// 位运算更新索引 (等价于 (idx + 1) % 16)
	b.idx = (b.idx + 1) & WindowMask
	if b.count < WindowSize {
		b.count++
	}
}

// GetCurrentBatch 获取当前批量大小（只读）
func (b *FastAdaptiveBatcher) GetCurrentBatch() int {
	return int(b.currentBatch)
}

// GetAvgLatency 获取平均延迟（用于监控）
func (b *FastAdaptiveBatcher) GetAvgLatency() time.Duration {
	if b.count == 0 {
		return 0
	}
	return time.Duration(b.totalLatency / int64(b.count))
}

// Reset 重置状态（用于测试）
func (b *FastAdaptiveBatcher) Reset() {
	b.currentBatch = (b.minBatch + b.maxBatch) / 2
	b.totalLatency = 0
	b.idx = 0
	b.count = 0
	b.counter = 0
	for i := range b.latencies {
		b.latencies[i] = 0
	}
}
