package zqueue

import (
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"sync/atomic"
	"unsafe"
)

// ================================================================
// 泛型节点定义 - 避免装箱
// ================================================================

// spscNode 泛型链表节点（直接存储 T 类型，避免 interface{} 装箱）
type spscNode[T any] struct {
	next unsafe.Pointer // *spscNode[T]
	val  T              // 直接存储值，不用 any
}

// UnboundedSPSC 是无界、单生产者、单消费者、无锁队列
//
// 性能特征：
// - 极致性能：无任何原子 RMW/CAS，只有 Store/Load
// - 缓存友好：生产者/消费者字段严格隔离
// - 泛型节点池：避免装箱，每个队列独立池
//
// 使用约束：
// - Enqueue: **仅限单生产者线程** 调用（非并发安全）
// - Dequeue/Empty: **仅限单消费者线程** 调用（非并发安全）
type UnboundedSPSC[T any] struct {
	// 生产者专用区
	head *spscNode[T]
	_    [cacheLineSize]byte

	// 消费者专用区
	tail *spscNode[T]
	_    [cacheLineSize]byte

	// 节点对象池（生产者和消费者共享）
	nodePool *zpool.Pool[*spscNode[T]]
	_        [cacheLineSize]byte

	// 消费者端本地缓存（仅消费者线程访问，批量归还到 sync.Pool）
	recycleCache []*spscNode[T]
	recycleCap   int
}

// NewUnboundedSPSC 创建 SPSC 队列（使用泛型节点池）
func NewUnboundedSPSC[T any]() *UnboundedSPSC[T] {
	q := &UnboundedSPSC[T]{
		recycleCap: 256,
		nodePool: zpool.NewPool(func() *spscNode[T] {
			return &spscNode[T]{}
		}),
	}

	q.recycleCache = make([]*spscNode[T], 0, q.recycleCap)

	sentinel := &spscNode[T]{} // 直接创建泛型节点
	sentinel.next = nil
	q.head = sentinel
	q.tail = sentinel

	return q
}

// Enqueue 入队（仅单生产者，wait-free）
func (q *UnboundedSPSC[T]) Enqueue(v T) {
	n := q.nodePool.Get() // 从对象池获取节点
	n.val = v
	n.next = nil

	atomic.StorePointer(&q.head.next, unsafe.Pointer(n))
	q.head = n
}

// EnqueueBatch 批量入队（仅单生产者，Wait-Free）
func (q *UnboundedSPSC[T]) EnqueueBatch(elements []T) int {
	if len(elements) == 0 {
		return 0
	}

	first := q.nodePool.Get() // 从对象池获取
	first.val = elements[0]

	current := first
	for i := 1; i < len(elements); i++ {
		n := q.nodePool.Get() // 从对象池获取
		n.val = elements[i]
		current.next = unsafe.Pointer(n)
		current = n
	}

	last := current
	last.next = nil

	atomic.StorePointer(&q.head.next, unsafe.Pointer(first))
	q.head = last

	return len(elements)
}

// Dequeue 出队（仅单消费者，wait-free）
func (q *UnboundedSPSC[T]) Dequeue() (T, bool) {
	tail := q.tail
	next := (*spscNode[T])(atomic.LoadPointer(&tail.next))

	if next != nil {
		v := next.val // 直接读取，不需要类型断言
		var zero T
		next.val = zero // 清零

		q.tail = next

		// 归还节点：先到本地缓存，批量归还到 sync.Pool
		tail.next = nil
		tail.val = zero
		if len(q.recycleCache) < q.recycleCap {
			q.recycleCache = append(q.recycleCache, tail)
		} else {
			// 缓存满了，批量归还到 sync.Pool
			for _, node := range q.recycleCache {
				q.nodePool.Put(node)
			}
			q.recycleCache = q.recycleCache[:0]
			q.recycleCache = append(q.recycleCache, tail)
		}

		return v, true
	}

	var zero T
	return zero, false
}

// DequeueBatch 批量出队（仅单消费者，Wait-Free）
func (q *UnboundedSPSC[T]) DequeueBatch(buffer []T) int {
	if len(buffer) == 0 {
		return 0
	}

	var zero T
	count := 0
	limit := len(buffer)

	tail := q.tail
	next := (*spscNode[T])(atomic.LoadPointer(&tail.next))

	for next != nil && count < limit {
		buffer[count] = next.val // 直接读取
		count++

		next.val = zero // 清零

		oldTail := tail
		tail = next
		oldTail.next = nil
		oldTail.val = zero

		if len(q.recycleCache) < q.recycleCap {
			q.recycleCache = append(q.recycleCache, oldTail)
		} else {
			// 缓存满了，批量归还到 sync.Pool
			for _, node := range q.recycleCache {
				q.nodePool.Put(node)
			}
			q.recycleCache = q.recycleCache[:0]
			q.recycleCache = append(q.recycleCache, oldTail)
		}

		next = (*spscNode[T])(atomic.LoadPointer(&tail.next))
	}

	q.tail = tail

	return count
}

// Empty 检查是否为空（仅消费者调用）
func (q *UnboundedSPSC[T]) Empty() bool {
	return atomic.LoadPointer(&q.tail.next) == nil
}

// Close 关闭队列并清理本地缓存
func (q *UnboundedSPSC[T]) Close() {
	// 归还本地缓存到 sync.Pool
	for _, node := range q.recycleCache {
		if node != nil {
			q.nodePool.Put(node)
		}
	}
	q.recycleCache = nil
}
