package zqueue

import (
	"sync"
)

// FullPolicy 定义有限队列在写满时的处理策略。
type FullPolicy int

const (
	// FullPolicyResize 队列满时自动扩容（默认行为）。
	FullPolicyResize FullPolicy = 0
	// FullPolicyDrop 队列满时丢弃新元素（不扩容），适合限内存场景。
	FullPolicyDrop FullPolicy = 1
)

// Queue 是基于环形数组实现的通用有界队列。
//
// 特点：
//   - 使用切片环形缓冲区，下标前进统一为 % len(items)，内存布局连续，缓存友好
//   - 支持自动扩容或丢弃新元素两种策略（见 FullPolicy）
//   - 使用互斥锁保护，适合中等并发场景
type Queue[T any] struct {
	items      []T
	head       int
	tail       int
	count      int // 当前元素个数（锁保护，非 atomic）
	lock       sync.Mutex
	fullPolicy FullPolicy
	maxSize    int // 环形容量上限（0 无限制）；存 nextPowerOfTwo 后的槽位数
	closed     bool
}

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 2
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}

func normalizeQueueMaxSize(capacity, maxSize int) int {
	if maxSize <= 0 {
		return 0
	}
	maxSize = nextPowerOfTwo(maxSize)
	// 初始环已大于配置上限时抬高上限，避免「刚创建即无法扩容」；调用方应保证 initialSize ≤ maxSize。
	if capacity > maxSize {
		maxSize = capacity
	}
	return maxSize
}

// NewQueue 创建一个带策略控制的有界队列。
//
// initialSize 为初始容量，maxSize 为环形容量上限（0 表示不限制；非 0 会规范为 ≥2 的 2 幂），
// policy 决定写满时的行为（自动扩容或丢弃）。
// 若 nextPowerOfTwo(initialSize) 已超过 maxSize，会将 maxSize 抬高至初始环长（兼容旧行为，见 TestNewQueue_MaxSizeLessThanInitial）。
func NewQueue[T any](initialSize int, maxSize int, policy FullPolicy) *Queue[T] {
	capacity := nextPowerOfTwo(initialSize)
	return &Queue[T]{
		items:      make([]T, capacity),
		fullPolicy: policy,
		maxSize:    normalizeQueueMaxSize(capacity, maxSize),
	}
}

// GetDefaultQueue 创建一个默认策略（可扩容）的队列。
// 等价于 NewQueue(size, 0, FullPolicyResize)。
func GetDefaultQueue[T any](size int) *Queue[T] {
	return NewQueue[T](size, 0, FullPolicyResize)
}

// Count 返回当前队列中的元素个数。
func (q *Queue[T]) Count() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.count
}

// Len 与 Count 相同，与同包其它队列 API 命名一致。
func (q *Queue[T]) Len() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.count
}

// Cap 返回当前环形缓冲区的槽位数。
// 在写满前可容纳的元素个数为 Cap()-1；FullPolicyResize 且无 maxSize 限制时环可扩容，瞬时上限随 Cap 增长。
func (q *Queue[T]) Cap() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.items)
}

// IsClosed 报告队列是否已关闭。
func (q *Queue[T]) IsClosed() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.closed
}

// Close 关闭队列；关闭后 Enqueue/TryEnqueue/EnqueueBatch 失败，已入队元素仍可 Dequeue。
func (q *Queue[T]) Close() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.closed = true
}

// Enqueue 入队一个元素。
// 返回 true 表示成功，false 表示队列已关闭、已满且策略不允许扩容。
func (q *Queue[T]) Enqueue(item T) bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.enqueueLocked(item)
}

// TryEnqueue 非阻塞入队：无法获取锁、队列已关闭、或队列满且无法扩容时返回 false。
func (q *Queue[T]) TryEnqueue(item T) bool {
	if !q.lock.TryLock() {
		return false
	}
	defer q.lock.Unlock()
	return q.enqueueLocked(item)
}

func (q *Queue[T]) enqueueLocked(item T) bool {
	if q.closed {
		return false
	}
	n := len(q.items)
	next := (q.tail + 1) % n
	if next == q.head {
		switch q.fullPolicy {
		case FullPolicyResize:
			if q.maxSize > 0 && n >= q.maxSize {
				return false
			}
			q.resize()
			n = len(q.items)
			next = (q.tail + 1) % n
		case FullPolicyDrop:
			return false
		}
	}

	q.items[q.tail] = item
	q.tail = next
	q.count++
	return true
}

// EnqueueBatch 批量入队（原子语义：要么整批成功，要么整批失败）。
// 空切片恒返回 true（含队列已关闭时的 no-op）；非空切片在已关闭或空间不足且无法扩容时返回 false。
func (q *Queue[T]) EnqueueBatch(items []T) bool {
	if len(items) == 0 {
		return true
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	if q.closed {
		return false
	}

	requiredSpace := len(items)
	currentCap := len(q.items)
	maxCount := currentCap - 1
	availableSpace := maxCount - q.count

	if availableSpace < requiredSpace {
		if q.fullPolicy == FullPolicyDrop {
			return false
		}

		targetSize := currentCap
		for (targetSize - 1 - q.count) < requiredSpace {
			targetSize *= 2
			if q.maxSize > 0 && targetSize > q.maxSize {
				return false
			}
		}
		q.resizeTo(targetSize)
	}

	firstChunkLen := len(q.items) - q.tail
	n := len(q.items)
	if firstChunkLen >= requiredSpace {
		copy(q.items[q.tail:], items)
		q.tail = (q.tail + requiredSpace) % n
	} else {
		copy(q.items[q.tail:], items[:firstChunkLen])
		secondChunkLen := requiredSpace - firstChunkLen
		copy(q.items[0:], items[firstChunkLen:])
		q.tail = secondChunkLen % n
	}

	q.count += requiredSpace
	return true
}

// Front 返回队首元素但不出队。
// 第二个返回值为 false 表示队列为空。
func (q *Queue[T]) Front() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.count == 0 {
		return *new(T), false
	}
	return q.items[q.head], true
}

// Dequeue 出队单个元素。
func (q *Queue[T]) Dequeue() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.count == 0 {
		return *new(T), false
	}
	item := q.items[q.head]
	clear(q.items[q.head : q.head+1])
	q.head = (q.head + 1) % len(q.items)
	q.count--
	return item, true
}

func (q *Queue[T]) resize() {
	q.resizeTo(len(q.items) * 2)
}

func (q *Queue[T]) resizeTo(targetSize int) {
	if targetSize <= len(q.items) {
		return
	}
	oldLen := len(q.items)
	newItems := make([]T, targetSize)

	curr := q.head
	for i := 0; i < q.count; i++ {
		newItems[i] = q.items[curr]
		curr = (curr + 1) % oldLen
	}

	q.items = newItems
	q.head = 0
	q.tail = q.count
}

// DequeueBatch 批量出队，结果写入 buf 中。
// 返回实际写入的切片视图以及出队后的剩余元素个数。
// buf 的容量决定单次最多出队多少元素。
func (q *Queue[T]) DequeueBatch(buf []T) ([]T, int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.count == 0 {
		return buf[:0], 0
	}
	limit := cap(buf)
	if limit == 0 {
		return buf[:0], q.count
	}
	if limit > q.count {
		limit = q.count
	}

	startHead := q.head
	nitems := len(q.items)
	first := nitems - startHead
	if first > limit {
		first = limit
	}

	buf = buf[:0]
	if first > 0 {
		buf = append(buf, q.items[startHead:startHead+first]...)
		clear(q.items[startHead : startHead+first])
	}
	remain := limit - first
	if remain > 0 {
		buf = append(buf, q.items[:remain]...)
		clear(q.items[:remain])
	}
	q.head = (startHead + limit) % nitems
	q.count -= limit
	return buf, q.count
}
