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

func (q *SmartDoubleQueue[T]) Signal() <-chan struct{} {
	return q.signal
}

func (q *SmartDoubleQueue[T]) Close() {
	if atomic.CompareAndSwapInt32(&q.closed, 0, 1) {
		close(q.signal)
	}
}
func (q *SmartDoubleQueue[T]) Len() int64 {
	return atomic.LoadInt64(&q.count)
}
