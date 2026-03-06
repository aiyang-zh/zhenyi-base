package zqueue

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDequeueBatch_Empty 测试空队列
func TestDequeueBatch_Empty(t *testing.T) {
	q := GetDefaultQueue[int](10)
	buff := make([]int, 0, 10)
	result, _ := q.DequeueBatch(buff)

	if len(result) != 0 {
		t.Errorf("期望空队列返回 nil，实际得到 %v", result)
	}
}

// TestDequeueBatch_LessThanLimit 测试元素少于限制
func TestDequeueBatch_LessThanLimit(t *testing.T) {
	q := GetDefaultQueue[int](10)

	// 添加 5 个元素
	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 10)
	// 请求 10 个，应该只返回 5 个
	result, _ := q.DequeueBatch(buff)

	if len(result) != 5 {
		t.Errorf("期望返回 5 个元素，实际返回 %d 个", len(result))
	}

	// 验证元素顺序
	for i := 0; i < 5; i++ {
		if result[i] != i {
			t.Errorf("期望 result[%d] = %d，实际 = %d", i, i, result[i])
		}
	}

	// 验证队列为空
	if q.Count() != 0 {
		t.Errorf("期望队列为空，实际还有 %d 个元素", q.Count())
	}
}

// TestDequeueBatch_ExactLimit 测试元素等于限制
func TestDequeueBatch_ExactLimit(t *testing.T) {
	q := GetDefaultQueue[int](10)

	// 添加 10 个元素
	for i := 0; i < 10; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 10)
	// 请求 10 个，应该返回 10 个
	result, _ := q.DequeueBatch(buff)

	if len(result) != 10 {
		t.Errorf("期望返回 10 个元素，实际返回 %d 个", len(result))
	}

	// 验证元素顺序
	for i := 0; i < 10; i++ {
		if result[i] != i {
			t.Errorf("期望 result[%d] = %d，实际 = %d", i, i, result[i])
		}
	}

	// 验证队列为空
	if q.Count() != 0 {
		t.Errorf("期望队列为空，实际还有 %d 个元素", q.Count())
	}
}

// TestDequeueBatch_MoreThanLimit 测试元素多于限制
func TestDequeueBatch_MoreThanLimit(t *testing.T) {
	q := GetDefaultQueue[int](20)

	// 添加 20 个元素
	for i := 0; i < 20; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 10)
	// 请求 10 个
	result, _ := q.DequeueBatch(buff)

	if len(result) != 10 {
		t.Errorf("期望返回 10 个元素，实际返回 %d 个", len(result))
	}

	// 验证返回的是前 10 个元素
	for i := 0; i < 10; i++ {
		if result[i] != i {
			t.Errorf("期望 result[%d] = %d，实际 = %d", i, i, result[i])
		}
	}

	// 验证队列还剩 10 个
	if q.Count() != 10 {
		t.Errorf("期望队列还有 10 个元素，实际有 %d 个", q.Count())
	}
	buff = make([]int, 0, 10)
	// 再取出剩余的
	result2, _ := q.DequeueBatch(buff)
	if len(result2) != 10 {
		t.Errorf("第二次期望返回 10 个元素，实际返回 %d 个", len(result2))
	}

	// 验证是后 10 个元素
	for i := 0; i < 10; i++ {
		if result2[i] != i+10 {
			t.Errorf("期望 result2[%d] = %d，实际 = %d", i, i+10, result2[i])
		}
	}
}

// TestDequeueBatch_MultipleOperations 测试多次批量操作
func TestDequeueBatch_MultipleOperations(t *testing.T) {
	q := GetDefaultQueue[int](100)

	// 第一批：添加 50，取出 30
	for i := 0; i < 50; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 30)
	batch1, _ := q.DequeueBatch(buff)
	if len(batch1) != 30 {
		t.Errorf("第一批期望 30 个，实际 %d 个", len(batch1))
	}

	// 第二批：再添加 50，取出 40
	for i := 50; i < 100; i++ {
		q.Enqueue(i)
	}
	buff = make([]int, 0, 40)
	batch2, _ := q.DequeueBatch(buff)
	if len(batch2) != 40 {
		t.Errorf("第二批期望 40 个，实际 %d 个", len(batch2))
	}

	// 验证顺序：batch2 应该从 30 开始（因为前 30 个 0-29 被取走了）
	if batch2[0] != 30 {
		t.Errorf("期望 batch2[0] = 30，实际 = %d", batch2[0])
	}

	// 验证队列还有 30 个
	if q.Count() != 30 {
		t.Errorf("期望队列还有 30 个元素，实际 %d 个", q.Count())
	}
}

// TestDequeueBatch_WithResize 测试扩容场景
func TestDequeueBatch_WithResize(t *testing.T) {
	q := GetDefaultQueue[int](10) // 小容量触发扩容

	// 添加大量元素触发扩容
	for i := 0; i < 1000; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 500)
	// 批量取出
	result, _ := q.DequeueBatch(buff)

	if len(result) != 500 {
		t.Errorf("期望返回 500 个元素，实际返回 %d 个", len(result))
	}

	// 验证顺序
	for i := 0; i < 500; i++ {
		if result[i] != i {
			t.Errorf("期望 result[%d] = %d，实际 = %d", i, i, result[i])
		}
	}

	// 验证队列还有 500 个
	if q.Count() != 500 {
		t.Errorf("期望队列还有 500 个元素，实际 %d 个", q.Count())
	}
}

// TestDequeueBatch_ZeroLimit 测试 limit 为 0
func TestDequeueBatch_ZeroLimit(t *testing.T) {
	q := GetDefaultQueue[int](10)

	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 0)
	result, _ := q.DequeueBatch(buff)

	if result != nil && len(result) != 0 {
		t.Errorf("期望返回空切片或 nil，实际返回 %v", result)
	}

	// 验证队列没有变化
	if q.Count() != 5 {
		t.Errorf("期望队列还有 5 个元素，实际 %d 个", q.Count())
	}
}

func TestQueue_Count(t *testing.T) {
	fmt.Println(nextPowerOfTwo(10))
	fmt.Println(nextPowerOfTwo(5000))
}

// 测试锁竞争情况
func TestQueue_LockContention(t *testing.T) {
	q := GetDefaultQueue[int](10000)

	// 极端并发：20个协程同时写
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50000; j++ {
				q.Enqueue(id*1000000 + j)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := 20 * 50000
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	t.Logf("20个协程并发写入:")
	t.Logf("  总操作数: %d", totalOps)
	t.Logf("  总耗时: %v", elapsed)
	t.Logf("  吞吐量: %.2f M ops/秒", opsPerSec/1000000)
	t.Logf("  平均延迟: %.2f ns/op", float64(elapsed.Nanoseconds())/float64(totalOps))
}

// 测试当前实现的 Count() 性能
func BenchmarkQueue_CountWithLock(b *testing.B) {
	q := GetDefaultQueue[int](1000)

	// 填充一些数据
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = q.Count() // 每次都加锁
	}
}

// 使用 DequeueBatch 的处理模式
func BenchmarkQueue_DequeueBatch(b *testing.B) {
	q := GetDefaultQueue[int](1000)
	buff := make([]int, 0, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 填充 100 条消息
		for j := 0; j < 100; j++ {
			q.Enqueue(j)
		}

		// 🔴 使用 DequeueBatch 一次性取出
		_, _ = q.DequeueBatch(buff)
	}
}

// 大批量测试 (1000条消息)
func BenchmarkQueue_DequeueBatch_1000(b *testing.B) {
	q := GetDefaultQueue[int](10000)
	buff := make([]int, 0, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 填充 1000 条消息
		for j := 0; j < 1000; j++ {
			q.Enqueue(j)
		}

		// 使用 DequeueBatch 批量取出
		_, _ = q.DequeueBatch(buff)
	}
}

// 测试高并发写入性能
func BenchmarkQueue_ConcurrentEnqueue(b *testing.B) {
	q := GetDefaultQueue[int](10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			q.Enqueue(i)
			i++
		}
	})
}

// 测试真实场景：多个生产者 + 批量消费
func BenchmarkQueue_RealisticWorkload(b *testing.B) {
	q := GetDefaultQueue[int](100000)

	b.ResetTimer()

	// 消费者：批量处理
	var consumed int64
	consumerDone := make(chan bool)
	buff := make([]int, 0, 500)
	go func() {
		for atomic.LoadInt64(&consumed) < int64(b.N) {
			batch, _ := q.DequeueBatch(buff)
			atomic.AddInt64(&consumed, int64(len(batch)))
			if len(batch) == 0 {
				time.Sleep(10 * time.Microsecond) // 队列空时短暂休眠
			}
		}
		consumerDone <- true
	}()

	// 多个生产者并发写入
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			q.Enqueue(i)
			i++
		}
	})

	<-consumerDone
}

// 测试在不同并发度下的性能
func BenchmarkQueue_ConcurrencyLevels(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16, 20}

	for _, level := range concurrencyLevels {
		b.Run(string(rune('0'+level)), func(b *testing.B) {
			q := GetDefaultQueue[int](10000)

			b.SetParallelism(level)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					q.Enqueue(i)
					i++
				}
			})
		})
	}
}
