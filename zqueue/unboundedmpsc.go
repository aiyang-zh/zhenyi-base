package zqueue

import (
	"github.com/aiyang-zh/zhenyi-base/zpool"
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

// UnboundedMPSC 是无界、多生产者、单消费者、无锁队列。
//
// 使用哨兵节点 + 双端对象池，性能高、GC 压力低，适合作为 Actor mailbox、
// 日志队列等突发流量场景的基础数据结构。
//
// 安全性约束：
//   - Enqueue / EnqueueBatch: 可多线程并发调用（wait-free）
//   - Dequeue / DequeueBatch / Empty / Shrink / Close: 仅限单消费者调用
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

	// enqueueStopped：单消费者调用 StopEnqueue 后，TryEnqueue 在链入链表前二次检查并不再接受新元素。
	// 与 BaseChannel.Close 配合，收紧「停止入队」与链式入队之间的 TOCTOU（无 mutex）。
	enqueueStopped atomic.Bool
}

// NewUnboundedMPSC 创建新的无界 MPSC 队列。
// 内部使用双端对象池减少 GC 压力。
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

// StopEnqueue 禁止后续 TryEnqueue 成功（仅生产者侧；Dequeue 仍由单消费者调用）。
// 典型由 Channel.Close 在通知发送协程退出之前调用，与 TryEnqueue 内二次检查配合收紧 TOCTOU。
func (q *UnboundedMPSC[T]) StopEnqueue() {
	q.enqueueStopped.Store(true)
}

// TryEnqueue 尝试入队；若已 StopEnqueue 或在分配节点后再次观察到停止，则返回 false（调用方保留 v 的所有权）。
func (q *UnboundedMPSC[T]) TryEnqueue(v T) bool {
	if q.enqueueStopped.Load() {
		return false
	}
	n := q.nodePool.Get()
	n.val = v
	atomic.StorePointer(&n.next, nil)
	if q.enqueueStopped.Load() {
		var zero T
		n.val = zero
		q.nodePool.Put(n)
		return false
	}
	prev := (*mpscNode[T])(atomic.SwapPointer(&q.head, unsafe.Pointer(n)))
	atomic.StorePointer(&prev.next, unsafe.Pointer(n))
	return true
}

// Enqueue 添加元素到队列（wait-free，多生产者安全）。
// 若已 StopEnqueue，静默丢弃 v；需回收资源时请使用 TryEnqueue 并在 false 时自行处理。
func (q *UnboundedMPSC[T]) Enqueue(v T) {
	_ = q.TryEnqueue(v)
}

// EnqueueBatch 批量入队（wait-free，多生产者安全）。
func (q *UnboundedMPSC[T]) EnqueueBatch(elements []T) {
	if len(elements) == 0 || q.enqueueStopped.Load() {
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
	// 二次检查，如果已停止则回滚释放所有节点
	if q.enqueueStopped.Load() {
		var zero T
		node := first
		for node != nil {
			next := (*mpscNode[T])(node.next)
			node.val = zero
			q.nodePool.Put(node)
			node = next
		}
		return
	}
	prevHead := (*mpscNode[T])(atomic.SwapPointer(&q.head, unsafe.Pointer(last)))
	atomic.StorePointer(&prevHead.next, unsafe.Pointer(first))
}

// Dequeue 出队一个元素（非阻塞，单消费者）。
// 返回 (值, true) 或 (零值, false)。
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

// DequeueBatch 批量出队（单消费者）。
// buffer 的长度决定单次最多出队多少元素，返回实际出队数量。
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

// Empty 检查队列是否为空（仅消费者调用）。
func (q *UnboundedMPSC[T]) Empty() bool {
	tail := (*mpscNode[T])(atomic.LoadPointer(&q.tail))
	return atomic.LoadPointer(&tail.next) == nil
}

// Shrink 缩容：清空消费者本地缓存，让 GC 回收 sync.Pool 中多余的对象。
// 适合在空闲时定期调用，降低长期运行后的内存驻留。
func (q *UnboundedMPSC[T]) Shrink() {
	for _, node := range q.recycleCache {
		if node != nil {
			q.nodePool.Put(node)
		}
	}
	q.recycleCache = q.recycleCache[:0]
}

// Close 关闭队列并清理本地缓存。
// 关闭后不再使用队列，适合进程退出或 Actor 停止时调用。
func (q *UnboundedMPSC[T]) Close() {
	q.StopEnqueue()
	for _, node := range q.recycleCache {
		if node != nil {
			q.nodePool.Put(node)
		}
	}
	q.recycleCache = nil
}
