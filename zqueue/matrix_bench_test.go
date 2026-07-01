// Package zqueue 组合基准测试矩阵
//
// 维度：类型(11) × 载荷档位(3) × 生产者(4) × 消费(2)
//
// 队列类型（zqueue 全部实现 + channel 基线）：
//   - MPSCBounded / MPSCPadded / MPSCUnbounded
//   - SPSCBounded / SPSCUnbounded（仅 P1，SPSC 语义）
//   - QueueResize / QueueDrop（mutex 环形队列）
//   - SmartDouble / Priority
//   - ChanBuffered / ChanUnbuffered
//
// 载荷档位：Small(256B)、Medium(4KB)、Large(~64KB 值类型，65528B，受 Go channel 元素 <64KiB 约束)
// 有界队列槽位数与 batch 窗口按档位对齐；ChanBuffered Large 缓冲槽位用 chanBufLarge(256)，避免 65536×64KB≈4GiB
// 生产者：1、4、16、64
// 消费：单条 API、批量 API（SmartDouble 仅 Batch；Priority 仅 Single，见 Skip）
//
// 计时覆盖「并发入队 + 排空」全链路。QueueDrop 按实际 consumed 计 SetBytes（非 b.N 尝试次数）。
//
// 组合数：9×3×4×2 + 2×3×1×2 − 12(SmartDouble Single) − 12(Priority Batch) = 204
package zqueue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	capSmall  = 256
	capMedium = 4096
	capLarge  = 65536

	batchSmall  = 32
	batchMedium = 128
	batchLarge  = 512

	chanBufLarge = 256 // Large 档 channel 缓冲槽位（元素为 ~64KB 值类型，不可与 capLarge 同槽数）

	payloadSmallBytes  = 256
	payloadMediumBytes = 4096
	// payloadLargeBytes Go channel 元素须 <65536；8+65520=65528 为 Large 档统一值载荷（队列与 channel 一致按值拷贝）
	payloadLargeBytes = 65528
)

type matrixItem256 struct {
	Seq int64
	Pad [payloadSmallBytes - 8]byte
}

type matrixItem4K struct {
	Seq int64
	Pad [payloadMediumBytes - 8]byte
}

type matrixItem64K struct {
	Seq int64
	Pad [payloadLargeBytes - 8]byte
}

func newMatrixItem256(seq int) matrixItem256 { return matrixItem256{Seq: int64(seq)} }
func newMatrixItem4K(seq int) matrixItem4K   { return matrixItem4K{Seq: int64(seq)} }
func newMatrixItem64K(seq int) matrixItem64K { return matrixItem64K{Seq: int64(seq)} }

func matrixPayloadBytes(ds int) int64 {
	switch ds {
	case 0:
		return payloadSmallBytes
	case 1:
		return payloadMediumBytes
	case 2:
		return payloadLargeBytes
	default:
		return payloadSmallBytes
	}
}

// chanBufferSlots 返回 ChanBuffered 缓冲槽位数；Large 与有界队列 capLarge 解耦。
func chanBufferSlots(ds int) int {
	if ds == 2 {
		return chanBufLarge
	}
	return dataSizeCaps[ds]
}

var (
	dataSizeCaps   = []int{capSmall, capMedium, capLarge}
	dataSizeNames  = []string{"Small", "Medium", "Large"}
	batchSizes     = []int{batchSmall, batchMedium, batchLarge}
	producerCounts = []int{1, 4, 16, 64}
	matrixTypes    = []string{
		"MPSCBounded",
		"MPSCPadded",
		"MPSCUnbounded",
		"SPSCBounded",
		"SPSCUnbounded",
		"QueueResize",
		"QueueDrop",
		"SmartDouble",
		"Priority",
		"ChanBuffered",
		"ChanUnbuffered",
	}
)

type matrixEnqueuePolicy int

const (
	matrixEnqueueBlock matrixEnqueuePolicy = iota
	matrixEnqueueDrop
)

func BenchmarkMatrix(b *testing.B) {
	for _, typ := range matrixTypes {
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
								runMatrixBench(b, typ, ds, capOrBatch, batchSize, p, true)
							})
							b.Run("Batch", func(b *testing.B) {
								runMatrixBench(b, typ, ds, capOrBatch, batchSize, p, false)
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

func matrixTypeSPSCOnly(typ string) bool {
	return typ == "SPSCBounded" || typ == "SPSCUnbounded"
}

func runMatrixBench(b *testing.B, typ string, ds, capOrBatch, batchSize, producers int, singleConsumer bool) {
	if matrixTypeSPSCOnly(typ) && producers != 1 {
		b.Skip("SPSC 队列仅支持单生产者")
		return
	}
	if typ == "SmartDouble" && singleConsumer {
		b.Skip("SmartDouble 仅 Pop 批量 API，无单条 Dequeue 语义")
		return
	}
	if typ == "Priority" && !singleConsumer {
		b.Skip("PriorityQueue 无批量 Dequeue API")
		return
	}

	bytesPerItem := matrixPayloadBytes(ds)
	if typ != "QueueDrop" {
		b.SetBytes(bytesPerItem)
	}
	b.ReportAllocs()

	if typ == "ChanBuffered" || typ == "ChanUnbuffered" {
		runMatrixChan(b, typ, ds, batchSize, producers, singleConsumer)
		return
	}

	switch ds {
	case 0:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, singleConsumer, bytesPerItem, newMatrixItem256, priorityOf256)
	case 1:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, singleConsumer, bytesPerItem, newMatrixItem4K, priorityOf4K)
	case 2:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, singleConsumer, bytesPerItem, newMatrixItem64K, priorityOf64K)
	default:
		b.Fatalf("unknown data size index %d", ds)
	}
}

func priorityOf256(v matrixItem256) int { return int(v.Seq % 17) }
func priorityOf4K(v matrixItem4K) int   { return int(v.Seq % 17) }
func priorityOf64K(v matrixItem64K) int { return int(v.Seq % 17) }

func runMatrixChan(b *testing.B, typ string, ds, batchSize, producers int, singleConsumer bool) {
	bufSlots := chanBufferSlots(ds)
	switch ds {
	case 0:
		if typ == "ChanBuffered" {
			benchChanBuffered(b, bufSlots, batchSize, producers, singleConsumer, newMatrixItem256)
		} else {
			benchChanUnbuffered(b, batchSize, producers, singleConsumer, newMatrixItem256)
		}
	case 1:
		if typ == "ChanBuffered" {
			benchChanBuffered(b, bufSlots, batchSize, producers, singleConsumer, newMatrixItem4K)
		} else {
			benchChanUnbuffered(b, batchSize, producers, singleConsumer, newMatrixItem4K)
		}
	case 2:
		if typ == "ChanBuffered" {
			benchChanBuffered(b, bufSlots, batchSize, producers, singleConsumer, newMatrixItem64K)
		} else {
			benchChanUnbuffered(b, batchSize, producers, singleConsumer, newMatrixItem64K)
		}
	default:
		b.Fatalf("unknown data size index %d", ds)
	}
}

func dispatchMatrixSkipChan[T any](b *testing.B, typ string, capOrBatch, batchSize, producers int, singleConsumer bool, bytesPerItem int64, mk func(int) T, prio func(T) int) {
	switch typ {
	case "MPSCBounded":
		benchMPSCBounded(b, capOrBatch, batchSize, producers, singleConsumer, false, bytesPerItem, mk)
	case "MPSCPadded":
		benchMPSCBounded(b, capOrBatch, batchSize, producers, singleConsumer, true, bytesPerItem, mk)
	case "MPSCUnbounded":
		benchMPSCUnbounded(b, batchSize, producers, singleConsumer, bytesPerItem, mk)
	case "SPSCBounded":
		benchSPSCBounded(b, capOrBatch, batchSize, singleConsumer, mk)
	case "SPSCUnbounded":
		benchSPSCUnbounded(b, batchSize, singleConsumer, mk)
	case "QueueResize":
		benchQueue(b, capOrBatch, batchSize, producers, singleConsumer, true, matrixEnqueueBlock, bytesPerItem, mk)
	case "QueueDrop":
		benchQueue(b, capOrBatch, batchSize, producers, singleConsumer, false, matrixEnqueueDrop, bytesPerItem, mk)
	case "SmartDouble":
		benchSmartDouble(b, capOrBatch, batchSize, producers, mk)
	case "Priority":
		benchPriority(b, capOrBatch, batchSize, producers, singleConsumer, bytesPerItem, mk, prio)
	default:
		b.Fatalf("unknown matrix type %q", typ)
	}
}

func benchMPSCBounded[T any](b *testing.B, cap, batchSize, producers int, singleConsumer, padded bool, bytesPerItem int64, mk func(int) T) {
	var q *MPSCQueue[T]
	if padded {
		q = NewMPSCQueuePadded[T](cap)
	} else {
		q = NewMPSCQueue[T](cap)
	}
	runProducersConsumers(b, q, batchSize, producers, singleConsumer, matrixEnqueueBlock, bytesPerItem,
		func(q *MPSCQueue[T], v T) bool {
			for !q.Enqueue(v) {
				runtime.Gosched()
			}
			return true
		},
		func(q *MPSCQueue[T], buf []T) int {
			return q.DequeueBatch(buf)
		},
		func(q *MPSCQueue[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		mk,
	)
}

func benchMPSCUnbounded[T any](b *testing.B, batchSize, producers int, singleConsumer bool, bytesPerItem int64, mk func(int) T) {
	q := NewUnboundedMPSC[T]()
	runProducersConsumers(b, q, batchSize, producers, singleConsumer, matrixEnqueueBlock, bytesPerItem,
		func(q *UnboundedMPSC[T], v T) bool {
			q.Enqueue(v)
			return true
		},
		func(q *UnboundedMPSC[T], buf []T) int {
			return q.DequeueBatch(buf)
		},
		func(q *UnboundedMPSC[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		mk,
	)
}

func benchSPSCBounded[T any](b *testing.B, cap, batchSize int, singleConsumer bool, mk func(int) T) {
	q := NewSPSCQueue[T](cap)
	runProducersConsumersSPSC(b, batchSize, singleConsumer, mk,
		func(q *SPSCQueue[T], v T) {
			for {
				if q.Enqueue(v) {
					return
				}
				if q.IsClosed() {
					return
				}
				runtime.Gosched()
			}
		},
		func(q *SPSCQueue[T], buf []T) int {
			n, _ := q.DequeueBatch(buf)
			return n
		},
		func(q *SPSCQueue[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		q,
	)
}

func benchSPSCUnbounded[T any](b *testing.B, batchSize int, singleConsumer bool, mk func(int) T) {
	q := NewUnboundedSPSC[T]()
	runProducersConsumersSPSC(b, batchSize, singleConsumer, mk,
		func(q *UnboundedSPSC[T], v T) {
			q.Enqueue(v)
		},
		func(q *UnboundedSPSC[T], buf []T) int {
			return q.DequeueBatch(buf)
		},
		func(q *UnboundedSPSC[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		q,
	)
}

func benchQueue[T any](b *testing.B, cap, batchSize, producers int, singleConsumer, resize bool, policy matrixEnqueuePolicy, bytesPerItem int64, mk func(int) T) {
	var q *Queue[T]
	if resize {
		q = NewQueue[T](cap, 0, FullPolicyResize)
	} else {
		q = NewQueue[T](cap, cap, FullPolicyDrop)
	}
	runProducersConsumers(b, q, batchSize, producers, singleConsumer, policy, bytesPerItem,
		func(q *Queue[T], v T) bool {
			return q.Enqueue(v)
		},
		func(q *Queue[T], buf []T) int {
			slice, _ := q.DequeueBatch(buf)
			return len(slice)
		},
		func(q *Queue[T]) (int, bool) {
			var one [1]T
			slice, _ := q.DequeueBatch(one[:])
			if len(slice) > 0 {
				return 1, true
			}
			return 0, false
		},
		mk,
	)
}

func benchSmartDouble[T any](b *testing.B, cap, batchSize, producers int, mk func(int) T) {
	q := NewSmartDoubleQueue[T](cap, cap, false)
	popAndCount := func() (int, bool) {
		batch, ok := q.Pop()
		if !ok || len(batch) == 0 {
			return 0, false
		}
		n := len(batch)
		q.ReleaseBatch()
		return n, true
	}
	runProducersConsumersSmartDouble(b, batchSize, producers, mk,
		func(v T) {
			for !q.Enqueue(v) {
				runtime.Gosched()
			}
		},
		popAndCount,
	)
}

func benchPriority[T any](b *testing.B, cap, batchSize, producers int, singleConsumer bool, bytesPerItem int64, mk func(int) T, prio func(T) int) {
	q := NewPriorityQueue[T](cap)
	runProducersConsumers(b, q, batchSize, producers, singleConsumer, matrixEnqueueBlock, bytesPerItem,
		func(q *PriorityQueue[T], v T) bool {
			q.Enqueue(v, prio(v))
			return true
		},
		nil,
		func(q *PriorityQueue[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		mk,
	)
}

func benchChanBuffered[T any](b *testing.B, bufSlots, batchSize, producers int, singleConsumer bool, mk func(int) T) {
	ch := make(chan T, bufSlots)
	runProducersConsumersChan(b, ch, bufSlots, batchSize, producers, singleConsumer, false, mk)
}

func benchChanUnbuffered[T any](b *testing.B, batchSize, producers int, singleConsumer bool, mk func(int) T) {
	ch := make(chan T)
	runProducersConsumersChan(b, ch, 0, batchSize, producers, singleConsumer, true, mk)
}

func runProducersConsumers[Q any, T any](b *testing.B, q Q, batchSize, producers int, singleConsumer bool, policy matrixEnqueuePolicy, bytesPerItem int64,
	enqueue func(Q, T) bool,
	dequeueBatch func(Q, []T) int,
	dequeueOne func(Q) (int, bool),
	mk func(int) T,
) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed, enqueued int64
	var producersDone atomic.Bool

	consumerDone := func() bool {
		if policy == matrixEnqueueDrop {
			return producersDone.Load() && atomic.LoadInt64(&consumed) >= atomic.LoadInt64(&enqueued)
		}
		return atomic.LoadInt64(&consumed) >= int64(total)
	}

	if singleConsumer {
		go func() {
			for !consumerDone() {
				n, ok := dequeueOne(q)
				if ok && n > 0 {
					atomic.AddInt64(&consumed, int64(n))
				} else if !ok {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	} else {
		buf := make([]T, batchSize)
		go func() {
			for !consumerDone() {
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
			if policy == matrixEnqueueDrop {
				for i := start; i < end; i++ {
					if enqueue(q, mk(i)) {
						atomic.AddInt64(&enqueued, 1)
					}
				}
				return
			}
			for i := start; i < end; i++ {
				for !enqueue(q, mk(i)) {
					runtime.Gosched()
				}
			}
		}(p)
	}
	wg.Wait()
	producersDone.Store(true)
	for !consumerDone() {
		runtime.Gosched()
	}
	<-done

	if policy == matrixEnqueueDrop {
		b.SetBytes(atomic.LoadInt64(&consumed) * bytesPerItem)
	}
}

func runProducersConsumersSPSC[Q any, T any](b *testing.B, batchSize int, singleConsumer bool, mk func(int) T,
	enqueue func(Q, T),
	dequeueBatch func(Q, []T) int,
	dequeueOne func(Q) (int, bool),
	q Q,
) {
	total := b.N
	done := make(chan struct{})
	var consumed int64

	if singleConsumer {
		go func() {
			for atomic.LoadInt64(&consumed) < int64(total) {
				n, ok := dequeueOne(q)
				if ok && n > 0 {
					atomic.AddInt64(&consumed, int64(n))
				} else if !ok {
					runtime.Gosched()
				}
			}
			close(done)
		}()
	} else {
		buf := make([]T, batchSize)
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
	for i := 0; i < total; i++ {
		enqueue(q, mk(i))
	}
	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done
}

func runProducersConsumersSmartDouble[T any](b *testing.B, batchSize, producers int, mk func(int) T,
	enqueue func(T),
	dequeue func() (int, bool),
) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed int64

	go func() {
		for atomic.LoadInt64(&consumed) < int64(total) {
			n, ok := dequeue()
			if ok && n > 0 {
				atomic.AddInt64(&consumed, int64(n))
			} else if !ok {
				runtime.Gosched()
			}
		}
		close(done)
	}()

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
				enqueue(mk(i))
			}
		}(p)
	}
	wg.Wait()
	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done
}

func runProducersConsumersChan[T any](b *testing.B, ch chan T, cap, batchSize, producers int, singleConsumer bool, unbuffered bool, mk func(int) T) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	done := make(chan struct{})
	var consumed int64

	if unbuffered {
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
			buf := make([]T, batchSize)
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
				ch <- mk(i)
			}
		}(p)
	}
	wg.Wait()
	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done

	if cap > 0 {
		for len(ch) > 0 {
			<-ch
		}
	}
}
