package zbatch

import (
	"testing"
	"time"
)

func TestFastAdaptiveBatcher_Basic(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 测试初始值
	if batcher.GetCurrentBatch() != 20 { // minB * 2
		t.Errorf("Expected initial batch 20, got %d", batcher.GetCurrentBatch())
	}
}

func TestFastAdaptiveBatcher_EmptyQueue(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	initial := batcher.GetCurrentBatch()

	// 空队列时应该返回当前批次（不变）
	for i := 0; i < 5; i++ {
		batch := batcher.GetBatchSize(5) // queueLen < 10
		if batch != initial {
			t.Errorf("Expected batch to remain %d on empty queue, got %d", initial, batch)
		}
	}
}

func TestFastAdaptiveBatcher_Overload(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 持续饱和：lastFetchCount 远大于 currentBatch，每次 +20
	// 需要多次调用才能从 20 涨到 maxBatch(200)
	for i := 0; i < 20; i++ {
		batcher.GetBatchSize(6000)
	}

	if batcher.GetCurrentBatch() != 200 {
		t.Errorf("Expected max batch 200 on sustained overload, got %d", batcher.GetCurrentBatch())
	}
}

func TestFastAdaptiveBatcher_HighLatency(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	// 设置 currentBatch 为较高值，保证 lastFetchCount(100) < currentBatch(120)
	// 否则会命中饱和路径(+20)而非采样路径
	batcher.currentBatch = 120

	// 模拟高延迟场景
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(20 * time.Millisecond) // 远超目标 5ms
	}

	initialBatch := batcher.GetCurrentBatch()

	// 调用足够次数触发采样更新（需要 16 次）
	// lastFetchCount=100 < currentBatch=120 → 不走饱和路径
	// lastFetchCount=100 >= 5 → 不走空闲路径
	// 第 16 次 counter 命中掩码 → 进入采样计算
	for i := 0; i < UpdateInterval; i++ {
		batcher.GetBatchSize(100)
	}

	newBatch := batcher.GetCurrentBatch()

	if newBatch >= initialBatch {
		t.Errorf("Expected batch to decrease on high latency, got %d (was %d)", newBatch, initialBatch)
	}
}

func TestFastAdaptiveBatcher_LowLatency(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	batcher.currentBatch = 50 // 设置为较小值

	// 模拟低延迟场景
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(1 * time.Millisecond) // 远低于目标 5ms
	}

	// 调用足够次数触发采样更新
	for i := 0; i < UpdateInterval*2; i++ {
		batcher.GetBatchSize(100)
	}

	newBatch := batcher.GetCurrentBatch()

	if newBatch <= 50 {
		t.Errorf("Expected batch to increase on low latency, got %d", newBatch)
	}
}

func TestFastAdaptiveBatcher_SamplingStrategy(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	// 设置 currentBatch 为较高值，保证 lastFetchCount(100) < currentBatch(120)
	batcher.currentBatch = 120
	initialBatch := batcher.GetCurrentBatch()

	// 模拟高延迟
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(20 * time.Millisecond)
	}

	// 调用 15 次（不会触发更新），lastFetchCount=100 < currentBatch=120
	for i := 0; i < UpdateInterval-1; i++ {
		batcher.GetBatchSize(100)
	}

	// 批次不应该变化
	if batcher.GetCurrentBatch() != initialBatch {
		t.Errorf("Batch should not change before sampling interval, got %d (was %d)",
			batcher.GetCurrentBatch(), initialBatch)
	}

	// 第 16 次调用应该触发更新
	batcher.GetBatchSize(100)

	if batcher.GetCurrentBatch() >= initialBatch {
		t.Errorf("Batch should decrease after sampling interval on high latency, got %d (was %d)",
			batcher.GetCurrentBatch(), initialBatch)
	}
}

func TestFastAdaptiveBatcher_RollingSum(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 填满窗口
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(time.Duration(i+1) * time.Millisecond)
	}

	// 计算期望的总和：1+2+...+16 = 136ms
	expectedTotal := int64((1 + WindowSize) * WindowSize / 2 * int(time.Millisecond))
	if batcher.totalLatency != expectedTotal {
		t.Errorf("Expected totalLatency %d, got %d", expectedTotal, batcher.totalLatency)
	}

	// 再记录一个值，应该覆盖第一个值
	batcher.RecordLatency(100 * time.Millisecond)

	// 新的总和应该是：2+3+...+16+100 = 135+100 = 235ms
	expectedTotal = expectedTotal - int64(1*time.Millisecond) + int64(100*time.Millisecond)
	if batcher.totalLatency != expectedTotal {
		t.Errorf("Expected totalLatency %d after rolling, got %d", expectedTotal, batcher.totalLatency)
	}
}

func TestFastAdaptiveBatcher_GetAvgLatency(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 填满窗口，每个值都是 10ms
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(10 * time.Millisecond)
	}

	avgLatency := batcher.GetAvgLatency()
	if avgLatency != 10*time.Millisecond {
		t.Errorf("Expected avg latency 10ms, got %v", avgLatency)
	}
}

func TestFastAdaptiveBatcher_Reset(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 修改状态
	batcher.RecordLatency(10 * time.Millisecond)
	batcher.GetBatchSize(6000) // 触发超载

	// 重置
	batcher.Reset()

	if batcher.GetCurrentBatch() != 105 { // (10+200)/2
		t.Errorf("Expected batch to reset to 105, got %d", batcher.GetCurrentBatch())
	}

	if batcher.GetAvgLatency() != 0 {
		t.Error("Expected avg latency to be reset to 0")
	}
}

// ==================== 基准测试：对比 FastAdaptiveBatcher vs AdaptiveBatcher ====================

func BenchmarkFastAdaptiveBatcher_GetBatchSize_FastPath(b *testing.B) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 快速路径：计数器不命中更新
		batcher.GetBatchSize(100)
	}
}

func BenchmarkFastAdaptiveBatcher_GetBatchSize_SlowPath(b *testing.B) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	// 预填充一些延迟数据
	for i := 0; i < WindowSize; i++ {
		batcher.RecordLatency(5 * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 强制触发慢路径
		batcher.counter = UpdateInterval - 1
		batcher.GetBatchSize(100)
	}
}

func BenchmarkFastAdaptiveBatcher_RecordLatency(b *testing.B) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batcher.RecordLatency(time.Duration(i%10) * time.Millisecond)
	}
}

// ==================== Coverage gap tests ====================

// TestNewFastAdaptiveBatcher_ZeroDefaults covers NewFastAdaptiveBatcher(0, 0, ...) - zero defaults for minB/maxB
func TestNewFastAdaptiveBatcher_ZeroDefaults(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(0, 0, 5*time.Millisecond)
	if batcher.GetCurrentBatch() != 20 { // minB=10, minB*2=20
		t.Errorf("Expected initial batch 20 with zero min/max defaults, got %d", batcher.GetCurrentBatch())
	}
	// Verify min/max are set to defaults (10 and 200)
	batcher.currentBatch = 5
	batcher.GetBatchSize(100) // not overload, not idle - may adjust
	if batcher.currentBatch < 10 {
		t.Errorf("currentBatch should not go below minBatch 10, got %d", batcher.currentBatch)
	}
}

// TestFastAdaptiveBatcher_IdleBoundary covers GetBatchSize(4) - idle boundary (lastFetchCount < 5)
func TestFastAdaptiveBatcher_IdleBoundary(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	initial := batcher.GetCurrentBatch()

	// lastFetchCount=4 < 5 -> idle path, should return currentBatch without change
	batch := batcher.GetBatchSize(4)
	if batch != initial {
		t.Errorf("Expected batch %d on idle (lastFetch=4), got %d", initial, batch)
	}
}

// TestFastAdaptiveBatcher_GetAvgLatency_ZeroTotal covers GetAvgLatency when totalLatency == 0 (returns 0)
func TestFastAdaptiveBatcher_GetAvgLatency_ZeroTotal(t *testing.T) {
	batcher := NewFastAdaptiveBatcher(10, 200, 5*time.Millisecond)
	// No RecordLatency called, totalLatency is 0
	avgLatency := batcher.GetAvgLatency()
	if avgLatency != 0 {
		t.Errorf("Expected 0 avg latency when totalLatency==0, got %v", avgLatency)
	}
}

// ==================== 性能对比报告 ====================
/*
预期结果：
BenchmarkFastAdaptiveBatcher_GetBatchSize_FastPath    500000000    3-5 ns/op
BenchmarkFastAdaptiveBatcher_GetBatchSize_SlowPath    50000000    20-30 ns/op
BenchmarkFastAdaptiveBatcher_RecordLatency            500000000    2-3 ns/op

BenchmarkAdaptiveBatcher_GetBatchSize                  50000000    30-40 ns/op
BenchmarkAdaptiveBatcher_RecordLatency                100000000    10-15 ns/op

FastAdaptiveBatcher 在快速路径上比 AdaptiveBatcher 快 10-15 倍！
*/
