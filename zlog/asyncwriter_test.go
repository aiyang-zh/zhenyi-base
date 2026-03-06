package zlog

import (
	"bytes"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"io"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

// TestAsyncWriter_PoolUsage 测试异步写入器使用全局 buffer 池
func TestAsyncWriter_PoolUsage(t *testing.T) {
	// 记录初始统计
	initialStats := zpool.GetStats()

	// 创建 mock WriteSyncer
	buf := &bytes.Buffer{}
	mockSyncer := zapcore.AddSync(buf)

	// 创建异步写入器
	async := newAsyncWriteSyncer(mockSyncer, 100)
	defer async.Close()

	// 写入 50 条日志
	testData := []byte("test log message")
	for i := 0; i < 50; i++ {
		n, err := async.Write(testData)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Write returned %d, expected %d", n, len(testData))
		}
	}

	// 等待写入完成
	time.Sleep(200 * time.Millisecond)

	// 同步
	if err := async.Sync(); err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// 检查统计
	finalStats := zpool.GetStats()

	// 计算 Get/Put 次数的差异
	var getTimes, putTimes uint64
	for i := range finalStats.BucketStats {
		getTimes += finalStats.BucketStats[i].GetRequests - initialStats.BucketStats[i].GetRequests
		putTimes += finalStats.BucketStats[i].PutReturns - initialStats.BucketStats[i].PutReturns
	}

	t.Logf("对象池使用统计:")
	t.Logf("  Get 次数: %d", getTimes)
	t.Logf("  Put 次数: %d", putTimes)

	// 验证对象池被使用
	if getTimes == 0 {
		t.Error("异步写入器应该使用对象池，但 Get 次数为 0")
	}

	// 验证复用率（应该接近 100%）
	if getTimes > 0 {
		reuseRate := float64(putTimes) / float64(getTimes) * 100
		t.Logf("  复用率: %.2f%%", reuseRate)

		// 允许一些误差（例如，最后几个 buffer 可能还在 channel 中）
		if reuseRate < 80 {
			t.Errorf("复用率过低: %.2f%%, 期望 >= 80%%", reuseRate)
		}
	}

	// 验证写入的内容
	written := buf.String()
	expectedLen := len(testData) * 50
	if len(written) != expectedLen {
		t.Errorf("写入内容长度 %d, 期望 %d", len(written), expectedLen)
	}
}

// TestAsyncWriter_ChannelFull 测试 channel 满时的行为
func TestAsyncWriter_ChannelFull(t *testing.T) {
	// 创建一个不会写入的 WriteSyncer（阻塞）
	blockingSyncer := &blockingWriteSyncer{
		block: make(chan struct{}),
	}

	// 创建小容量的异步写入器
	async := newAsyncWriteSyncer(blockingSyncer, 2)
	defer func() {
		close(blockingSyncer.block) // 解除阻塞
		async.Close()
	}()

	// 写入超过 channel 容量的数据
	testData := []byte("test")
	successCount := 0
	for i := 0; i < 10; i++ {
		n, err := async.Write(testData)
		if err == nil && n == len(testData) {
			successCount++
		}
	}

	// 统计丢弃的日志数
	stats := async.GetPoolStats()
	dropped := async.GetDroppedCount()

	t.Logf("成功写入: %d/10", successCount)
	t.Logf("丢弃日志: %d", dropped)
	t.Logf("对象池统计: %+v", stats)

	// 所有写入都应该"成功"（返回成功），但有些会被丢弃
	if successCount != 10 {
		t.Errorf("所有 Write 调用应该返回成功，实际成功 %d/10", successCount)
	}

	// 应该有一些日志被丢弃
	if dropped == 0 {
		t.Error("期望有日志被丢弃，但 dropped count 为 0")
	}
}

// TestAsyncWriter_Stats 测试统计功能
func TestAsyncWriter_Stats(t *testing.T) {
	buf := &bytes.Buffer{}
	mockSyncer := zapcore.AddSync(buf)

	async := newAsyncWriteSyncer(mockSyncer, 100)
	defer async.Close()

	// 写入一些数据
	for i := 0; i < 10; i++ {
		async.Write([]byte("test"))
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 获取统计
	written, dropped, errors, queueLen := async.Stats()

	t.Logf("统计信息:")
	t.Logf("  已写入: %d", written)
	t.Logf("  已丢弃: %d", dropped)
	t.Logf("  错误数: %d", errors)
	t.Logf("  队列长度: %d", queueLen)

	// 验证
	if written == 0 {
		t.Error("应该有日志被写入")
	}

	if errors != 0 {
		t.Errorf("不应该有错误，实际错误数: %d", errors)
	}
}

// TestAsyncWriter_GracefulClose 测试优雅关闭
func TestAsyncWriter_GracefulClose(t *testing.T) {
	buf := &bytes.Buffer{}
	mockSyncer := zapcore.AddSync(buf)

	async := newAsyncWriteSyncer(mockSyncer, 100)

	// 写入大量数据
	for i := 0; i < 100; i++ {
		async.Write([]byte("test\n"))
	}

	// 立即关闭（不等待）
	err := async.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 验证所有数据都被写入
	lines := bytes.Count(buf.Bytes(), []byte("\n"))
	t.Logf("写入了 %d 行", lines)

	if lines != 100 {
		t.Errorf("期望写入 100 行，实际 %d 行", lines)
	}
}

// blockingWriteSyncer 用于测试的阻塞写入器
type blockingWriteSyncer struct {
	block chan struct{}
}

func (b *blockingWriteSyncer) Write(p []byte) (n int, err error) {
	<-b.block // 阻塞直到 channel 被关闭
	return len(p), nil
}

func (b *blockingWriteSyncer) Sync() error {
	return nil
}

// BenchmarkAsyncWriter_Write 异步写入性能基准测试
func BenchmarkAsyncWriter_Write(b *testing.B) {
	// 使用 discardWriteSyncer 避免 I/O 干扰
	discard := &discardWriteSyncer{}
	async := newAsyncWriteSyncer(discard, 4096)
	defer async.Close()

	data := []byte("2026-01-26 16:00:00.000 INFO test message with some fields")

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		async.Write(data)
	}

	b.StopTimer()
	async.Sync()
}

// BenchmarkAsyncWriter_WriteDifferentSizes 不同大小的日志性能
func BenchmarkAsyncWriter_WriteDifferentSizes(b *testing.B) {
	sizes := []int{
		50,   // 简单日志
		200,  // 普通日志
		500,  // 详细日志
		1024, // 大日志
		4096, // 超大日志（带 stacktrace）
	}

	for _, size := range sizes {
		b.Run("Size-"+string(rune(size)), func(b *testing.B) {
			discard := &discardWriteSyncer{}
			async := newAsyncWriteSyncer(discard, 4096)
			defer async.Close()

			data := make([]byte, size)
			for i := range data {
				data[i] = 'x'
			}

			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				async.Write(data)
			}

			b.StopTimer()
			async.Sync()
		})
	}
}

// BenchmarkAsyncWriter_Concurrent 并发写入性能
func BenchmarkAsyncWriter_Concurrent(b *testing.B) {
	discard := &discardWriteSyncer{}
	async := newAsyncWriteSyncer(discard, 4096)
	defer async.Close()

	data := []byte("2026-01-26 16:00:00.000 INFO concurrent log message")

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			async.Write(data)
		}
	})

	b.StopTimer()
	async.Sync()
}

// BenchmarkBufferedWriter_Write 对比：buffered 写入性能
func BenchmarkBufferedWriter_Write(b *testing.B) {
	discard := &discardWriteSyncer{}
	buffered := newBufferedWriteSyncer(discard, 8192)

	data := []byte("2026-01-26 16:00:00.000 INFO test message with some fields")

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buffered.Write(data)
	}

	b.StopTimer()
	buffered.Sync()
}

// BenchmarkDirectWriter_Write 对比：直接写入性能（无缓冲）
func BenchmarkDirectWriter_Write(b *testing.B) {
	discard := &discardWriteSyncer{}

	data := []byte("2026-01-26 16:00:00.000 INFO test message with some fields")

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		discard.Write(data)
	}
}

// BenchmarkAsyncWriter_WriteAndSync 写入 + 同步的性能
func BenchmarkAsyncWriter_WriteAndSync(b *testing.B) {
	discard := &discardWriteSyncer{}
	async := newAsyncWriteSyncer(discard, 4096)
	defer async.Close()

	data := []byte("2026-01-26 16:00:00.000 INFO test message")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		async.Write(data)
		if i%100 == 0 { // 每 100 条同步一次
			async.Sync()
		}
	}
}

// discardWriteSyncer 用于基准测试的丢弃写入器（无 I/O 开销）
type discardWriteSyncer struct{}

func (d *discardWriteSyncer) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (d *discardWriteSyncer) Sync() error {
	return nil
}

var _ zapcore.WriteSyncer = (*discardWriteSyncer)(nil)
var _ io.Writer = (*discardWriteSyncer)(nil)
