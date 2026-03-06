package zqueue

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ==========================================
// 1. 基础功能测试
// ==========================================

func TestNewMPMCQueue_Capacity(t *testing.T) {
	// 测试容量自动修正为 2 的幂
	q := NewMPSCQueue[int](10) // 应该修正为 16
	if q.Cap() != 16 {
		t.Errorf("expected cap 16, got %d", q.Cap())
	}

	q2 := NewMPSCQueue[int](16)
	if q2.Cap() != 16 {
		t.Errorf("expected cap 16, got %d", q2.Cap())
	}
}

func TestSimpleEnqueueDequeue(t *testing.T) {
	q := NewMPSCQueue[int](4)

	// 写入 1, 2
	if !q.Enqueue(1) {
		t.Error("enqueue 1 failed")
	}
	if !q.Enqueue(2) {
		t.Error("enqueue 2 failed")
	}

	// 读取
	if val, ok := q.Dequeue(); !ok || val != 1 {
		t.Errorf("dequeue expected 1, got %d (ok=%v)", val, ok)
	}
	if val, ok := q.Dequeue(); !ok || val != 2 {
		t.Errorf("dequeue expected 2, got %d (ok=%v)", val, ok)
	}

	// 队列空
	// 注意：Dequeue 在空时会自旋等待，所以不能直接调 Dequeue 测试空，
	// 除非我们在另一协程 Close，或者修改 Dequeue 为非阻塞。
	// 这里我们通过 Close 来测试 "空且关闭" 的情况。
	q.Close()
	if _, ok := q.Dequeue(); ok {
		t.Error("expected empty & closed queue to return false")
	}
}

// ==========================================
// 3. 高并发正确性测试 (核心)
// ==========================================

func TestMPSCQueue_SoakTest(t *testing.T) {
	// 1. 创建一个小容量队列，强迫发生频繁的 Full/WrapAround
	q := NewMPSCQueue[int](1024)

	const producerCount = 100
	const msgsPerProducer = 10000
	const totalMsgs = producerCount * msgsPerProducer

	var wg sync.WaitGroup
	wg.Add(producerCount)

	// 2. 启动多个生产者
	for i := 0; i < producerCount; i++ {
		go func(pid int) {
			defer wg.Done()
			for j := 0; j < msgsPerProducer; j++ {
				// Enqueue 非阻塞：队列满时返回 false，需主动重试
				for !q.Enqueue(1) {
					runtime.Gosched()
				}
			}
		}(i)
	}

	// 3. 启动单个消费者
	var receivedCount int64
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, ok := q.Dequeue()
			if ok {
				atomic.AddInt64(&receivedCount, 1)
			} else {
				// 队列空了，歇一会
				// 如果生产者都结束了，且队列空了，就退出
				if atomic.LoadInt64(&receivedCount) == int64(totalMsgs) {
					return
				}
				runtime.Gosched()
			}
		}
	}()

	// 4. 等待生产者结束
	wg.Wait()

	// 5. 等待消费者消费完剩余数据
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout! Lost data? Received: %d, Expected: %d", atomic.LoadInt64(&receivedCount), totalMsgs)
	}

	finalCount := atomic.LoadInt64(&receivedCount)
	if finalCount != int64(totalMsgs) {
		t.Fatalf("Data Lost! Expected %d, got %d", totalMsgs, finalCount)
	} else {
		t.Logf("Success! Processed %d messages without loss.", finalCount)
	}
}

// TestMPSCQueue_Padded_SoakTest 验证 Padded 版本在高并发下的正确性
func TestMPSCQueue_Padded_SoakTest(t *testing.T) {
	q := NewMPSCQueuePadded[int](1024)

	const producerCount = 100
	const msgsPerProducer = 10000
	const totalMsgs = producerCount * msgsPerProducer

	var wg sync.WaitGroup
	wg.Add(producerCount)

	for i := 0; i < producerCount; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < msgsPerProducer; j++ {
				for !q.Enqueue(1) {
					runtime.Gosched()
				}
			}
		}()
	}

	var receivedCount int64
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, ok := q.Dequeue()
			if ok {
				atomic.AddInt64(&receivedCount, 1)
			} else {
				if atomic.LoadInt64(&receivedCount) == int64(totalMsgs) {
					return
				}
				runtime.Gosched()
			}
		}
	}()

	wg.Wait()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout! Received: %d, Expected: %d", atomic.LoadInt64(&receivedCount), totalMsgs)
	}

	finalCount := atomic.LoadInt64(&receivedCount)
	if finalCount != int64(totalMsgs) {
		t.Fatalf("Data Lost! Expected %d, got %d", totalMsgs, finalCount)
	}
}

// ==========================================
// 4. 关闭逻辑测试 (Drain 模式)
// ==========================================

func TestCloseDrain(t *testing.T) {
	q := NewMPSCQueue[int](16)

	// 写入数据
	q.Enqueue(100)
	q.Enqueue(200)

	// 先关闭
	q.Close()

	// 再读取，应该能读出数据
	v1, ok1 := q.Dequeue()
	if !ok1 || v1 != 100 {
		t.Error("Should be able to drain queue after close")
	}

	v2, ok2 := q.Dequeue()
	if !ok2 || v2 != 200 {
		t.Error("Should be able to drain queue after close")
	}

	// 再次读取，应返回 false
	_, ok3 := q.Dequeue()
	if ok3 {
		t.Error("Should return false when empty and closed")
	}
}
func TestSmartDoubleQueue_Basic(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false)

	// 测试写入
	for i := 0; i < 5; i++ {
		if ok := q.Enqueue(i); !ok {
			t.Fatalf("Enqueue failed: %v", ok)
		}
	}

	// 第一次 Pop（应该触发 Swap）
	batch, ok := q.Pop()
	if !ok {
		t.Fatal("Pop returned false unexpectedly")
	}
	if len(batch) != 5 {
		t.Errorf("Expected batch size 5, got %d", len(batch))
	}

	// 验证数据顺序
	for i, v := range batch {
		if v != i {
			t.Errorf("Expected %d, got %d", i, v)
		}
	}

	// 再次 Pop 应该为空
	batch2, ok2 := q.Pop()
	if !ok2 {
		t.Fatal("Pop returned false unexpectedly")
	}
	if len(batch2) != 0 {
		t.Errorf("Expected empty batch, got length %d", len(batch2))
	}
}

func TestSmartDoubleQueue_MaxCap(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 5, false) // MaxCap = 5

	for i := 0; i < 5; i++ {
		if ok := q.Enqueue(i); !ok {
			t.Fatalf("Enqueue failed at %d: %v", i, ok)
		}
	}

	// 第6个应该报错
	if ok := q.Enqueue(6); ok {
		t.Errorf("Expected ErrQueueFull, got %v", ok)
	}

	// Pop 之后应该能继续写
	q.Pop()
	if ok := q.Enqueue(7); !ok {
		t.Errorf("Enqueue failed after Pop: %v", ok)
	}
}

func TestSmartDoubleQueue_Concurrency(t *testing.T) {
	q := NewSmartDoubleQueue[int](1024, 0, false) // 无限制容量
	producerCount := 10
	itemsPerProducer := 1000
	totalItems := producerCount * itemsPerProducer

	var wg sync.WaitGroup
	wg.Add(producerCount)

	// 并发写入
	for i := 0; i < producerCount; i++ {
		go func(pid int) {
			defer wg.Done()
			for j := 0; j < itemsPerProducer; j++ {
				_ = q.Enqueue(pid*itemsPerProducer + j)
			}
		}(i)
	}

	// 单消费者读取
	receivedCount := 0
	done := make(chan struct{})

	go func() {
		defer close(done)
		// 简单的轮询消费
		timeout := time.After(2 * time.Second)
		for {
			select {
			case <-q.Signal():
				batch, ok := q.Pop()
				if !ok {
					return
				}
				receivedCount += len(batch)
				if receivedCount == totalItems {
					return
				}
			case <-timeout:
				// 防止死锁，最后再 check 一次
				batch, _ := q.Pop()
				receivedCount += len(batch)
				return
			}
		}
	}()

	wg.Wait()
	<-done

	if receivedCount != totalItems {
		// 注意：由于是最后一次 Pop 可能没触发 Signal，这里手动再 Pop 一次兜底验证
		batch, _ := q.Pop()
		receivedCount += len(batch)
	}

	if receivedCount != totalItems {
		t.Errorf("Data loss! Expected %d, got %d", totalItems, receivedCount)
	}
}

func TestSmartDoubleQueue_Close(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 0, false)
	q.Enqueue(1)
	q.Close()

	// 关闭后不允许写入
	if ok := q.Enqueue(2); ok {
		t.Errorf("Expected ErrQueueDisposed, go")
	}

	// 关闭后应该能读出剩余数据
	batch, ok := q.Pop()
	if !ok {
		t.Error("Should be able to read remaining data after close")
	}
	if len(batch) != 1 || batch[0] != 1 {
		t.Error("Data mismatch after close")
	}

	// 再次读，应该返回 false
	batch, ok = q.Pop()
	if ok {
		t.Error("Should return false when closed and empty")
	}
	if batch != nil {
		t.Error("Batch should be nil")
	}
}

// ==========================================
// 补充单元测试
// ==========================================

func TestSmartDoubleQueue_EnqueueEmpty(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false)
	// 空参数入队应返回 true
	if ok := q.Enqueue(); !ok {
		t.Error("Enqueue with no args should return true")
	}
}

func TestSmartDoubleQueue_BatchEnqueue(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false)

	// 批量入队
	if ok := q.Enqueue(1, 2, 3, 4, 5); !ok {
		t.Error("batch enqueue failed")
	}

	batch, ok := q.Pop()
	if !ok {
		t.Fatal("Pop returned false")
	}
	if len(batch) != 5 {
		t.Errorf("expected batch size 5, got %d", len(batch))
	}
}

func TestSmartDoubleQueue_Len(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false)

	if q.Len() != 0 {
		t.Errorf("empty queue len should be 0, got %d", q.Len())
	}

	q.Enqueue(1, 2, 3)
	if q.Len() != 3 {
		t.Errorf("after enqueue 3, len should be 3, got %d", q.Len())
	}

	q.Pop()
	if q.Len() != 0 {
		t.Errorf("after pop, len should be 0, got %d", q.Len())
	}
}

func TestSmartDoubleQueue_Signal(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, true) // isNotify=true

	// Signal 通道应该非 nil
	if q.Signal() == nil {
		t.Fatal("Signal channel should not be nil")
	}

	q.Enqueue(42)

	// 应该能从 Signal 收到通知
	select {
	case <-q.Signal():
		// 成功
	case <-time.After(100 * time.Millisecond):
		t.Error("should receive signal after enqueue")
	}
}

func TestSmartDoubleQueue_NoSignal(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false) // isNotify=false

	q.Enqueue(42)

	// 不应该收到信号
	select {
	case <-q.Signal():
		t.Error("should not receive signal when isNotify=false")
	case <-time.After(50 * time.Millisecond):
		// 正确
	}
}

func TestSmartDoubleQueue_PopEmpty(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 100, false)

	batch, ok := q.Pop()
	if !ok {
		t.Error("Pop on empty queue should return ok=true")
	}
	if len(batch) != 0 {
		t.Error("Pop on empty queue should return empty batch")
	}
}

func TestSmartDoubleQueue_DoubleClose(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 0, false)
	q.Close()
	// 第二次 Close 不应 panic
	q.Close()
}

func TestSmartDoubleQueue_MaxCapBatchExceeds(t *testing.T) {
	q := NewSmartDoubleQueue[int](10, 3, false)

	// 写入超过 maxCap 的批量数据
	if ok := q.Enqueue(1, 2, 3, 4); ok {
		t.Error("batch exceeding maxCap should return false")
	}

	// 单条应该可以写入
	if ok := q.Enqueue(1); !ok {
		t.Error("single enqueue within maxCap should succeed")
	}
}

func TestMPSCQueue_CloseThenEnqueue(t *testing.T) {
	q := NewMPSCQueue[int](16)
	q.Close()

	if ok := q.Enqueue(1); ok {
		t.Error("Enqueue after close should return false")
	}
}

func TestMPSCQueue_MinCapacity(t *testing.T) {
	// 容量 1 是 2 的幂，roundUp 不会修正
	q := NewMPSCQueue[int](1)
	if q.Cap() != 1 {
		t.Errorf("expected cap 1 (power of 2), got %d", q.Cap())
	}

	// 容量 3 应该被修正为 4
	q2 := NewMPSCQueue[int](3)
	if q2.Cap() != 4 {
		t.Errorf("expected cap 4, got %d", q2.Cap())
	}
}

func TestSmartDoubleQueue_MinInitCap(t *testing.T) {
	// initCap < 64 应该被修正为 64
	q := NewSmartDoubleQueue[int](1, 0, false)
	q.Enqueue(1)
	batch, ok := q.Pop()
	if !ok || len(batch) != 1 || batch[0] != 1 {
		t.Error("should work with corrected initCap")
	}
}

// ==========================================
// 5. 基准测试 (BenchmarkSwap) vs Channel
// ==========================================

// BenchmarkSwapMPSC_EnqueueDequeue 测试 MPSC 队列吞吐量
// ✅ 修复: MPSCQueue 是单消费者队列，不能在 RunParallel 中并发 Dequeue
// 正确模式：多 goroutine 并发 Enqueue，单 goroutine 串行 Dequeue
func BenchmarkSwapMPSC_EnqueueDequeue(b *testing.B) {
	q := NewMPSCQueue[int](1024 * 4)

	// 单消费者 goroutine
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				q.Dequeue()
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for !q.Enqueue(1) {
				runtime.Gosched()
			}
		}
	})
	b.StopTimer()
	close(done)
}

// BenchmarkSwapChannel_EnqueueDequeue 对照组：原生 Channel
func BenchmarkSwapChannel_EnqueueDequeue(b *testing.B) {
	ch := make(chan int, 1024*4)

	// 单消费者 goroutine
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ch <- 1
		}
	})
	b.StopTimer()
	close(done)
}

// BenchmarkSwapMPMC_Uncontended 单线程无竞争基准
func BenchmarkSwapMPMC_Uncontended(b *testing.B) {
	q := NewMPSCQueue[int](1024 * 4)
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		q.Dequeue()
	}
}

// BenchmarkSwapChannel_Uncontended 单线程无竞争 Channel
func BenchmarkSwapChannel_Uncontended(b *testing.B) {
	ch := make(chan int, 1024*4)
	for i := 0; i < b.N; i++ {
		ch <- i
		<-ch
	}
}

// BenchmarkSwapMPSC_Padded_EnqueueDequeue Padded 版并发吞吐
func BenchmarkSwapMPSC_Padded_EnqueueDequeue(b *testing.B) {
	q := NewMPSCQueuePadded[int](1024 * 4)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				q.Dequeue()
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for !q.Enqueue(1) {
				runtime.Gosched()
			}
		}
	})
	b.StopTimer()
	close(done)
}

// BenchmarkSwapMPMC_Padded_Uncontended Padded 版单线程无竞争
func BenchmarkSwapMPMC_Padded_Uncontended(b *testing.B) {
	q := NewMPSCQueuePadded[int](1024 * 4)
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		q.Dequeue()
	}
}

// BenchmarkSwapMPSC_BatchCAS 批量 CAS 入队吞吐
func BenchmarkSwapMPSC_BatchCAS(b *testing.B) {
	for _, batchSize := range []int{10, 100} {
		b.Run("Compact/"+strconv.Itoa(batchSize), func(b *testing.B) {
			q := NewMPSCQueue[int](1024 * 64)
			items := make([]int, batchSize)

			done := make(chan struct{})
			go func() {
				buf := make([]int, batchSize)
				for {
					select {
					case <-done:
						return
					default:
						q.DequeueBatch(buf)
					}
				}
			}()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for q.EnqueueBatch(items) == 0 {
						runtime.Gosched()
					}
				}
			})
			b.StopTimer()
			close(done)
		})

		b.Run("Padded/"+strconv.Itoa(batchSize), func(b *testing.B) {
			q := NewMPSCQueuePadded[int](1024 * 64)
			items := make([]int, batchSize)

			done := make(chan struct{})
			go func() {
				buf := make([]int, batchSize)
				for {
					select {
					case <-done:
						return
					default:
						q.DequeueBatch(buf)
					}
				}
			}()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for q.EnqueueBatch(items) == 0 {
						runtime.Gosched()
					}
				}
			})
			b.StopTimer()
			close(done)
		})
	}
}

// 场景：多生产者并发写入，单消费者批量读取
// 这是该数据结构最典型的使用场景
func BenchmarkSwapSmartDoubleQueue_Throughput(b *testing.B) {
	q := NewSmartDoubleQueue[int](1024*4, 0, false)
	const batchSize = 100 // 模拟每次处理一批

	b.Run("SmartQueue", func(b *testing.B) {
		var wg sync.WaitGroup

		// 启动消费者
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := 0
			for {
				// 模拟 select 监听
				<-q.Signal()
				batch, ok := q.Pop()
				if !ok {
					return
				}
				count += len(batch)
				if count >= b.N {
					return
				}
			}
		}()

		// 启动生产者（模拟高并发）
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				q.Enqueue(1)
			}
		})

		// 这里的测试逻辑稍微有点 tricky，因为 b.N 是总操作数
		// 我们通过关闭队列来停止消费者，或者让消费者计数达到 b.N
	})
}

// 下面是一个更纯粹的 Enqueue 性能测试，不包含消费开销
// 用来测试互斥锁 + append 的写入性能
func BenchmarkSwapSmartDoubleQueue_EnqueueOnly(b *testing.B) {
	q := NewSmartDoubleQueue[int](1024*1024, 0, false) // 足够大避免扩容干扰
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.Enqueue(1)
		}
	})
}

// 综合场景：一边写一边读
// SmartQueue 的优势在于 Pop 是一次拿走所有，锁持有时间极短
func BenchmarkSwapSmartDoubleQueue_ReadWrite(b *testing.B) {
	q := NewSmartDoubleQueue[int](1024, 0, false)

	go func() {
		for {
			_, ok := q.Pop()
			if !ok {
				// 这里的 ok 只在 Close 后才会 false，或者我们不管它
				// 在 BenchmarkSwap 里通常一直跑
				// 为了避免死循环占满 CPU，空转时 Gosched
				// runtime.Gosched()
			}
			<-q.Signal() // 简单的等待信号
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.Enqueue(1)
		}
	})
}

func BenchmarkSwapChannel_ReadWrite(b *testing.B) {
	ch := make(chan int, 1024)

	go func() {
		for range ch {
			// 消费
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ch <- 1
		}
	})
}
