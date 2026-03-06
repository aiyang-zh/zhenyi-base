package zqueue

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// UnboundedMPSC 单元测试
// =============================================================================

func TestUnboundedMPSC_BasicEnqueueDequeue(t *testing.T) {
	q := NewUnboundedMPSC[int]()

	// 测试空队列
	if !q.Empty() {
		t.Fatal("新队列应该为空")
	}

	val, ok := q.Dequeue()
	if ok {
		t.Fatal("空队列 Dequeue 应该返回 false")
	}
	if val != 0 {
		t.Fatal("空队列 Dequeue 应该返回零值")
	}

	// 测试单个元素
	q.Enqueue(42)
	if q.Empty() {
		t.Fatal("入队后不应该为空")
	}

	val, ok = q.Dequeue()
	if !ok || val != 42 {
		t.Fatalf("期望 (42, true)，得到 (%d, %v)", val, ok)
	}

	if !q.Empty() {
		t.Fatal("出队后应该为空")
	}
}

func TestUnboundedMPSC_FIFO(t *testing.T) {
	q := NewUnboundedMPSC[int]()
	n := 1000

	// 入队
	for i := 0; i < n; i++ {
		q.Enqueue(i)
	}

	// 验证 FIFO
	for i := 0; i < n; i++ {
		val, ok := q.Dequeue()
		if !ok {
			t.Fatalf("第 %d 次出队失败", i)
		}
		if val != i {
			t.Fatalf("第 %d 次出队：期望 %d，得到 %d", i, i, val)
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedMPSC_DequeueBatch(t *testing.T) {
	q := NewUnboundedMPSC[int]()

	// 入队 100 个元素
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}

	// 批量出队（每次 10 个）
	buffer := make([]int, 10)
	for batch := 0; batch < 10; batch++ {
		n := q.DequeueBatch(buffer)
		if n != 10 {
			t.Fatalf("批次 %d：期望出队 10 个，实际 %d", batch, n)
		}
		for i := 0; i < 10; i++ {
			expected := batch*10 + i
			if buffer[i] != expected {
				t.Fatalf("批次 %d，索引 %d：期望 %d，得到 %d", batch, i, expected, buffer[i])
			}
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedMPSC_EnqueueBatch(t *testing.T) {
	q := NewUnboundedMPSC[int]()

	// 批量入队
	batch1 := []int{1, 2, 3, 4, 5}
	q.EnqueueBatch(batch1)

	batch2 := []int{6, 7, 8}
	q.EnqueueBatch(batch2)

	// 验证顺序
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for i, exp := range expected {
		val, ok := q.Dequeue()
		if !ok || val != exp {
			t.Fatalf("索引 %d：期望 (%d, true)，得到 (%d, %v)", i, exp, val, ok)
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedMPSC_Concurrent(t *testing.T) {
	q := NewUnboundedMPSC[int]()
	producers := 10
	itemsPerProducer := 1000
	total := producers * itemsPerProducer

	// 并发生产
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		producerID := p
		go func() {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				q.Enqueue(producerID*itemsPerProducer + i)
			}
		}()
	}
	wg.Wait()

	// 单消费者
	received := make(map[int]bool)
	for i := 0; i < total; i++ {
		val, ok := q.Dequeue()
		if !ok {
			t.Fatalf("第 %d 次出队失败", i)
		}
		if received[val] {
			t.Fatalf("重复接收到值: %d", val)
		}
		received[val] = true
	}

	if len(received) != total {
		t.Fatalf("期望接收 %d 个不同的值，实际 %d", total, len(received))
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedMPSC_ConcurrentBatch(t *testing.T) {
	q := NewUnboundedMPSC[int]()
	producers := 4
	batchesPerProducer := 100
	batchSize := 10
	total := producers * batchesPerProducer * batchSize

	// 并发批量生产
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		producerID := p
		go func() {
			defer wg.Done()
			for b := 0; b < batchesPerProducer; b++ {
				batch := make([]int, batchSize)
				base := producerID*batchesPerProducer*batchSize + b*batchSize
				for i := 0; i < batchSize; i++ {
					batch[i] = base + i
				}
				q.EnqueueBatch(batch)
			}
		}()
	}
	wg.Wait()

	// 单消费者批量出队
	received := make(map[int]bool)
	buffer := make([]int, 50)
	for {
		n := q.DequeueBatch(buffer)
		if n == 0 {
			break
		}
		for i := 0; i < n; i++ {
			if received[buffer[i]] {
				t.Fatalf("重复接收到值: %d", buffer[i])
			}
			received[buffer[i]] = true
		}
	}

	if len(received) != total {
		t.Fatalf("期望接收 %d 个不同的值，实际 %d", total, len(received))
	}
}

func TestUnboundedMPSC_StressTest(t *testing.T) {

	q := NewUnboundedMPSC[int]()
	producers := 20
	duration := 2 * time.Second
	stop := make(chan struct{})

	// 统计
	var produced, consumed atomic.Int64

	// 并发生产
	var wg sync.WaitGroup
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-stop:
					return
				default:
					q.Enqueue(count)
					count++
					produced.Add(1)
				}
			}
		}()
	}

	// 单消费者
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]int, 100)
		for {
			select {
			case <-stop:
				// 排空队列
				for {
					n := q.DequeueBatch(buffer)
					if n == 0 {
						break
					}
					consumed.Add(int64(n))
				}
				return
			default:
				n := q.DequeueBatch(buffer)
				if n > 0 {
					consumed.Add(int64(n))
				} else {
					runtime.Gosched()
				}
			}
		}
	}()

	// 运行指定时间
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// 验证
	p := produced.Load()
	c := consumed.Load()
	if p != c {
		t.Fatalf("生产 %d，消费 %d，不匹配", p, c)
	}
	t.Logf("压力测试：%d 秒内生产并消费 %d 个元素", int(duration.Seconds()), p)
}

// =============================================================================
// UnboundedSPSC 单元测试
// =============================================================================

func TestUnboundedSPSC_BasicEnqueueDequeue(t *testing.T) {
	q := NewUnboundedSPSC[int]()

	// 测试空队列
	if !q.Empty() {
		t.Fatal("新队列应该为空")
	}

	val, ok := q.Dequeue()
	if ok {
		t.Fatal("空队列 Dequeue 应该返回 false")
	}
	if val != 0 {
		t.Fatal("空队列 Dequeue 应该返回零值")
	}

	// 测试单个元素
	q.Enqueue(42)
	if q.Empty() {
		t.Fatal("入队后不应该为空")
	}

	val, ok = q.Dequeue()
	if !ok || val != 42 {
		t.Fatalf("期望 (42, true)，得到 (%d, %v)", val, ok)
	}

	if !q.Empty() {
		t.Fatal("出队后应该为空")
	}
}

func TestUnboundedSPSC_FIFO(t *testing.T) {
	q := NewUnboundedSPSC[int]()
	n := 1000

	// 入队
	for i := 0; i < n; i++ {
		q.Enqueue(i)
	}

	// 验证 FIFO
	for i := 0; i < n; i++ {
		val, ok := q.Dequeue()
		if !ok {
			t.Fatalf("第 %d 次出队失败", i)
		}
		if val != i {
			t.Fatalf("第 %d 次出队：期望 %d，得到 %d", i, i, val)
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedSPSC_DequeueBatch(t *testing.T) {
	q := NewUnboundedSPSC[int]()

	// 入队 100 个元素
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}

	// 批量出队（每次 10 个）
	buffer := make([]int, 10)
	for batch := 0; batch < 10; batch++ {
		n := q.DequeueBatch(buffer)
		if n != 10 {
			t.Fatalf("批次 %d：期望出队 10 个，实际 %d", batch, n)
		}
		for i := 0; i < 10; i++ {
			expected := batch*10 + i
			if buffer[i] != expected {
				t.Fatalf("批次 %d，索引 %d：期望 %d，得到 %d", batch, i, expected, buffer[i])
			}
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedSPSC_EnqueueBatch(t *testing.T) {
	q := NewUnboundedSPSC[int]()

	// 批量入队
	batch1 := []int{1, 2, 3, 4, 5}
	q.EnqueueBatch(batch1)

	batch2 := []int{6, 7, 8}
	q.EnqueueBatch(batch2)

	// 验证顺序
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for i, exp := range expected {
		val, ok := q.Dequeue()
		if !ok || val != exp {
			t.Fatalf("索引 %d：期望 (%d, true)，得到 (%d, %v)", i, exp, val, ok)
		}
	}

	if !q.Empty() {
		t.Fatal("所有元素出队后应该为空")
	}
}

func TestUnboundedSPSC_PingPong(t *testing.T) {
	q := NewUnboundedSPSC[int]()
	iterations := 10000

	// 单生产者
	go func() {
		for i := 0; i < iterations; i++ {
			q.Enqueue(i)
		}
	}()

	// 单消费者
	for i := 0; i < iterations; i++ {
		for {
			val, ok := q.Dequeue()
			if ok {
				if val != i {
					t.Errorf("期望 %d，得到 %d", i, val)
				}
				break
			}
			runtime.Gosched()
		}
	}
}

func TestUnboundedSPSC_BatchPingPong(t *testing.T) {
	q := NewUnboundedSPSC[int]()
	batchSize := 10
	batches := 100
	total := batchSize * batches

	// 单生产者（批量）
	go func() {
		for b := 0; b < batches; b++ {
			batch := make([]int, batchSize)
			for i := 0; i < batchSize; i++ {
				batch[i] = b*batchSize + i
			}
			q.EnqueueBatch(batch)
		}
	}()

	// 单消费者（批量）
	received := 0
	buffer := make([]int, 20)
	for received < total {
		n := q.DequeueBatch(buffer)
		if n > 0 {
			for i := 0; i < n; i++ {
				if buffer[i] != received {
					t.Errorf("期望 %d，得到 %d", received, buffer[i])
				}
				received++
			}
		} else {
			runtime.Gosched()
		}
	}
}

// =============================================================================
// 基准测试
// =============================================================================

// --- UnboundedMPSC 基准测试（单线程，与 SPSC 可直接对比）---

func BenchmarkUnboundedMPSC_Enqueue(b *testing.B) {
	q := NewUnboundedMPSC[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
	}
}

func BenchmarkUnboundedMPSC_Dequeue(b *testing.B) {
	q := NewUnboundedMPSC[int]()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Dequeue()
	}
}

func BenchmarkUnboundedMPSC_EnqueueDequeue(b *testing.B) {
	q := NewUnboundedMPSC[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		q.Dequeue()
	}
}

func BenchmarkUnboundedMPSC_DequeueBatch(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, size := range sizes {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			q := NewUnboundedMPSC[int]()
			for i := 0; i < b.N*size; i++ {
				q.Enqueue(i)
			}

			buffer := make([]int, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.DequeueBatch(buffer)
			}
		})
	}
}

func BenchmarkUnboundedMPSC_EnqueueBatch(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, size := range sizes {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			q := NewUnboundedMPSC[int]()
			batch := make([]int, size)
			for i := 0; i < size; i++ {
				batch[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.EnqueueBatch(batch)
			}
		})
	}
}

// --- UnboundedMPSC 并发基准测试（多生产者吞吐量）---

func BenchmarkUnboundedMPSC_Enqueue_Parallel(b *testing.B) {
	q := NewUnboundedMPSC[int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		val := 0
		for pb.Next() {
			q.Enqueue(val)
			val++
		}
	})
}

func BenchmarkUnboundedMPSC_EnqueueBatch_Parallel(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, size := range sizes {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			q := NewUnboundedMPSC[int]()
			batch := make([]int, size)
			for i := 0; i < size; i++ {
				batch[i] = i
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					q.EnqueueBatch(batch)
				}
			})
		})
	}
}

func BenchmarkUnboundedMPSC_Concurrent_1P1C(b *testing.B) {
	benchmarkMPSCConcurrent(b, 1)
}

func BenchmarkUnboundedMPSC_Concurrent_2P1C(b *testing.B) {
	benchmarkMPSCConcurrent(b, 2)
}

func BenchmarkUnboundedMPSC_Concurrent_4P1C(b *testing.B) {
	benchmarkMPSCConcurrent(b, 4)
}

func BenchmarkUnboundedMPSC_Concurrent_8P1C(b *testing.B) {
	benchmarkMPSCConcurrent(b, 8)
}

func benchmarkMPSCConcurrent(b *testing.B, producers int) {
	q := NewUnboundedMPSC[int]()
	total := b.N
	itemsPerProducer := total / producers
	if itemsPerProducer == 0 {
		itemsPerProducer = 1
	}
	actualTotal := itemsPerProducer * producers

	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(producers + 1)

	// 生产者
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				q.Enqueue(i)
			}
		}()
	}

	// 消费者
	go func() {
		defer wg.Done()
		buffer := make([]int, 100)
		consumed := 0
		for consumed < actualTotal {
			n := q.DequeueBatch(buffer)
			consumed += n
		}
	}()

	wg.Wait()
}

// --- UnboundedSPSC 基准测试 ---

func BenchmarkUnboundedSPSC_Enqueue(b *testing.B) {
	q := NewUnboundedSPSC[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
	}
}

func BenchmarkUnboundedSPSC_Dequeue(b *testing.B) {
	q := NewUnboundedSPSC[int]()
	// 预填充
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Dequeue()
	}
}

func BenchmarkUnboundedSPSC_EnqueueDequeue(b *testing.B) {
	q := NewUnboundedSPSC[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		q.Dequeue()
	}
}

func BenchmarkUnboundedSPSC_DequeueBatch(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, size := range sizes {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			q := NewUnboundedSPSC[int]()
			// 预填充
			for i := 0; i < b.N*size; i++ {
				q.Enqueue(i)
			}

			buffer := make([]int, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.DequeueBatch(buffer)
			}
		})
	}
}

func BenchmarkUnboundedSPSC_EnqueueBatch(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, size := range sizes {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			q := NewUnboundedSPSC[int]()
			batch := make([]int, size)
			for i := 0; i < size; i++ {
				batch[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.EnqueueBatch(batch)
			}
		})
	}
}

func BenchmarkUnboundedSPSC_PingPong(b *testing.B) {
	q := NewUnboundedSPSC[int]()

	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	// 生产者
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
	}()

	// 消费者
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			for {
				_, ok := q.Dequeue()
				if ok {
					break
				}
				runtime.Gosched()
			}
		}
	}()

	wg.Wait()
}

func BenchmarkUnboundedSPSC_BatchPingPong(b *testing.B) {
	q := NewUnboundedSPSC[int]()
	batchSize := 100

	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	// 生产者
	go func() {
		defer wg.Done()
		batch := make([]int, batchSize)
		for i := 0; i < batchSize; i++ {
			batch[i] = i
		}
		for i := 0; i < b.N; i++ {
			q.EnqueueBatch(batch)
		}
	}()

	// 消费者
	go func() {
		defer wg.Done()
		buffer := make([]int, batchSize)
		consumed := 0
		target := b.N * batchSize
		for consumed < target {
			n := q.DequeueBatch(buffer)
			if n > 0 {
				consumed += n
			} else {
				runtime.Gosched()
			}
		}
	}()

	wg.Wait()
}

// --- 对比基准测试（相同方法论，可直接对比 ns/op）---

func BenchmarkComparison_MPSC_vs_SPSC_Enqueue(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
	})
}

func BenchmarkComparison_MPSC_vs_SPSC_Dequeue(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Dequeue()
		}
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Dequeue()
		}
	})
}

func BenchmarkComparison_MPSC_vs_SPSC_EnqueueDequeue(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
			q.Dequeue()
		}
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
			q.Dequeue()
		}
	})
}

func BenchmarkComparison_MPSC_vs_SPSC_EnqueueBatch(b *testing.B) {
	for _, size := range []int{10, 100} {
		b.Run(formatBatchSize(size), func(b *testing.B) {
			batch := make([]int, size)
			for i := range batch {
				batch[i] = i
			}

			b.Run("MPSC", func(b *testing.B) {
				q := NewUnboundedMPSC[int]()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					q.EnqueueBatch(batch)
				}
			})

			b.Run("SPSC", func(b *testing.B) {
				q := NewUnboundedSPSC[int]()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					q.EnqueueBatch(batch)
				}
			})
		})
	}
}

func BenchmarkComparison_MPSC_vs_SPSC_PingPong(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		var wg sync.WaitGroup
		wg.Add(2)

		b.ResetTimer()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				q.Enqueue(i)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				for {
					_, ok := q.Dequeue()
					if ok {
						break
					}
					runtime.Gosched()
				}
			}
		}()
		wg.Wait()
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		var wg sync.WaitGroup
		wg.Add(2)

		b.ResetTimer()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				q.Enqueue(i)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				for {
					_, ok := q.Dequeue()
					if ok {
						break
					}
					runtime.Gosched()
				}
			}
		}()
		wg.Wait()
	})
}

// --- 对比基准测试：Unbounded 队列 vs Channel ---

func BenchmarkComparison_Unbounded_vs_Channel_Enqueue(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			q.Enqueue(i)
		}
	})

	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 1024)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for range ch {
			}
		}()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- i
		}
		b.StopTimer()
		close(ch)
		<-done
	})
}

func BenchmarkComparison_Unbounded_vs_Channel_EnqueueParallel(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			val := 0
			for pb.Next() {
				q.Enqueue(val)
				val++
			}
		})
	})

	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 1024*1024)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for range ch {
			}
		}()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			val := 0
			for pb.Next() {
				ch <- val
				val++
			}
		})
		b.StopTimer()
		close(ch)
		<-done
	})
}

func BenchmarkComparison_Unbounded_vs_Channel_PingPong(b *testing.B) {
	b.Run("MPSC", func(b *testing.B) {
		q := NewUnboundedMPSC[int]()
		var wg sync.WaitGroup
		wg.Add(2)

		b.ResetTimer()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				q.Enqueue(i)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				for {
					_, ok := q.Dequeue()
					if ok {
						break
					}
					runtime.Gosched()
				}
			}
		}()
		wg.Wait()
	})

	b.Run("SPSC", func(b *testing.B) {
		q := NewUnboundedSPSC[int]()
		var wg sync.WaitGroup
		wg.Add(2)

		b.ResetTimer()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				q.Enqueue(i)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				for {
					_, ok := q.Dequeue()
					if ok {
						break
					}
					runtime.Gosched()
				}
			}
		}()
		wg.Wait()
	})

	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 1024)
		var wg sync.WaitGroup
		wg.Add(2)

		b.ResetTimer()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				ch <- i
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				<-ch
			}
		}()
		wg.Wait()
	})
}

// =============================================================================
// 辅助函数
// =============================================================================

func formatBatchSize(size int) string {
	return strconv.Itoa(size)
}
