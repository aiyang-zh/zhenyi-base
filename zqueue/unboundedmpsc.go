package zqueue

import (
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"sync/atomic"
	"unsafe"
)

// ================================================================
// 泛型节点定义 - 避免装箱
// ================================================================

// mpscNode 泛型链表节点（直接存储 T 类型，避免 interface{} 装箱）
type mpscNode[T any] struct {
	next unsafe.Pointer // *mpscNode[T]
	val  T              // 直接存储值，不用 any
}

// UnboundedMPSC 是无界、多生产者、单消费者、无锁队列
// 使用哨兵节点 + 双端对象池，性能高、GC 压力低
//
// 安全性约束：
// - Enqueue: 可多线程并发调用（wait-free）
// - Dequeue / DequeueBatch / Empty: 仅限单消费者调用
type UnboundedMPSC[T any] struct {
	// head 指向最新插入的节点（生产者原子交换）
	head unsafe.Pointer // *mpscNode[T]
	_    [cacheLineSize]byte

	// tail 指向当前哨兵节点（消费者读取）
	tail unsafe.Pointer // *mpscNode[T]
	_    [cacheLineSize]byte

	// 生产者端对象池（多线程安全，用于 Enqueue）
	nodePool *zpool.Pool[*mpscNode[T]]
	_        [cacheLineSize]byte

	// 消费者端本地缓存（单线程，用于 Dequeue 归还）
	recycleCache []*mpscNode[T]
	recycleCap   int
}

// NewUnboundedMPSC 创建新队列（使用双端对象池）
func NewUnboundedMPSC[T any]() *UnboundedMPSC[T] {
	q := &UnboundedMPSC[T]{
		recycleCap: 256,
		nodePool: zpool.NewPool(func() *mpscNode[T] {
			return &mpscNode[T]{}
		}),
	}
	q.recycleCache = make([]*mpscNode[T], 0, q.recycleCap)

	sentinel := &mpscNode[T]{} // 直接创建泛型节点
	sentinel.next = nil
	q.head = unsafe.Pointer(sentinel)
	q.tail = unsafe.Pointer(sentinel)

	return q
}

// Enqueue 添加元素到队列（wait-free，多生产者安全）
func (q *UnboundedMPSC[T]) Enqueue(v T) {
	n := q.nodePool.Get() // 从对象池获取节点
	n.val = v
	atomic.StorePointer(&n.next, nil)

	prev := (*mpscNode[T])(atomic.SwapPointer(&q.head, unsafe.Pointer(n)))
	atomic.StorePointer(&prev.next, unsafe.Pointer(n))
}

// EnqueueBatch 批量入队 (Wait-Free, 多生产者安全)
func (q *UnboundedMPSC[T]) EnqueueBatch(elements []T) {
	if len(elements) == 0 {
		return
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

	prevHead := (*mpscNode[T])(atomic.SwapPointer(&q.head, unsafe.Pointer(last)))
	atomic.StorePointer(&prevHead.next, unsafe.Pointer(first))
}

// Dequeue 出队一个元素（非阻塞，单消费者）
func (q *UnboundedMPSC[T]) Dequeue() (T, bool) {
	tail := (*mpscNode[T])(atomic.LoadPointer(&q.tail))
	next := (*mpscNode[T])(atomic.LoadPointer(&tail.next))
	if next == nil {
		var zero T
		return zero, false
	}

	v := next.val // 直接读取，不需要类型断言
	var zero T
	next.val = zero // 清零

	atomic.StorePointer(&q.tail, unsafe.Pointer(next))

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

// DequeueBatch 批量出队（单消费者）
func (q *UnboundedMPSC[T]) DequeueBatch(buffer []T) int {
	if len(buffer) == 0 {
		return 0
	}

	var zero T
	count := 0
	limit := len(buffer)
	current := (*mpscNode[T])(atomic.LoadPointer(&q.tail))

	for count < limit {
		next := (*mpscNode[T])(atomic.LoadPointer(&current.next))
		if next == nil {
			break
		}

		buffer[count] = next.val // 直接读取
		count++
		next.val = zero // 清零

		oldSentinel := current
		atomic.StorePointer(&q.tail, unsafe.Pointer(next))

		// 归还到本地缓存
		oldSentinel.next = nil
		oldSentinel.val = zero
		if len(q.recycleCache) < q.recycleCap {
			q.recycleCache = append(q.recycleCache, oldSentinel)
		} else {
			// 缓存满了，批量归还到 sync.Pool
			for _, node := range q.recycleCache {
				q.nodePool.Put(node)
			}
			q.recycleCache = q.recycleCache[:0]
			q.recycleCache = append(q.recycleCache, oldSentinel)
		}

		current = next
	}

	return count
}

// Empty 检查队列是否为空
func (q *UnboundedMPSC[T]) Empty() bool {
	tail := (*mpscNode[T])(atomic.LoadPointer(&q.tail))
	return atomic.LoadPointer(&tail.next) == nil
}

// Shrink 缩容：清空消费者本地缓存，让 GC 回收 sync.Pool 中多余的对象
// 适合在空闲时定期调用，降低长期运行后的内存驻留
func (q *UnboundedMPSC[T]) Shrink() {
	for _, node := range q.recycleCache {
		if node != nil {
			q.nodePool.Put(node)
		}
	}
	q.recycleCache = q.recycleCache[:0]
}

// Close 关闭队列并清理本地缓存
func (q *UnboundedMPSC[T]) Close() {
	for _, node := range q.recycleCache {
		if node != nil {
			q.nodePool.Put(node)
		}
	}
	q.recycleCache = nil
}
