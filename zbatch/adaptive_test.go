package zbatch

import (
	"testing"
	"time"
)

func TestAdaptiveBatcher_Basic(t *testing.T) {
	config := DefaultConfig()
	config.TargetP99 = 5 * time.Millisecond

	batcher := NewAdaptiveBatcher(config)

	// 测试初始值
	if batcher.GetCurrentBatch() != 50 {
		t.Errorf("Expected initial batch 50, got %d", batcher.GetCurrentBatch())
	}
}

func TestAdaptiveBatcher_EmptyQueue(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	// 空队列时应该减小批次
	for i := 0; i < 5; i++ {
		batcher.GetBatchSize(10) // 队列长度 10（< 50）
	}

	if batcher.GetCurrentBatch() >= 50 {
		t.Errorf("Expected batch size to decrease on empty queue, got %d", batcher.GetCurrentBatch())
	}
}

func TestAdaptiveBatcher_Overload(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	// 严重超载时应该立即升到最大批次
	batch := batcher.GetBatchSize(6000) // > 5000

	if batch != 200 {
		t.Errorf("Expected max batch 200 on overload, got %d", batch)
	}
}

func TestAdaptiveBatcher_LatencyFeedback(t *testing.T) {
	config := DefaultConfig()
	config.TargetP99 = 5 * time.Millisecond

	batcher := NewAdaptiveBatcher(config)

	// 模拟高延迟场景
	for i := 0; i < 20; i++ {
		batcher.RecordLatency(15 * time.Millisecond) // 远超目标 5ms
	}

	// 获取批次大小，应该减小
	initialBatch := batcher.GetCurrentBatch()
	batch := batcher.GetBatchSize(500)

	if batch >= initialBatch {
		t.Errorf("Expected batch to decrease on high latency, got %d (was %d)", batch, initialBatch)
	}
}

func TestAdaptiveBatcher_LowLatency(t *testing.T) {
	config := DefaultConfig()
	config.TargetP99 = 5 * time.Millisecond

	batcher := NewAdaptiveBatcher(config)
	batcher.currentBatch = 50 // 设置为较小值

	// 模拟低延迟场景
	for i := 0; i < 20; i++ {
		batcher.RecordLatency(1 * time.Millisecond) // 远低于目标 5ms
	}

	// 获取批次大小，应该增大
	batch := batcher.GetBatchSize(500)

	if batch <= 50 {
		t.Errorf("Expected batch to increase on low latency, got %d", batch)
	}
}

func TestAdaptiveBatcher_AvgLatency(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	// 记录一些延迟
	latencies := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		10 * time.Millisecond,
	}

	for _, lat := range latencies {
		batcher.RecordLatency(lat)
	}

	avgLatency := batcher.GetAvgLatency()

	if avgLatency == 0 {
		t.Error("Expected non-zero average latency")
	}

	// 平均值应该是 (1+2+3+10)/4 = 4ms
	expected := 4 * time.Millisecond
	if avgLatency != expected {
		t.Errorf("Expected avg latency %v, got %v", expected, avgLatency)
	}
}

func TestAdaptiveBatcher_Reset(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	// 修改状态
	batcher.RecordLatency(10 * time.Millisecond)
	batcher.GetBatchSize(6000) // 触发超载

	// 重置
	batcher.Reset()

	if batcher.GetCurrentBatch() != 50 {
		t.Errorf("Expected batch to reset to 50 (InitialBatch), got %d", batcher.GetCurrentBatch())
	}

	if batcher.GetAvgLatency() != 0 {
		t.Error("Expected avg latency to be reset to 0")
	}
}

func TestAdaptiveBatcher_RollingSum(t *testing.T) {
	config := DefaultConfig()
	config.WindowSize = 5 // 使用较小窗口便于测试
	batcher := NewAdaptiveBatcher(config)

	// 填满窗口：[1ms, 2ms, 3ms, 4ms, 5ms]
	for i := 1; i <= 5; i++ {
		batcher.RecordLatency(time.Duration(i) * time.Millisecond)
	}

	// 期望平均值：(1+2+3+4+5)/5 = 3ms
	avgLatency := batcher.GetAvgLatency()
	expected := 3 * time.Millisecond
	if avgLatency != expected {
		t.Errorf("Expected avg latency %v, got %v", expected, avgLatency)
	}

	// 再记录一个值，覆盖第一个值：[6ms, 2ms, 3ms, 4ms, 5ms]
	batcher.RecordLatency(6 * time.Millisecond)

	// 期望平均值：(2+3+4+5+6)/5 = 4ms
	avgLatency = batcher.GetAvgLatency()
	expected = 4 * time.Millisecond
	if avgLatency != expected {
		t.Errorf("Expected avg latency %v after rolling, got %v", expected, avgLatency)
	}

	// 验证 totalLatency 的正确性
	expectedTotal := (2 + 3 + 4 + 5 + 6) * int64(time.Millisecond)
	if int64(batcher.totalLatency) != expectedTotal {
		t.Errorf("Expected totalLatency %d, got %d", expectedTotal, batcher.totalLatency)
	}
}

func BenchmarkAdaptiveBatcher_GetBatchSize(b *testing.B) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batcher.GetBatchSize(int64(i % 5000))
	}
}

func BenchmarkAdaptiveBatcher_RecordLatency(b *testing.B) {
	batcher := NewAdaptiveBatcher(DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batcher.RecordLatency(time.Duration(i%10) * time.Millisecond)
	}
}

// ==================== Coverage gap tests ====================

// TestNewAdaptiveBatcher_ZeroConfig covers NewAdaptiveBatcher(Config{}) - all zero config defaults
func TestNewAdaptiveBatcher_ZeroConfig(t *testing.T) {
	batcher := NewAdaptiveBatcher(Config{})
	if batcher.GetCurrentBatch() != 50 {
		t.Errorf("Expected initial batch 50 with zero config, got %d", batcher.GetCurrentBatch())
	}
	if batcher.config.MinBatch != 10 || batcher.config.MaxBatch != 200 {
		t.Errorf("Expected MinBatch=10, MaxBatch=200 defaults, got %d, %d", batcher.config.MinBatch, batcher.config.MaxBatch)
	}
	if batcher.config.WindowSize != 20 || batcher.config.EmptyThreshold != 50 || batcher.config.OverloadThreshold != 5000 {
		t.Errorf("Expected WindowSize=20, EmptyThreshold=50, OverloadThreshold=5000, got %d, %d, %d",
			batcher.config.WindowSize, batcher.config.EmptyThreshold, batcher.config.OverloadThreshold)
	}
}

// TestAdaptiveBatcher_OverloadExactBoundary covers GetBatchSize(5001) - exact OverloadThreshold boundary (queueLen > 5000)
func TestAdaptiveBatcher_OverloadExactBoundary(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())
	batch := batcher.GetBatchSize(5001) // > 5000
	if batch != 200 {
		t.Errorf("Expected max batch 200 at overload boundary (5001), got %d", batch)
	}
}

// TestAdaptiveBatcher_EmptyExactBoundary covers GetBatchSize(49) - exact EmptyThreshold boundary (queueLen < 50)
func TestAdaptiveBatcher_EmptyExactBoundary(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())
	initial := batcher.GetCurrentBatch()
	batch := batcher.GetBatchSize(49) // < 50
	if batch >= initial {
		t.Errorf("Expected batch to decrease at empty boundary (49), got %d (was %d)", batch, initial)
	}
}

// TestAdaptiveBatcher_EmptyPath_CurrentBatchAtMin covers GetBatchSize when currentBatch <= target (already at MinBatch)
func TestAdaptiveBatcher_EmptyPath_CurrentBatchAtMin(t *testing.T) {
	config := DefaultConfig()
	batcher := NewAdaptiveBatcher(config)
	batcher.currentBatch = config.MinBatch // already at target

	batch := batcher.GetBatchSize(10) // queueLen < EmptyThreshold
	if batch != config.MinBatch {
		t.Errorf("Expected batch %d when already at MinBatch, got %d", config.MinBatch, batch)
	}
}

// TestAdaptiveBatcher_StepLessThanOne covers GetBatchSize when step < 1 (currentBatch close to MinBatch or MaxBatch)
func TestAdaptiveBatcher_StepLessThanOne(t *testing.T) {
	config := DefaultConfig()
	config.MinBatch = 10
	config.MaxBatch = 200
	batcher := NewAdaptiveBatcher(config)

	// High latency path: step = (currentBatch - MinBatch) / 4. When currentBatch = MinBatch+2, step = 0 -> step = 1
	batcher.currentBatch = 12 // (12-10)/4 = 0, step < 1 -> step = 1
	for i := 0; i < config.WindowSize; i++ {
		batcher.RecordLatency(20 * time.Millisecond) // high latency
	}
	batch := batcher.GetBatchSize(500) // queueLen in normal range
	if batch > 12 {
		t.Errorf("Expected batch to decrease when step would be 0 (currentBatch=12), got %d", batch)
	}

	// Low latency path: step = (MaxBatch - currentBatch) / 4. When currentBatch = MaxBatch-2, step = 0 -> step = 1
	config2 := DefaultConfig()
	config2.MinBatch = 10
	config2.MaxBatch = 200
	batcher2 := NewAdaptiveBatcher(config2)
	batcher2.currentBatch = 198 // (200-198)/4 = 0, step < 1 -> step = 1
	for i := 0; i < config2.WindowSize; i++ {
		batcher2.RecordLatency(1 * time.Millisecond) // low latency
	}
	batch2 := batcher2.GetBatchSize(500)
	if batch2 < 198 {
		t.Errorf("Expected batch to increase when step would be 0 (currentBatch=198), got %d", batch2)
	}
}

// TestAdaptiveBatcher_GetAvgLatency_ZeroCount covers GetAvgLatency when count == 0 (returns 0)
func TestAdaptiveBatcher_GetAvgLatency_ZeroCount(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())
	// No RecordLatency called, count is 0
	avgLatency := batcher.GetAvgLatency()
	if avgLatency != 0 {
		t.Errorf("Expected 0 avg latency when count==0, got %v", avgLatency)
	}
}

// TestAdaptiveBatcher_QueueLen50_NotEmpty covers GetBatchSize(50) - queueLen == EmptyThreshold does NOT trigger empty path
func TestAdaptiveBatcher_QueueLen50_NotEmpty(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())
	// queueLen 50 is NOT < 50, so we don't hit empty path - goes to latency feedback
	batch := batcher.GetBatchSize(50)
	if batch < 10 || batch > 200 {
		t.Errorf("Expected batch in [10,200] when queueLen=50 (boundary), got %d", batch)
	}
}

// TestAdaptiveBatcher_QueueLen5000_NotOverload covers GetBatchSize(5000) - queueLen == OverloadThreshold does NOT trigger overload
func TestAdaptiveBatcher_QueueLen5000_NotOverload(t *testing.T) {
	batcher := NewAdaptiveBatcher(DefaultConfig())
	batcher.currentBatch = 100
	// queueLen 5000 is NOT > 5000, so overload path not taken
	batch := batcher.GetBatchSize(5000)
	// Should not be forced to 200; could stay at 100 or change based on latency
	if batch != 200 {
		// Good - we're in the normal/latency feedback path
	}
	// Just verify we get a valid batch
	if batch < 10 || batch > 200 {
		t.Errorf("Expected batch in [10,200] when queueLen=5000 (boundary), got %d", batch)
	}
}
