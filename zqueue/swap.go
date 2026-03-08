package zqueue

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrQueueFull     = errors.New("queue is full")
	ErrQueueDisposed = errors.New("queue is disposed")
)

// SmartDoubleQueue 是基于“双缓冲”结构的通用队列。
//
// 特点：
//   - 写入始终追加到 write 缓冲，读取时一次性交换到 read 缓冲批量消费
//   - 可选通知通道 Signal，适合 select 驱动的异步消费模型
//   - 支持最大容量上限与动态缩容，适合高并发、突刺流量场景
type SmartDoubleQueue[T any] struct {
	mu       sync.Mutex
	write    []T
	read     []T
	signal   chan struct{}
	closed   int32
	initCap  int
	maxCap   int
	count    int64
	isNotify bool
}

// NewSmartDoubleQueue 创建一个双缓冲队列。
//
// initCap 为初始容量，maxCap 为最大容量（0 表示不限制），
// isNotify 为 true 时在首次写入后向 Signal 通道发送一次通知。
func NewSmartDoubleQueue[T any](initCap, maxCap int, isNotify bool) *SmartDoubleQueue[T] {
	if initCap < 64 {
		initCap = 64
	}
	q := &SmartDoubleQueue[T]{
		write:    make([]T, 0, initCap),
		read:     make([]T, 0, initCap),
		initCap:  initCap,
		maxCap:   maxCap,
		isNotify: isNotify,
		signal:   make(chan struct{}, 1),
	}
	return q
}

// Enqueue 将一批元素追加到队列尾部。
// 返回 true 表示成功，false 表示队列已关闭或超过最大容量。
func (q *SmartDoubleQueue[T]) Enqueue(items ...T) bool {
	if len(items) == 0 {
		return true
	}
	if atomic.LoadInt32(&q.closed) == 1 {
		return false
	}
	q.mu.Lock()
	if q.closed == 1 { // 双重检查
		q.mu.Unlock()
		return false
	}

	if q.maxCap > 0 && len(q.write)+len(items) > q.maxCap {
		q.mu.Unlock()
		return false
	}

	q.write = append(q.write, items...)
	shouldNotify := q.isNotify && q.closed == 0
	q.mu.Unlock()
	atomic.AddInt64(&q.count, int64(len(items)))
	if shouldNotify {
		select {
		case q.signal <- struct{}{}:
		default:
		}
	}
	return true
}

// Pop 交换读写缓冲并返回当前可读的全部元素切片。
//
// 返回值：
//   - (slice, true): 本次有数据或暂时无数据但队列未完全关闭
//   - (nil, false): 队列已关闭且无任何未读数据
func (q *SmartDoubleQueue[T]) Pop() ([]T, bool) {
	// 快速检查：如果已关闭且写队列空，直接退出
	if atomic.LoadInt32(&q.closed) == 1 {
		q.mu.Lock()
		empty := len(q.write) == 0
		q.mu.Unlock()
		if empty {
			return nil, false
		}
	}
	if len(q.read) > 0 {
		clear(q.read)
		q.read = q.read[:0]
	}

	if q.maxCap > 0 && cap(q.read) > q.initCap*32 {
		q.read = make([]T, 0, q.initCap)
	}
	q.mu.Lock()
	if len(q.write) == 0 {
		q.mu.Unlock()
		return nil, true
	}
	q.read, q.write = q.write, q.read
	swappedCount := int64(len(q.read))
	q.mu.Unlock()
	atomic.AddInt64(&q.count, -swappedCount)
	return q.read, true
}

// Signal 返回用于通知消费方有新数据写入的只读通道。
// 仅在 isNotify=true 时有效。
func (q *SmartDoubleQueue[T]) Signal() <-chan struct{} {
	return q.signal
}

// Close 关闭队列并关闭通知通道。
// 关闭后 Enqueue 将返回 false，Pop 在数据消费完后返回 (nil, false)。
func (q *SmartDoubleQueue[T]) Close() {
	if atomic.CompareAndSwapInt32(&q.closed, 0, 1) {
		close(q.signal)
	}
}

// Len 返回队列中当前元素数量的估计值。
// 适合作为监控指标使用。
func (q *SmartDoubleQueue[T]) Len() int64 {
	return atomic.LoadInt64(&q.count)
}
