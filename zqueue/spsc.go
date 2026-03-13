package zqueue

import (
	"github.com/aiyang-zh/zhenyi-base/zbackoff"
	"sync/atomic"
	"time"
)

const (
	maxCapacity = 1 << 30
)

// SPSCQueue 是一个有界、无锁、单生产者单消费者（SPSC）的环形队列。
//
// 特点：
//   - Producer/Consumer 字段严格分离，避免伪共享
//   - 使用自旋 + 退避策略实现阻塞写/读，适合高频场景
//   - 支持阻塞 / Try* 两种 API，满足不同延迟/丢包需求
type SPSCQueue[T any] struct {
	// --- 生产者独占字段 ---
	head      atomic.Uint64
	_padding0 [cacheLineSize]byte

	tailCache uint64
	_padding1 [cacheLineSize]byte

	// --- 消费者独占字段 ---
	tail      atomic.Uint64
	_padding2 [cacheLineSize]byte

	headCache uint64
	_padding3 [cacheLineSize]byte

	// --- 共享状态字段 ---
	closed    atomic.Bool
	_padding4 [cacheLineSize]byte

	// --- 只读常量字段 ---
	mask   uint64
	buffer []T
}

// NewSPSCQueue 创建一个有界 SPSC 队列。
// capacity 会自动向上调整为 [2, maxCapacity] 范围内的 2 的幂。
func NewSPSCQueue[T any](capacity int) *SPSCQueue[T] {
	if capacity < 2 {
		capacity = 2
	}
	if capacity > maxCapacity {
		capacity = maxCapacity
	}
	if !isPowerOfTwo(capacity) {
		capacity = nextPowerOfTwo(capacity)
	}

	return &SPSCQueue[T]{
		buffer: make([]T, capacity),
		mask:   uint64(capacity - 1),
	}
}

// Close 关闭队列，后续写入立即失败。
// 已在队列中的数据仍可被读取。
func (q *SPSCQueue[T]) Close() {
	q.closed.Store(true)
}

// IsClosed 返回队列是否已被关闭。
func (q *SPSCQueue[T]) IsClosed() bool {
	return q.closed.Load()
}

// ==========================================
// Producer Methods
// ==========================================

// EnqueueBatch 阻塞批量写入。
// 队列空间不足时会自旋等待，直到全部写入或队列被关闭。
func (q *SPSCQueue[T]) EnqueueBatch(items []T) bool {
	if q.closed.Load() {
		return false
	}
	if len(items) == 0 {
		return true
	}

	count := uint64(len(items))
	head := q.head.Load()
	capacity := q.mask + 1
	targetHead := head + count
	tailLimit := q.tailCache + capacity

	if targetHead > tailLimit {
		realTail := q.tail.Load()
		if targetHead > realTail+capacity {
			spinCount := 0
			for {
				if q.closed.Load() {
					return false
				}
				realTail = q.tail.Load()
				if targetHead <= realTail+capacity {
					break
				}
				zbackoff.Backoff(spinCount, 10, 30, 10*time.Microsecond)
				spinCount++
			}
		}
		q.tailCache = realTail
	}

	offset := head & q.mask
	toEnd := capacity - offset

	if count <= toEnd {
		copy(q.buffer[offset:offset+count], items)
	} else {
		copy(q.buffer[offset:], items[:toEnd])
		copy(q.buffer[0:], items[toEnd:])
	}

	q.head.Store(targetHead)
	return true
}

// TryEnqueueBatch 非阻塞批量写入。
// 在空间不足时立即返回 false，不做阻塞等待。
func (q *SPSCQueue[T]) TryEnqueueBatch(items []T) bool {
	if q.closed.Load() {
		return false
	}
	if len(items) == 0 {
		return true
	}

	count := uint64(len(items))
	head := q.head.Load()
	capacity := q.mask + 1
	targetHead := head + count
	tailLimit := q.tailCache + capacity

	if targetHead > tailLimit {
		realTail := q.tail.Load()
		// 如果空间不足，直接返回 false
		if targetHead > realTail+capacity {
			return false
		}
		q.tailCache = realTail
	}

	offset := head & q.mask
	toEnd := capacity - offset

	if count <= toEnd {
		copy(q.buffer[offset:offset+count], items)
	} else {
		copy(q.buffer[offset:], items[:toEnd])
		copy(q.buffer[0:], items[toEnd:])
	}

	q.head.Store(targetHead)
	return true
}

// Enqueue 阻塞单个写入。
// 如果队列已关闭则返回 false。
func (q *SPSCQueue[T]) Enqueue(item T) bool {
	if q.closed.Load() {
		return false
	}

	head := q.head.Load()
	capacity := q.mask + 1
	tailLimit := q.tailCache + capacity

	if head >= tailLimit {
		realTail := q.tail.Load()
		if head >= realTail+capacity {
			spinCount := 0
			for {
				if q.closed.Load() {
					return false
				}
				realTail = q.tail.Load()
				if head < realTail+capacity {
					break
				}
				zbackoff.Backoff(spinCount, 10, 30, 10*time.Microsecond)
				spinCount++
			}
		}
		q.tailCache = realTail
	}

	q.buffer[head&q.mask] = item
	q.head.Store(head + 1)
	return true
}

// TryEnqueue 非阻塞单个写入。
// 队列空间不足时立即返回 false。
func (q *SPSCQueue[T]) TryEnqueue(item T) bool {
	if q.closed.Load() {
		return false
	}

	head := q.head.Load()
	capacity := q.mask + 1
	tailLimit := q.tailCache + capacity

	if head >= tailLimit {
		realTail := q.tail.Load()
		// 空间不足直接返回
		if head >= realTail+capacity {
			return false
		}
		q.tailCache = realTail
	}

	q.buffer[head&q.mask] = item
	q.head.Store(head + 1)
	return true
}

// ==========================================
// Consumer Methods
// ==========================================

// DequeueBatch 阻塞批量读取。
// 返回 (n, ok):
//   - n > 0: 成功读取 n 条数据
//   - n == 0, ok == true: 暂时无数据（仅在 limit=0 时出现）
//   - n == 0, ok == false: 队列已关闭且无剩余数据
func (q *SPSCQueue[T]) DequeueBatch(limit []T) (int, bool) {
	if len(limit) == 0 {
		return 0, true
	}

	tail := q.tail.Load()
	var available uint64

	if tail < q.headCache {
		available = q.headCache - tail
	} else {
		realHead := q.head.Load()
		if tail >= realHead {
			spinCount := 0
			for {
				realHead = q.head.Load()
				if tail < realHead {
					break
				}
				if q.closed.Load() {
					if q.head.Load() <= tail {
						return 0, false
					}
				}
				zbackoff.Backoff(spinCount, 10, 30, 10*time.Microsecond)
				spinCount++
			}
		}
		q.headCache = realHead
		available = realHead - tail
	}

	batchSize := min(available, uint64(len(limit)))
	return q.consumeBatch(tail, batchSize, limit), true
}

// TryDequeueBatch 非阻塞批量读取。
// 返回 (n, ok):
//   - n > 0: 成功读取 n 条
//   - n == 0, ok == true: 队列为空，未关闭
//   - n == 0, ok == false: 队列已关闭且空
func (q *SPSCQueue[T]) TryDequeueBatch(limit []T) (int, bool) {
	if len(limit) == 0 {
		return 0, true
	}

	tail := q.tail.Load()
	var available uint64

	if tail < q.headCache {
		available = q.headCache - tail
	} else {
		realHead := q.head.Load()
		if tail >= realHead {
			if q.closed.Load() {
				return 0, false
			}
			return 0, true
		}
		q.headCache = realHead
		available = realHead - tail
	}

	batchSize := min(available, uint64(len(limit)))
	return q.consumeBatch(tail, batchSize, limit), true
}

// Dequeue 单个阻塞读取。
// 当队列空且未关闭时会自旋等待；关闭且无剩余数据时返回 (零值, false)。
func (q *SPSCQueue[T]) Dequeue() (T, bool) {
	tail := q.tail.Load()
	var zero T

	if tail >= q.headCache {
		realHead := q.head.Load()
		if tail >= realHead {
			spinCount := 0
			for {
				realHead = q.head.Load()
				if tail < realHead {
					break
				}
				if q.closed.Load() {
					if q.head.Load() <= tail {
						return zero, false
					}
				}
				zbackoff.Backoff(spinCount, 10, 30, 10*time.Microsecond)
				spinCount++
			}
		}
		q.headCache = realHead
	}

	index := tail & q.mask
	val := q.buffer[index]
	q.buffer[index] = zero
	q.tail.Store(tail + 1)
	return val, true
}

// ==========================================
// Helper Methods
// ==========================================

func (q *SPSCQueue[T]) consumeBatch(tail, batchSize uint64, limit []T) int {
	if batchSize == 0 {
		return 0
	}

	offset := tail & q.mask
	capacity := q.mask + 1
	toEnd := capacity - offset
	var zero T

	// [优化] 使用更显式的 Slice 写法 (Point 1)
	if batchSize <= toEnd {
		copy(limit[:batchSize], q.buffer[offset:offset+batchSize])
		// GC Clear
		for i := uint64(0); i < batchSize; i++ {
			q.buffer[offset+i] = zero
		}
	} else {
		// 回绕
		copy(limit[:toEnd], q.buffer[offset:]) // Part 1
		remaining := batchSize - toEnd
		copy(limit[toEnd:batchSize], q.buffer[:remaining]) // Part 2

		// GC Clear
		for i := uint64(0); i < toEnd; i++ {
			q.buffer[offset+i] = zero
		}
		for i := uint64(0); i < remaining; i++ {
			q.buffer[i] = zero
		}
	}

	q.tail.Store(tail + batchSize)
	return int(batchSize)
}

// Len 返回当前队列中的元素数量（近似值）。
// 在单生产者/单消费者模型下等价于真实长度。
func (q *SPSCQueue[T]) Len() int {
	tail := q.tail.Load()
	head := q.head.Load()
	return int(head - tail)
}
func isPowerOfTwo(x int) bool {
	return (x & (x - 1)) == 0
}
