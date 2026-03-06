package zqueue

import (
	"sync"
	"testing"
)

// TestFullPolicyDrop 测试队列满时丢弃策略
func TestFullPolicyDrop(t *testing.T) {
	// 创建一个容量为4的队列（实际分配 nextPowerOfTwo(4) = 4）
	q := NewQueue[int](4, 4, FullPolicyDrop)

	// 填满队列（环形缓冲区实际可用容量是 capacity-1 = 3）
	for i := 0; i < 3; i++ {
		if !q.Enqueue(i) {
			t.Fatalf("Enqueue %d 失败，队列不应该满", i)
		}
	}

	// 尝试再插入一个元素，应该被丢弃
	if q.Enqueue(999) {
		t.Error("队列满时应该丢弃新元素，返回 false")
	}

	// 验证队列中的元素仍然是最初的3个
	if q.Count() != 3 {
		t.Errorf("队列元素数量应该是 3，实际是 %d", q.Count())
	}
	buff := make([]int, 0, 3)
	// 验证元素内容
	result, _ := q.DequeueBatch(buff)
	for i := 0; i < 3; i++ {
		if result[i] != i {
			t.Errorf("期望 %d，实际 %d", i, result[i])
		}
	}

	// 队列应该为空
	if q.Count() != 0 {
		t.Errorf("队列应该为空，实际数量 %d", q.Count())
	}
}

// TestFullPolicyResize 测试队列满时自动扩容策略（默认）
func TestFullPolicyResize(t *testing.T) {
	q := GetDefaultQueue[int](4)

	// 填满初始容量
	for i := 0; i < 3; i++ {
		if !q.Enqueue(i) {
			t.Fatalf("Enqueue %d 失败", i)
		}
	}

	// 继续插入，应该触发扩容
	if !q.Enqueue(100) {
		t.Error("Enqueue 失败，应该触发扩容")
	}

	// 验证元素数量
	if q.Count() != 4 {
		t.Errorf("队列元素数量应该是 4，实际是 %d", q.Count())
	}

	// 验证可以继续插入更多元素（说明已扩容）
	for i := 0; i < 10; i++ {
		if !q.Enqueue(200 + i) {
			t.Errorf("扩容后应该能继续插入，第 %d 次失败", i)
		}
	}

	if q.Count() != 14 {
		t.Errorf("队列元素数量应该是 14，实际是 %d", q.Count())
	}
}

// TestFullPolicyResizeWithMaxSize 测试扩容策略配合最大容量限制
func TestFullPolicyResizeWithMaxSize(t *testing.T) {
	q := NewQueue[int](4, 8, FullPolicyResize)

	// 填满初始容量 (3个元素)
	for i := 0; i < 3; i++ {
		if !q.Enqueue(i) {
			t.Fatalf("Enqueue %d 失败", i)
		}
	}

	// 触发第一次扩容 (4 -> 8)
	if !q.Enqueue(100) {
		t.Error("第一次扩容失败")
	}

	// 继续填充到接近最大容量 (环形缓冲区可用容量是 7)
	for i := 0; i < 3; i++ {
		if !q.Enqueue(200 + i) {
			t.Fatalf("Enqueue %d 失败", 200+i)
		}
	}

	// 现在队列已满，且达到最大容量，应该无法再插入
	if q.Enqueue(999) {
		t.Error("达到最大容量后不应该继续扩容")
	}
}

// TestNewQueue 测试 NewQueue 构造函数
func TestNewQueue(t *testing.T) {
	// 测试正常创建
	q1 := NewQueue[int](10, 100, FullPolicyResize)
	if q1 == nil {
		t.Error("NewQueue 应该返回非空队列")
	}

	// 测试丢弃策略
	q2 := NewQueue[string](5, 5, FullPolicyDrop)
	if q2 == nil {
		t.Error("NewQueue 应该返回非空队列")
	}

	// 验证可以正常使用
	if !q2.Enqueue("test") {
		t.Error("新创建的队列应该能插入元素")
	}
}

// TestDropPolicy_MultipleTries 测试丢弃策略下多次尝试入队
func TestDropPolicy_MultipleTries(t *testing.T) {
	q := NewQueue[int](4, 4, FullPolicyDrop)

	// 填满队列
	for i := 0; i < 3; i++ {
		q.Enqueue(i)
	}

	// 多次尝试入队，都应该失败
	for i := 0; i < 5; i++ {
		if q.Enqueue(100 + i) {
			t.Errorf("第 %d 次尝试应该失败", i)
		}
	}

	// 队列数量不应该变化
	if q.Count() != 3 {
		t.Errorf("队列数量应该保持为 3，实际 %d", q.Count())
	}
}

// TestResizePolicy_ContinuousGrowth 测试扩容策略下的连续增长
func TestResizePolicy_ContinuousGrowth(t *testing.T) {
	q := GetDefaultQueue[int](2) // 初始容量很小

	// 连续插入大量元素，测试多次扩容
	total := 1000
	for i := 0; i < total; i++ {
		if !q.Enqueue(i) {
			t.Fatalf("第 %d 次入队失败", i)
		}
	}

	if q.Count() != total {
		t.Errorf("期望数量 %d，实际 %d", total, q.Count())
	}
	buff := make([]int, 0, total)
	// 验证数据完整性
	result, _ := q.DequeueBatch(buff)
	for i := 0; i < total; i++ {
		if result[i] != i {
			t.Errorf("索引 %d: 期望 %d，实际 %d", i, i, result[i])
		}
	}
}

// TestMaxSizeLimit_ExactBoundary 测试 maxSize 的精确边界
func TestMaxSizeLimit_ExactBoundary(t *testing.T) {
	maxSize := 16
	q := NewQueue[int](4, maxSize, FullPolicyResize)

	// 填充到接近 maxSize（环形队列可用空间是 capacity-1）
	// nextPowerOfTwo(16) = 16，可用空间是 15
	for i := 0; i < 15; i++ {
		if !q.Enqueue(i) {
			t.Fatalf("第 %d 次入队失败", i)
		}
	}

	// 现在应该已经到达极限
	if q.Enqueue(999) {
		t.Error("超过 maxSize 限制应该失败")
	}
}

// TestDequeueAndEnqueueCycle 测试出队后再入队的循环
func TestDequeueAndEnqueueCycle(t *testing.T) {
	q := NewQueue[int](8, 8, FullPolicyDrop)

	// 第一轮：填满
	for i := 0; i < 7; i++ {
		q.Enqueue(i)
	}
	buff := make([]int, 0, 4)
	// 出队一半
	q.DequeueBatch(buff)

	// 应该可以再入队
	for i := 0; i < 4; i++ {
		if !q.Enqueue(100 + i) {
			t.Errorf("出队后应该有空间，第 %d 次失败", i)
		}
	}

	if q.Count() != 7 {
		t.Errorf("期望数量 7，实际 %d", q.Count())
	}

	// 验证数据顺序：应该是 4,5,6,100,101,102,103
	expected := []int{4, 5, 6, 100, 101, 102, 103}
	buff = make([]int, 0, 7)
	result, _ := q.DequeueBatch(buff)
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("索引 %d: 期望 %d，实际 %d", i, exp, result[i])
		}
	}
}

// TestConcurrentEnqueueWithDrop 并发测试：丢弃策略
func TestConcurrentEnqueueWithDrop(t *testing.T) {
	q := NewQueue[int](16, 16, FullPolicyDrop)
	done := make(chan bool)
	successCount := 0
	var mu sync.Mutex

	// 启动多个 goroutine 并发写入
	goroutines := 10
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if q.Enqueue(id*1000 + j) {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
			done <- true
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	t.Logf("队列当前元素: %d, 成功入队: %d, 丢弃数量: %d",
		q.Count(), successCount, goroutines*100-successCount)

	// 验证队列元素数量合理
	if q.Count() > 15 {
		t.Errorf("队列元素数量 %d 不应超过可用容量 15", q.Count())
	}
}

// TestConcurrentEnqueueWithResize 并发测试：扩容策略
func TestConcurrentEnqueueWithResize(t *testing.T) {
	q := GetDefaultQueue[int](16)
	var wg sync.WaitGroup
	goroutines := 10
	itemsPerGoroutine := 100

	// 启动多个 goroutine 并发写入
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				if !q.Enqueue(id*1000 + j) {
					t.Errorf("扩容策略下入队不应该失败")
				}
			}
		}(i)
	}

	wg.Wait()

	expectedCount := goroutines * itemsPerGoroutine
	if q.Count() != expectedCount {
		t.Errorf("期望数量 %d，实际 %d", expectedCount, q.Count())
	}
}

// BenchmarkEnqueueDrop 基准测试：丢弃策略下的入队性能
func BenchmarkEnqueueDrop(b *testing.B) {
	q := NewQueue[int](1024, 1024, FullPolicyDrop)

	// 先填满队列
	for i := 0; i < 1023; i++ {
		q.Enqueue(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i) // 会被丢弃
	}
}

// BenchmarkEnqueueResize 基准测试：扩容策略下的入队性能
func BenchmarkEnqueueResize(b *testing.B) {
	q := GetDefaultQueue[int](1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		if i%1000 == 0 {
			// 定期清空队列，避免无限扩容
			buff := make([]int, 0, q.Count())
			q.DequeueBatch(buff)
		}
	}
}
