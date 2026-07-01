// Package zqueue 组合基准测试矩阵
//
// 维度：类型(11) × 载荷档位(3) × 生产者(4) × 入出 API(4)
//
// 队列类型（zqueue 全部实现 + channel 基线）：
//   - MPSCBounded / MPSCPadded / MPSCUnbounded
//   - SPSCBounded / SPSCUnbounded（仅 P1，SPSC 语义）
//   - QueueResize / QueueDrop（mutex 环形队列）
//   - SmartDouble / Priority
//   - ChanBuffered / ChanUnbuffered
//
// 载荷档位：Small(256B)、Medium(4KB)、Large(~64KB 值类型，65528B，受 Go channel 元素 <64KiB 约束)
// Large 有界队列槽位 capLargeMatrix(1024×64KB≈64MiB)，非历史 65536×64KB≈4GiB；跨版本/旧矩阵对比 Large 吞吐不可直接横比，仅适合同矩阵内队列相对比较
// Large 批量条数受 matrixMaxBatchBytes(1MiB) 限制（512 条×64KB 退化为 16 条）
// ChanBuffered Large 缓冲槽位 chanBufLarge(256)，约 16MiB
// 生产者：1、4、16、64
// 入出 API：SingleSingle / SingleBatch / BatchSingle / BatchBatch（见 matrixModeSupported Skip）
//
// 计时覆盖「并发入队 + 排空」全链路。SetBytes 为每消息字节数（MB/s = bytesPerItem/ns_per_msg）。
//
// 组合数：MPSC×3 + Queue×2 + SPSC×2 + SmartDouble + Priority + Chan×2 = 348（见 matrixModeSupported）
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
	// capLargeMatrix Large 档有界环形容量：1024×payloadLargeBytes≈64MiB（全矩阵串行跑时 1–2GiB 机器可承受）
	capLargeMatrix = 1024

	batchSmall  = 32
	batchMedium = 128
	batchLarge  = 512 // 条数上限；Large 档按 matrixMaxBatchBytes 与载荷折算（见 matrixBatchCount）

	matrixMaxBatchBytes = 1 << 20 // 单次批量入/出窗口字节上限 1MiB

	chanBufLarge = 256 // Large 档 channel 缓冲槽位（约 16MiB）

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

// matrixBatchCount 返回该载荷档位下单次批量 API 使用的条数（不超过 batchSizes[ds] 与 matrixMaxBatchBytes/bytesPerItem）。
func matrixBatchCount(ds int) int {
	n := batchSizes[ds]
	bytesPerItem := matrixPayloadBytes(ds)
	if bytesPerItem <= 0 {
		return n
	}
	maxByBytes := int(matrixMaxBatchBytes / bytesPerItem)
	if maxByBytes < 1 {
		maxByBytes = 1
	}
	if n > maxByBytes {
		return maxByBytes
	}
	return n
}

// matrixQueueCap 返回该档位有界队列/SmartDouble 等使用的槽位数。
func matrixQueueCap(ds int) int {
	switch ds {
	case 0:
		return capSmall
	case 1:
		return capMedium
	case 2:
		return capLargeMatrix
	default:
		return capSmall
	}
}

// chanBufferSlots 返回 ChanBuffered 缓冲槽位数。
func chanBufferSlots(ds int) int {
	if ds == 2 {
		return chanBufLarge
	}
	return matrixQueueCap(ds)
}

var (
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

// matrixIOMode 入队/出队 API 组合：Single=单条 API，Batch=批量 API。
type matrixIOMode int

const (
	matrixIOSingleSingle matrixIOMode = iota // Enqueue + Dequeue
	matrixIOSingleBatch                      // Enqueue + DequeueBatch
	matrixIOBatchSingle                      // EnqueueBatch + Dequeue
	matrixIOBatchBatch                       // EnqueueBatch + DequeueBatch
)

var matrixIOModes = []struct {
	mode matrixIOMode
	name string
}{
	{matrixIOSingleSingle, "SingleSingle"},
	{matrixIOSingleBatch, "SingleBatch"},
	{matrixIOBatchSingle, "BatchSingle"},
	{matrixIOBatchBatch, "BatchBatch"},
}

func matrixModeSingleProducer(mode matrixIOMode) bool {
	return mode == matrixIOSingleSingle || mode == matrixIOSingleBatch
}

func matrixModeSingleConsumer(mode matrixIOMode) bool {
	return mode == matrixIOSingleSingle || mode == matrixIOBatchSingle
}

func matrixModeSupported(typ string, mode matrixIOMode) (bool, string) {
	switch typ {
	case "SmartDouble":
		if mode == matrixIOSingleSingle || mode == matrixIOBatchSingle {
			return false, "SmartDouble 仅 Pop 批量 API，无单条出队"
		}
	case "Priority":
		if mode != matrixIOSingleSingle {
			return false, "PriorityQueue 仅单条入队/出队 API"
		}
	case "ChanBuffered", "ChanUnbuffered":
		if mode == matrixIOBatchSingle || mode == matrixIOBatchBatch {
			return false, "channel 无批量 send API"
		}
	}
	return true, ""
}

func BenchmarkMatrix(b *testing.B) {
	for _, typ := range matrixTypes {
		typ := typ
		b.Run(typ, func(b *testing.B) {
			for ds := 0; ds < 3; ds++ {
				capOrBatch := matrixQueueCap(ds)
				batchSize := matrixBatchCount(ds)
				dsName := dataSizeNames[ds]

				b.Run(dsName, func(b *testing.B) {
					for _, p := range producerCounts {
						b.Run(producerName(p), func(b *testing.B) {
							for _, ioMode := range matrixIOModes {
								ioMode := ioMode
								b.Run(ioMode.name, func(b *testing.B) {
									runMatrixBench(b, typ, ds, capOrBatch, batchSize, p, ioMode.mode)
								})
							}
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

func runMatrixBench(b *testing.B, typ string, ds, capOrBatch, batchSize, producers int, mode matrixIOMode) {
	if matrixTypeSPSCOnly(typ) && producers != 1 {
		b.Skip("SPSC 队列仅支持单生产者")
		return
	}
	if ok, reason := matrixModeSupported(typ, mode); !ok {
		b.Skip(reason)
		return
	}

	bytesPerItem := matrixPayloadBytes(ds)
	b.SetBytes(bytesPerItem)
	b.ReportAllocs()

	if typ == "ChanBuffered" || typ == "ChanUnbuffered" {
		runMatrixChan(b, typ, ds, batchSize, producers, mode)
		return
	}

	switch ds {
	case 0:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, mode, bytesPerItem, newMatrixItem256, priorityOf256)
	case 1:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, mode, bytesPerItem, newMatrixItem4K, priorityOf4K)
	case 2:
		dispatchMatrixSkipChan(b, typ, capOrBatch, batchSize, producers, mode, bytesPerItem, newMatrixItem64K, priorityOf64K)
	default:
		b.Fatalf("unknown data size index %d", ds)
	}
}

func priorityOf256(v matrixItem256) int { return int(v.Seq % 17) }
func priorityOf4K(v matrixItem4K) int   { return int(v.Seq % 17) }
func priorityOf64K(v matrixItem64K) int { return int(v.Seq % 17) }

func runMatrixChan(b *testing.B, typ string, ds, batchSize, producers int, mode matrixIOMode) {
	bufSlots := chanBufferSlots(ds)
	singleConsumer := matrixModeSingleConsumer(mode)
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

func dispatchMatrixSkipChan[T any](b *testing.B, typ string, capOrBatch, batchSize, producers int, mode matrixIOMode, bytesPerItem int64, mk func(int) T, prio func(T) int) {
	switch typ {
	case "MPSCBounded":
		benchMPSCBounded(b, capOrBatch, batchSize, producers, mode, false, bytesPerItem, mk)
	case "MPSCPadded":
		benchMPSCBounded(b, capOrBatch, batchSize, producers, mode, true, bytesPerItem, mk)
	case "MPSCUnbounded":
		benchMPSCUnbounded(b, batchSize, producers, mode, bytesPerItem, mk)
	case "SPSCBounded":
		benchSPSCBounded(b, capOrBatch, batchSize, mode, mk)
	case "SPSCUnbounded":
		benchSPSCUnbounded(b, batchSize, mode, mk)
	case "QueueResize":
		benchQueue(b, capOrBatch, batchSize, producers, mode, true, matrixEnqueueBlock, bytesPerItem, mk)
	case "QueueDrop":
		benchQueue(b, capOrBatch, batchSize, producers, mode, false, matrixEnqueueDrop, bytesPerItem, mk)
	case "SmartDouble":
		benchSmartDouble(b, capOrBatch, batchSize, producers, mode, mk)
	case "Priority":
		benchPriority(b, capOrBatch, batchSize, producers, mode, bytesPerItem, mk, prio)
	default:
		b.Fatalf("unknown matrix type %q", typ)
	}
}

func benchMPSCBounded[T any](b *testing.B, cap, batchSize, producers int, mode matrixIOMode, padded bool, bytesPerItem int64, mk func(int) T) {
	var q *MPSCQueue[T]
	if padded {
		q = NewMPSCQueuePadded[T](cap)
	} else {
		q = NewMPSCQueue[T](cap)
	}
	runProducersConsumers(b, q, batchSize, producers, mode, matrixEnqueueBlock, bytesPerItem,
		func(q *MPSCQueue[T], v T) bool {
			for !q.Enqueue(v) {
				runtime.Gosched()
			}
			return true
		},
		func(q *MPSCQueue[T], items []T) int {
			want := len(items)
			for len(items) > 0 {
				n := q.EnqueueBatch(items)
				if n == 0 {
					runtime.Gosched()
					continue
				}
				items = items[n:]
			}
			return want
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

func benchMPSCUnbounded[T any](b *testing.B, batchSize, producers int, mode matrixIOMode, bytesPerItem int64, mk func(int) T) {
	q := NewUnboundedMPSC[T]()
	runProducersConsumers(b, q, batchSize, producers, mode, matrixEnqueueBlock, bytesPerItem,
		func(q *UnboundedMPSC[T], v T) bool {
			q.Enqueue(v)
			return true
		},
		func(q *UnboundedMPSC[T], items []T) int {
			q.EnqueueBatch(items)
			return len(items)
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

func benchSPSCBounded[T any](b *testing.B, cap, batchSize int, mode matrixIOMode, mk func(int) T) {
	q := NewSPSCQueue[T](cap)
	runProducersConsumersSPSC(b, batchSize, mode, mk,
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
		func(q *SPSCQueue[T], items []T) {
			for {
				if q.EnqueueBatch(items) {
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

func benchSPSCUnbounded[T any](b *testing.B, batchSize int, mode matrixIOMode, mk func(int) T) {
	q := NewUnboundedSPSC[T]()
	runProducersConsumersSPSC(b, batchSize, mode, mk,
		func(q *UnboundedSPSC[T], v T) {
			q.Enqueue(v)
		},
		func(q *UnboundedSPSC[T], items []T) {
			q.EnqueueBatch(items)
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

func benchQueue[T any](b *testing.B, cap, batchSize, producers int, mode matrixIOMode, resize bool, policy matrixEnqueuePolicy, bytesPerItem int64, mk func(int) T) {
	var q *Queue[T]
	if resize {
		q = NewQueue[T](cap, 0, FullPolicyResize)
	} else {
		q = NewQueue[T](cap, cap, FullPolicyDrop)
	}
	runProducersConsumers(b, q, batchSize, producers, mode, policy, bytesPerItem,
		func(q *Queue[T], v T) bool {
			return q.Enqueue(v)
		},
		func(q *Queue[T], items []T) int {
			if q.EnqueueBatch(items) {
				return len(items)
			}
			return 0
		},
		func(q *Queue[T], buf []T) int {
			slice, _ := q.DequeueBatch(buf)
			return len(slice)
		},
		func(q *Queue[T]) (int, bool) {
			_, ok := q.Dequeue()
			if ok {
				return 1, true
			}
			return 0, false
		},
		mk,
	)
}

func benchSmartDouble[T any](b *testing.B, cap, batchSize, producers int, mode matrixIOMode, mk func(int) T) {
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
	if mode == matrixIOSingleBatch {
		runProducersConsumersSmartDouble(b, batchSize, producers, mk, popAndCount,
			func(v T) {
				for !q.Enqueue(v) {
					runtime.Gosched()
				}
			},
			nil,
		)
	} else {
		runProducersConsumersSmartDouble(b, batchSize, producers, mk, popAndCount,
			nil,
			func(items []T) {
				for !q.Enqueue(items...) {
					runtime.Gosched()
				}
			},
		)
	}
}

func benchPriority[T any](b *testing.B, cap, batchSize, producers int, mode matrixIOMode, bytesPerItem int64, mk func(int) T, prio func(T) int) {
	q := NewPriorityQueue[T](cap)
	runProducersConsumers(b, q, batchSize, producers, mode, matrixEnqueueBlock, bytesPerItem,
		func(q *PriorityQueue[T], v T) bool {
			q.Enqueue(v, prio(v))
			return true
		},
		nil,
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

func runProducersConsumers[Q any, T any](b *testing.B, q Q, batchSize, producers int, mode matrixIOMode, policy matrixEnqueuePolicy, bytesPerItem int64,
	enqueue func(Q, T) bool,
	enqueueBatch func(Q, []T) int,
	dequeueBatch func(Q, []T) int,
	dequeueOne func(Q) (int, bool),
	mk func(int) T,
) {
	total := b.N
	opsPerProducer := (total + producers - 1) / producers
	if opsPerProducer == 0 {
		opsPerProducer = 1
	}

	singleProducer := matrixModeSingleProducer(mode)
	singleConsumer := matrixModeSingleConsumer(mode)

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
			if singleProducer {
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
				return
			}

			batchBuf := make([]T, batchSize)
			for i := start; i < end; {
				chunk := batchSize
				if remain := end - i; remain < chunk {
					chunk = remain
				}
				for j := 0; j < chunk; j++ {
					batchBuf[j] = mk(i + j)
				}
				slice := batchBuf[:chunk]
				if policy == matrixEnqueueDrop {
					if enqueueBatch(q, slice) == chunk {
						atomic.AddInt64(&enqueued, int64(chunk))
					}
					i += chunk
					continue
				}
				for len(slice) > 0 {
					n := enqueueBatch(q, slice)
					if n == 0 {
						runtime.Gosched()
						continue
					}
					slice = slice[n:]
				}
				i += chunk
			}
		}(p)
	}
	wg.Wait()
	producersDone.Store(true)
	for !consumerDone() {
		runtime.Gosched()
	}
	<-done
}

func runProducersConsumersSPSC[Q any, T any](b *testing.B, batchSize int, mode matrixIOMode, mk func(int) T,
	enqueue func(Q, T),
	enqueueBatch func(Q, []T),
	dequeueBatch func(Q, []T) int,
	dequeueOne func(Q) (int, bool),
	q Q,
) {
	total := b.N
	singleProducer := matrixModeSingleProducer(mode)
	singleConsumer := matrixModeSingleConsumer(mode)

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
	if singleProducer {
		for i := 0; i < total; i++ {
			enqueue(q, mk(i))
		}
	} else {
		batchBuf := make([]T, batchSize)
		for i := 0; i < total; {
			chunk := batchSize
			if remain := total - i; remain < chunk {
				chunk = remain
			}
			for j := 0; j < chunk; j++ {
				batchBuf[j] = mk(i + j)
			}
			enqueueBatch(q, batchBuf[:chunk])
			i += chunk
		}
	}
	for atomic.LoadInt64(&consumed) < int64(total) {
		runtime.Gosched()
	}
	<-done
}

func runProducersConsumersSmartDouble[T any](b *testing.B, batchSize, producers int, mk func(int) T,
	dequeue func() (int, bool),
	enqueueOne func(T),
	enqueueMany func([]T),
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
			if enqueueOne != nil {
				for i := start; i < end; i++ {
					enqueueOne(mk(i))
				}
				return
			}
			batchBuf := make([]T, batchSize)
			for i := start; i < end; {
				chunk := batchSize
				if remain := end - i; remain < chunk {
					chunk = remain
				}
				for j := 0; j < chunk; j++ {
					batchBuf[j] = mk(i + j)
				}
				enqueueMany(batchBuf[:chunk])
				i += chunk
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
