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
//   - 使用切片 + 位运算实现环形缓冲区，内存布局连续，缓存友好
//   - 支持自动扩容或丢弃新元素两种策略（见 FullPolicy）
//   - 使用互斥锁保护，适合中等并发场景
type Queue[T any] struct {
	items      []T        // 使用切片替代链表
	head       int        // 队列头部指针
	tail       int        // 队列尾部指针
	mask       int        // 队列容量
	count      int        // 原子计数器
	lock       sync.Mutex // 优化为互斥锁
	fullPolicy FullPolicy // 队列满时的策略
	maxSize    int        // 最大容量（0表示无限制）
}

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 2
	} // 默认最小容量 2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

// NewQueue 创建一个带策略控制的有界队列。
//
// initialSize 为初始容量，maxSize 为最大容量（0 表示不限制），
// policy 决定写满时的行为（自动扩容或丢弃）。
func NewQueue[T any](initialSize int, maxSize int, policy FullPolicy) *Queue[T] {
	capacity := nextPowerOfTwo(initialSize)
	// 如果设置了上限，且初始容量已经比上限大，这通常是配置错误，但也兼容处理
	if maxSize > 0 && capacity > nextPowerOfTwo(maxSize) {
		maxSize = capacity
	}

	return &Queue[T]{
		mask:       capacity - 1,
		items:      make([]T, capacity),
		fullPolicy: policy,
		maxSize:    maxSize,
	}
}

// GetDefaultQueue 创建一个默认策略（可扩容）的队列。
// 等价于 NewQueue(size, 0, FullPolicyResize)。
func GetDefaultQueue[T any](size int) *Queue[T] {
	capacity := nextPowerOfTwo(size)
	q := &Queue[T]{
		mask:       capacity - 1,
		items:      make([]T, capacity), // 环形缓冲区需额外空间
		fullPolicy: FullPolicyResize,    // 默认自动扩容
	}
	return q
}

// Count 返回当前队列中的元素个数。
// 内部加锁获取一致性快照。
func (q *Queue[T]) Count() int {
	q.lock.Lock() // 必须加锁才能获取准确快照
	c := q.count
	q.lock.Unlock()
	return c
}

// Enqueue 入队一个元素。
// 返回 true 表示成功，false 表示队列已满且策略不允许扩容。
func (q *Queue[T]) Enqueue(item T) bool {
	q.lock.Lock()
	// 计算下一个位置
	next := (q.tail + 1) & q.mask
	// 队列满时的处理
	if next == q.head {
		switch q.fullPolicy {
		case FullPolicyResize:
			if q.maxSize > 0 && len(q.items) >= q.maxSize {
				q.lock.Unlock()
				return false // 队列真满了，且不能再扩了 -> 返回 false 触发外部重试
			}
			q.resize()
			next = (q.tail + 1) & q.mask
		case FullPolicyDrop:
			q.lock.Unlock()
			return false // 队列满，丢弃新元素
		}
	}

	q.items[q.tail] = item
	q.tail = next
	q.count++ // 普通 int 操作
	q.lock.Unlock()
	return true
}

// EnqueueBatch 批量入队（原子语义：要么整批成功，要么整批失败）。
// 返回 true 表示整批入队成功，false 表示空间不足且策略不允许扩容。
func (q *Queue[T]) EnqueueBatch(items []T) bool {
	if len(items) == 0 {
		return true
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	requiredSpace := len(items)
	currentCap := len(q.items)
	availableSpace := currentCap - q.count

	// 注意：环形队列实际可用空间通常认为是 capacity - 1 (为了区分空和满)，
	// 但我们这里用 count 计数，实际上 resize 也是基于 count 的。
	// 当 count == len(items) 时，就需要扩容了。

	// 1. 判断是否需要扩容
	// 如果剩余空间不足以放下这批数据
	if availableSpace < requiredSpace {
		if q.fullPolicy == FullPolicyDrop {
			return false // 空间不够，策略是丢弃，直接失败
		}

		// 尝试计算需要扩容到多大
		targetSize := currentCap
		// 循环翻倍直到能装下或者超过 MaxSize
		for (targetSize - q.count) < requiredSpace {
			targetSize *= 2
			// 如果超过了最大限制，且最大限制被设置了
			if q.maxSize > 0 && targetSize > nextPowerOfTwo(q.maxSize) {
				return false // 即使扩容到最大也装不下这批数据
			}
		}

		// 执行具体的扩容操作
		// 注意：上面的循环只是模拟计算，resize() 是单纯翻倍，所以可能需要多次 resize
		// 或者我们改写 resize 支持一步到位，为了简单安全，这里循环调用 resize
		for len(q.items) < targetSize {
			q.resize()
		}
	}

	// 2. 此时空间一定足够，开始批量写入
	// 因为是环形队列，可能需要分两段写入

	// 第一段：从 tail 到数组末尾
	firstChunkLen := len(q.items) - q.tail
	if firstChunkLen >= requiredSpace {
		// 情况A：直接追加在后面，不需要绕回头部
		copy(q.items[q.tail:], items)
		q.tail = (q.tail + requiredSpace) & q.mask
	} else {
		// 情况B：需要绕回头部
		// 1. 先填满尾部
		copy(q.items[q.tail:], items[:firstChunkLen])
		// 2. 再填头部
		secondChunkLen := requiredSpace - firstChunkLen
		copy(q.items[0:], items[firstChunkLen:])
		q.tail = secondChunkLen & q.mask
	}

	q.count += requiredSpace
	return true
}

// Front 返回队首元素但不出队。
// 第二个返回值为 false 表示队列为空。
func (q *Queue[T]) Front() (T, bool) {
	q.lock.Lock()
	if q.count == 0 {
		q.lock.Unlock()
		return *new(T), false
	}
	item := q.items[q.head]
	q.lock.Unlock()
	return item, true
}

func (q *Queue[T]) resize() {
	newSize := len(q.items) * 2
	newItems := make([]T, newSize)

	curr := q.head
	for i := 0; i < q.count; i++ {
		newItems[i] = q.items[curr]
		curr = (curr + 1) & q.mask
	}

	q.items = newItems
	q.head = 0
	q.tail = q.count
	q.mask = newSize - 1
}

// DequeueBatch 批量出队，结果写入 buf 中。
// 返回实际写入的切片视图以及出队后的剩余元素个数。
// buf 的容量决定单次最多出队多少元素。
func (q *Queue[T]) DequeueBatch(buf []T) ([]T, int) {
	q.lock.Lock()
	defer q.lock.Unlock() // 🔴 只加一次锁

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

	buf = buf[:0] // 重置长度
	for i := 0; i < limit; i++ {
		data := q.items[q.head]
		var zero T
		q.items[q.head] = zero // 防止内存泄漏
		q.head = (q.head + 1) & q.mask
		buf = append(buf, data)
	}
	q.count -= limit
	return buf, q.count
}
