// Package zqueue 96 组合基准测试矩阵
//
// 维度：类型(4) × 数据大小(3) × 生产者(4) × 消费(2) = 96
//
// 类型：MPSC有界、MPSC无界、Channel有缓冲、Channel无缓冲
// 数据大小：小(256)、中(4096)、大(65536) — 有界/有缓冲的容量，无界/无缓冲时对应 batchSize
// 生产者：1、4、16、64
// 消费：单条、批量
package zqueue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// 容量与 batch 配置
const (
	capSmall  = 256
	capMedium = 4096
	capLarge  = 65536

	batchSmall  = 32
	batchMedium = 128
	batchLarge  = 512
)

var (
	dataSizeCaps   = []int{capSmall, capMedium, capLarge}
	dataSizeNames  = []string{"Small", "Medium", "Large"}
	batchSizes     = []int{batchSmall, batchMedium, batchLarge}
	producerCounts = []int{1, 4, 16, 64}
)

func BenchmarkMatrix(b *testing.B) {
	for _, typ := range []string{"MPSCBounded", "MPSCUnbounded", "ChanBuffered", "ChanUnbuffered"} {
		typ := typ
		b.Run(typ, func(b *testing.B) {
			for ds := 0; ds < 3; ds++ {
				capOrBatch := dataSizeCaps[ds]
				batchSize := batchSizes[ds]
				dsName := dataSizeNames[ds]

				b.Run(dsName, func(b *testing.B) {
					for _, p := range producerCounts {
						b.Run(producerName(p), func(b *testing.B) {
							b.Run("Single", func(b *testing.B) {
								runMatrixBench(b, typ, capOrBatch, batchSize, p, true)
							})
							b.Run("Batch", func(b *testing.B) {
								runMatrixBench(b, typ, capOrBatch, batchSize, p, false)
							})
						})
					}
				})
			}
		})
	}
}

func producerName(p int) string {
	switch p {
	case 1:
		return "P1"
	case 4:
		return "P4"
	case 16:
		return "P16"
	case 64:
		return "P64"
	default:
		return "P?"
	}
}

func runMatrixBench(b *testing.B, typ string, capOrBatch, batchSize, producers int, singleConsumer bool) {
	switch typ {
	case "MPSCBounded":
		benchMPSCBounded(b, capOrBatch, batchSize, producers, singleConsumer)
	case "MPSCUnbounded":
		benchMPSCUnbounded(b, batchSize, producers, singleConsumer)
	case "ChanBuffered":
		benchChanBuffered(b, capOrBatch, batchSize, producers, singleConsumer)
	case "ChanUnbuffered":
		benchChanUnbuffered(b, batchSize, producers, singleConsumer)
	}
}

func benchMPSCBounded(b *testing.B, cap, batchSize, producers int, singleConsumer bool) {
	q := NewMPSCQueue[int](cap)
	runProducersConsumers(b, q, nil, nil, cap, batchSize, producers, singleConsumer,
		func(q *MPSCQueue[int], v int) bool {
			for !q.Enqueue(v) {
				runtime.Gosched()
			}
			return true
		},
		func(q *MPSCQueue[int], buf []int) int {
			return q.DequeueBatch(buf)
		},
		func(q *MPSCQueue[int], buf []int) (int, bool) {
			v, ok := q.Dequeue()
			if ok {
				if len(buf) > 0 {
					buf[0] = v
				}
				return 1, true
			}
			return 0, false
		},
	)
}

func benchMPSCUnbounded(b *testing.B, batchSize, producers int, singleConsumer bool) {
	q := NewUnboundedMPSC[int]()
	runProducersConsumersUnbounded(b, q, batchSize, producers, singleConsumer,
		func(q *UnboundedMPSC[int], v int) {
			q.Enqueue(v)
		},
		func(q *UnboundedMPSC[int], buf []int) int {
			return q.DequeueBatch(buf)
		},
		func(q *UnboundedMPSC[int]) (int, bool) {
			v, ok := q.Dequeue()
			_ = v
			return 0, ok
		},
	)
}

func benchChanBuffered(b *testing.B, cap, batchSize, producers int, singleConsumer bool) {
	ch := make(chan int, cap)
	runProducersConsumersChan(b, ch, cap, batchSize, producers, singleConsumer, false)
}

func benchChanUnbuffered(b *testing.B, batchSize, producers int, singleConsumer bool) {
	ch := make(chan int)
	runProducersConsumersChan(b, ch, 0, batchSize, producers, singleConsumer, true)
}

// --- MPSC 有界 ---
func runProducersConsumers(b *testing.B, q *MPSCQueue[int], _, _ interface{}, cap, batchSize, producers int, singleConsumer bool,
	enqueue func(*MPSCQueue[int], int) bool,
	dequeueBatch func(*MPSCQueue[int], []int) int,
	dequeueOne func(*MPSCQueue[int], []int) (int, bool),
) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed int64

	// singleConsumer=true => 单条消费 (Single), false => 批量消费 (Batch)
	if singleConsumer {
		go func() {
			for atomic.LoadInt64(&consumed) < int64(total) {
				n, ok := dequeueOne(q, nil)
				if ok && n > 0 {
					atomic.AddInt64(&consumed, int64(n))
				} else if !ok {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	} else {
		buf := make([]int, batchSize)
		go func() {
			for atomic.LoadInt64(&consumed) < int64(total) {
				n := dequeueBatch(q, buf)
				if n > 0 {
					atomic.AddInt64(&consumed, int64(n))
				} else {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	}

	b.ResetTimer()
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			start := pid * opsPerProducer
			end := start + opsPerProducer
			if end > total {
				end = total
			}
			for i := start; i < end; i++ {
				enqueue(q, i)
			}
		}(p)
	}
	wg.Wait()
	b.StopTimer()

	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done
}

// --- MPSC 无界 ---
type unboundedEnqueue func(*UnboundedMPSC[int], int)
type unboundedDequeueBatch func(*UnboundedMPSC[int], []int) int
type unboundedDequeueOne func(*UnboundedMPSC[int]) (int, bool)

func runProducersConsumersUnbounded(b *testing.B, q *UnboundedMPSC[int], batchSize, producers int, singleConsumer bool,
	enqueue unboundedEnqueue,
	dequeueBatch unboundedDequeueBatch,
	dequeueOne unboundedDequeueOne,
) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed int64

	// singleConsumer=true => 单条消费 (Single), false => 批量消费 (Batch)
	if singleConsumer {
		go func() {
			for atomic.LoadInt64(&consumed) < int64(total) {
				_, ok := dequeueOne(q)
				if ok {
					atomic.AddInt64(&consumed, 1)
				} else {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	} else {
		buf := make([]int, batchSize)
		go func() {
			for atomic.LoadInt64(&consumed) < int64(total) {
				n := dequeueBatch(q, buf)
				if n > 0 {
					atomic.AddInt64(&consumed, int64(n))
				} else {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	}

	b.ResetTimer()
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			start := pid * opsPerProducer
			end := start + opsPerProducer
			if end > total {
				end = total
			}
			for i := start; i < end; i++ {
				enqueue(q, i)
			}
		}(p)
	}
	wg.Wait()
	b.StopTimer()

	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done
}

// --- Channel ---
func runProducersConsumersChan(b *testing.B, ch chan int, cap, batchSize, producers int, singleConsumer bool, unbuffered bool) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed int64

	if unbuffered {
		// 无缓冲：每个 send 阻塞直到 recv，消费者持续 recv
		if singleConsumer {
			go func() {
				for atomic.LoadInt64(&consumed) < int64(total) {
					<-ch
					atomic.AddInt64(&consumed, 1)
				}
				close(done)
			}()
		} else {
			go func() {
				for atomic.LoadInt64(&consumed) < int64(total) {
					for i := 0; i < batchSize && atomic.LoadInt64(&consumed) < int64(total); i++ {
						<-ch
						atomic.AddInt64(&consumed, 1)
					}
				}
				close(done)
			}()
		}
	} else {
		// singleConsumer=true => 单条消费, false => 批量消费
		if singleConsumer {
			go func() {
				for atomic.LoadInt64(&consumed) < int64(total) {
					select {
					case <-ch:
						atomic.AddInt64(&consumed, 1)
					default:
						runtime.Gosched()
					}
				}
				close(done)
			}()
		} else {
			buf := make([]int, batchSize)
			go func() {
				for atomic.LoadInt64(&consumed) < int64(total) {
					n := 0
				batchLoop:
					for n < batchSize {
						select {
						case v := <-ch:
							buf[n] = v
							n++
						default:
							break batchLoop
						}
					}
					if n > 0 {
						atomic.AddInt64(&consumed, int64(n))
					} else {
						runtime.Gosched()
					}
				}
				close(done)
			}()
		}
	}

	b.ResetTimer()
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			start := pid * opsPerProducer
			end := start + opsPerProducer
			if end > total {
				end = total
			}
			for i := start; i < end; i++ {
				ch <- i
			}
		}(p)
	}
	wg.Wait()
	b.StopTimer()

	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done

	// 排空 channel 中可能剩余的元素（有缓冲时）
	if cap > 0 {
		for len(ch) > 0 {
			<-ch
		}
	}
}
