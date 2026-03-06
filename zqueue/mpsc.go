package zqueue

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

// cacheLineSize 用于防止伪共享 (False Sharing)。
// 现代 CPU 通常为 64 或 128 字节。
const cacheLineSize = 128

// slot 代表环形缓冲区中的一个槽位。
type slot[T any] struct {
	sequence atomic.Uint64
	data     T
}

// slotAccess 通过 base + stride 访问 slot 数组。
// 紧凑模式 stride = sizeof(slot[T])；
// padded 模式 stride 向上对齐到 cache line，通过 over-allocate 物理 slot 实现，
// 保证相邻逻辑 slot 的 sequence 位于不同 cache line，消除伪共享。
type slotAccess[T any] struct {
	seqOffset  uintptr
	dataOffset uintptr
	stride     uintptr
	base       unsafe.Pointer
}

func (s *slotAccess[T]) sequenceAt(i uint64) *atomic.Uint64 {
	return (*atomic.Uint64)(unsafe.Add(s.base, s.stride*uintptr(i)+s.seqOffset))
}

func (s *slotAccess[T]) dataAt(i uint64) *T {
	return (*T)(unsafe.Add(s.base, s.stride*uintptr(i)+s.dataOffset))
}

// MPSCQueue 是一个无锁、有界、多生产者单消费者 (MPSC) 的环形队列。
//
// 基于 Dmitry Vyukov 的 bounded MPMC queue 变体，针对单消费者场景优化。
// 生产者通过 CAS 竞争 head，消费者独占 tail（无竞争）。
//
// 性能优化：
//   - CAS 退避：CAS 失败后调用 runtime.Gosched()，减少 cache line bouncing
//   - 批量 CAS：EnqueueBatch 一次 CAS 占 N 个连续 slot，摊薄竞争开销
//   - Padded 布局：NewMPSCQueuePadded 使每个 slot 对齐到 cache line，消除伪共享
//
// 与 UnboundedMPSC 的区别：
//   - MPSCQueue: 有界、环形数组、零分配（适合固定容量、延迟敏感场景）
//   - UnboundedMPSC: 无界、链表+对象池（适合 Actor mailbox 等突发流量场景）
//
// 安全性约束：
//   - Enqueue / TryEnqueue / EnqueueBatch / TryEnqueueBatch: 多线程安全
//   - Dequeue / DequeueBatch: 仅限单消费者调用
type MPSCQueue[T any] struct {
	head atomic.Uint64
	_    [cacheLineSize]byte

	tail atomic.Uint64
	_    [cacheLineSize]byte

	mask   uint64
	closed atomic.Bool
	slots  slotAccess[T]
	buffer []slot[T] // GC 引用保持，防止 backing array 被回收
}

// NewMPSCQueue 创建一个有界 MPSC 队列（紧凑布局，节省内存）。
// capacity 会自动向上取整到最近的 2 的幂。
func NewMPSCQueue[T any](capacity int) *MPSCQueue[T] {
	capacity = nextPowerOfTwo(capacity)
	buf := make([]slot[T], capacity)
	q := &MPSCQueue[T]{
		mask:   uint64(capacity - 1),
		buffer: buf,
	}
	q.slots = newSlotAccess(buf, unsafe.Sizeof(buf[0]))
	for i := 0; i < capacity; i++ {
		q.slots.sequenceAt(uint64(i)).Store(uint64(i))
	}
	return q
}

// NewMPSCQueuePadded 创建一个有界 MPSC 队列（padded 布局，消除伪共享）。
// 通过 over-allocate 使每个逻辑 slot 占满整条 cache line，
// 保证相邻 slot 的 sequence 不在同一 cache line，消除 false sharing。
//
// 内存开销：capacity × max(cacheLineSize, sizeof(slot))
//
//	T = int:  1024 → 128 KB,  8192 → 1 MB
//	T = *Msg: 1024 → 128 KB,  8192 → 1 MB
func NewMPSCQueuePadded[T any](capacity int) *MPSCQueue[T] {
	capacity = nextPowerOfTwo(capacity)

	slotSize := unsafe.Sizeof(slot[T]{})
	// stride 向上取整到 cacheLineSize 的整数倍，确保相邻 slot 的
	// sequence 字段一定落在不同 cache line（消除 false sharing）
	stride := (slotSize + cacheLineSize - 1) / cacheLineSize * cacheLineSize
	slotsPerLogical := stride / slotSize
	if stride%slotSize != 0 {
		slotsPerLogical++
		stride = slotsPerLogical * slotSize
	}

	buf := make([]slot[T], uint64(capacity)*uint64(slotsPerLogical))
	q := &MPSCQueue[T]{
		mask:   uint64(capacity - 1),
		buffer: buf,
	}
	q.slots = newSlotAccess(buf, stride)
	for i := 0; i < capacity; i++ {
		q.slots.sequenceAt(uint64(i)).Store(uint64(i))
	}
	return q
}

func newSlotAccess[T any](buf []slot[T], stride uintptr) slotAccess[T] {
	return slotAccess[T]{
		base:       unsafe.Pointer(&buf[0]),
		seqOffset:  unsafe.Offsetof(buf[0].sequence),
		dataOffset: unsafe.Offsetof(buf[0].data),
		stride:     stride,
	}
}

// Enqueue 非阻塞入队（处理 CAS 竞争）。
// 多生产者竞争同一槽位时自动重试 CAS，队列满或已关闭时立即返回 false。
// CAS 失败时调用 runtime.Gosched() 退避，减少 cache line bouncing。
func (q *MPSCQueue[T]) Enqueue(item T) bool {
	for {
		if q.closed.Load() {
			return false
		}

		head := q.head.Load()
		index := head & q.mask
		seq := q.slots.sequenceAt(index).Load()

		dif := int64(seq) - int64(head)

		if dif == 0 {
			if q.head.CompareAndSwap(head, head+1) {
				*q.slots.dataAt(index) = item
				q.slots.sequenceAt(index).Store(head + 1)
				return true
			}
			runtime.Gosched()
			continue
		}

		if dif < 0 {
			if q.head.Load() == head {
				return false
			}
			continue
		}
	}
}

// EnqueueBatch 批量入队。
// 优先尝试一次 CAS 占 N 个连续 slot（摊薄竞争开销），失败时退化为逐个入队。
// 返回成功入队的数量，调用方可据此判断 items[n:] 未入队。
//
// 快速路径保证 items 在环形缓冲区中连续存放（一次 CAS 原子占位）。
// 退化到慢速路径（逐个 Enqueue）时，items 间可能插入其他生产者的数据，
// 但每条 item 都是原子可见的，消费者按 sequence 顺序出队不受影响。
func (q *MPSCQueue[T]) EnqueueBatch(items []T) int {
	n := uint64(len(items))
	if n == 0 {
		return 0
	}

	cap := q.mask + 1

	// 快速路径：一次 CAS 占 N 个 slot（仅当 n <= capacity 时有效）
	for attempt := 0; attempt < 3 && n <= cap; attempt++ {
		if q.closed.Load() {
			return 0
		}

		head := q.head.Load()
		lastIndex := (head + n - 1) & q.mask
		lastSeq := q.slots.sequenceAt(lastIndex).Load()

		if int64(lastSeq)-int64(head+n-1) < 0 {
			break
		}

		firstSeq := q.slots.sequenceAt(head & q.mask).Load()

		if int64(firstSeq)-int64(head) != 0 {
			runtime.Gosched()
			continue
		}

		if q.head.CompareAndSwap(head, head+n) {
			for i := uint64(0); i < n; i++ {
				idx := (head + i) & q.mask
				*q.slots.dataAt(idx) = items[i]
				q.slots.sequenceAt(idx).Store(head + i + 1)
			}
			return int(n)
		}
		runtime.Gosched()
	}

	// 慢速路径：逐个入队
	for i, item := range items {
		if !q.Enqueue(item) {
			return i
		}
	}
	return len(items)
}

// TryEnqueue 非阻塞入队（单次尝试）。
// 队列满或 CAS 竞争失败时立即返回 false，不重试。
func (q *MPSCQueue[T]) TryEnqueue(item T) bool {
	if q.closed.Load() {
		return false
	}

	head := q.head.Load()
	index := head & q.mask
	seq := q.slots.sequenceAt(index).Load()

	dif := int64(seq) - int64(head)
	if dif == 0 {
		if q.head.CompareAndSwap(head, head+1) {
			*q.slots.dataAt(index) = item
			q.slots.sequenceAt(index).Store(head + 1)
			return true
		}
	}
	return false
}

// TryEnqueueBatch 尝试批量入队（单次尝试，不重试 CAS）。
// 返回成功入队的数量。首次失败即停止，保持批内顺序。
func (q *MPSCQueue[T]) TryEnqueueBatch(items []T) int {
	count := 0
	for _, item := range items {
		if q.TryEnqueue(item) {
			count++
		} else {
			break
		}
	}
	return count
}

// -----------------------------------------------------------------------------
// 消费者方法 (Single Consumer)
// -----------------------------------------------------------------------------

// Dequeue 单个出队（非阻塞）。
// 返回 (数据, true) 或 (零值, false)。
//
// closed 标志只影响生产者（拒绝入队），不影响消费者读取。
// 消费者应持续调用 Dequeue/DequeueBatch 直到返回 false 以排空队列。
func (q *MPSCQueue[T]) Dequeue() (T, bool) {
	tail := q.tail.Load()
	index := tail & q.mask
	seq := q.slots.sequenceAt(index).Load()

	if int64(seq)-int64(tail) != 1 {
		var zero T
		return zero, false
	}

	val := q.consumeOne(index, tail)
	return val, true
}

// DequeueBatch 批量出队（非阻塞，两阶段提交）。
// result 切片长度决定最大批量，返回实际读取的数量。
//
// 两阶段提交设计：
//
//	Phase 1（读取）：遍历可读槽位，拷贝数据、清零，但不释放 sequence。
//	Phase 2（提交）：先推进全局 tail，再释放所有已读槽位的 sequence。
//
// 先推进 tail 再释放 sequence 的原因：
// 如果先释放 sequence，生产者可立刻复用槽位并推进 head，
// 但 tail 尚未更新，导致 Len() = head - tail 瞬间膨胀（可超过 Cap()）。
func (q *MPSCQueue[T]) DequeueBatch(result []T) int {
	limit := len(result)
	if limit == 0 {
		return 0
	}

	tail := q.tail.Load()
	count := 0

	// Phase 1: 读取数据，不释放槽位
	for count < limit {
		index := tail & q.mask
		seq := q.slots.sequenceAt(index).Load()

		if int64(seq)-int64(tail) != 1 {
			break
		}

		result[count] = *q.slots.dataAt(index)
		var zero T
		*q.slots.dataAt(index) = zero
		tail++
		count++
	}

	if count == 0 {
		return 0
	}

	// Phase 2: 先推进 tail，再释放所有槽位
	q.tail.Store(tail)

	baseTail := tail - uint64(count)
	for i := uint64(0); i < uint64(count); i++ {
		releaseTail := baseTail + i
		index := releaseTail & q.mask
		q.slots.sequenceAt(index).Store(releaseTail + q.mask + 1)
	}

	return count
}

// consumeOne 内部方法：读取数据、清理槽位、推进 tail、释放 sequence。
// 严格保持"先 tail 后 sequence"的顺序，与 DequeueBatch 一致，
// 确保 Len() 不会因 sequence 提前释放而瞬间膨胀。
func (q *MPSCQueue[T]) consumeOne(index uint64, tail uint64) T {
	val := *q.slots.dataAt(index)

	var zero T
	*q.slots.dataAt(index) = zero

	q.tail.Store(tail + 1)

	q.slots.sequenceAt(index).Store(tail + q.mask + 1)

	return val
}

// -----------------------------------------------------------------------------
// 辅助方法
// -----------------------------------------------------------------------------

// Len 返回队列中当前的估计元素数量。
// 先加载 tail 后加载 head，结果最接近真实值或略小（安全侧估计）。
func (q *MPSCQueue[T]) Len() uint64 {
	t := q.tail.Load()
	h := q.head.Load()

	if h < t {
		return 0
	}
	return h - t
}

// Cap 返回队列总容量。
func (q *MPSCQueue[T]) Cap() uint64 {
	return q.mask + 1
}

// IsClosed 检查队列是否已关闭。
func (q *MPSCQueue[T]) IsClosed() bool {
	return q.closed.Load()
}

// Close 关闭队列。关闭后 Enqueue 返回 false，已入队数据仍可 Dequeue。
func (q *MPSCQueue[T]) Close() {
	q.closed.Store(true)
}
