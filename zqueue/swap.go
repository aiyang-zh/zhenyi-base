package zqueue

import (
	"sync"
	"sync/atomic"
)

// SmartDoubleQueue 是基于“双缓冲”结构的通用队列。
//
// 特点：
//   - 写入始终追加到 write 缓冲，读取时一次性交换到 read 缓冲批量消费
//   - 可选通知通道 Signal，适合 select 驱动的异步消费模型
//   - 支持最大容量上限与动态缩容，适合高并发、突刺流量场景
//
// maxCap 为硬上限（0 表示不限制）：统计 write/read 缓冲与尚未 ReleaseBatch 的 Pop 批次
// 元素总数；Pop 后须在处理完 batch 后调用 ReleaseBatch 释放配额，否则 Enqueue 将持续受占满。
type SmartDoubleQueue[T any] struct {
	mu          sync.Mutex
	write       []T
	read        []T
	signal      chan struct{}
	closed      int32
	initCap     int
	maxCap      int
	count       int64
	outstanding int
	isNotify    bool
}

// NewSmartDoubleQueue 创建一个双缓冲队列。
//
// initCap 为初始容量；maxCap 为元素总数硬上限（0 表示不限制，见 SmartDoubleQueue 说明）；
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
// 返回 true 表示成功，false 表示队列已关闭或超过 maxCap 硬上限。
//
// closed 仅通过 atomic 读写（Close 使用 CompareAndSwapInt32）；锁外第一次 Load 为快速路径，
// 锁内再次 Load 与 Close 并发时仍合法——持锁时直读 q.closed 字段会与 CAS 构成 data race，故不用字段直读。
func (q *SmartDoubleQueue[T]) Enqueue(items ...T) bool {
	if len(items) == 0 {
		return true
	}
	if atomic.LoadInt32(&q.closed) == 1 {
		return false
	}
	q.mu.Lock()
	if atomic.LoadInt32(&q.closed) == 1 {
		q.mu.Unlock()
		return false
	}

	if q.maxCap > 0 {
		pending := len(q.write) + len(q.read) + q.outstanding + len(items)
		if pending > q.maxCap {
			q.mu.Unlock()
			return false
		}
	}

	q.write = append(q.write, items...)
	shouldNotify := q.isNotify && atomic.LoadInt32(&q.closed) == 0
	q.mu.Unlock()
	atomic.AddInt64(&q.count, int64(len(items)))
	if shouldNotify && atomic.LoadInt32(&q.closed) == 0 {
		select {
		case q.signal <- struct{}{}:
		default:
		}
	}
	return true
}

// Pop 交换读写缓冲并返回当前可读的全部元素切片。
//
// 返回的切片在下次 Pop 之前一直有效；调用方可在处理完本批数据前再次 Pop（将得到空批次）。
// 当 maxCap > 0 时，Pop 取得的 batch 在 ReleaseBatch 之前计入容量，处理完毕后须调用 ReleaseBatch。
//
// 返回值：
//   - (slice, true): 本次有数据或暂时无数据但队列未完全关闭
//   - (nil, false): 队列已关闭且无任何未读数据
func (q *SmartDoubleQueue[T]) Pop() ([]T, bool) {
	q.mu.Lock()
	if len(q.write) == 0 {
		closed := atomic.LoadInt32(&q.closed) == 1
		q.mu.Unlock()
		if closed {
			return nil, false
		}
		return nil, true
	}
	if q.maxCap > 0 && cap(q.read) > q.initCap*32 && len(q.read) == 0 {
		q.read = make([]T, 0, q.initCap)
	}
	q.read, q.write = q.write, q.read
	batch := q.read
	q.read = q.read[:0]
	if cap(q.write) > q.initCap*32 {
		q.write = make([]T, 0, q.initCap)
	} else {
		q.write = q.write[:0]
	}
	batchLen := len(batch)
	if q.maxCap > 0 {
		q.outstanding += batchLen
	}
	q.mu.Unlock()
	if q.maxCap == 0 && batchLen > 0 {
		atomic.AddInt64(&q.count, -int64(batchLen))
	}
	return batch, true
}

// ReleaseBatch 释放当前所有尚未归还的 Pop 批次所占用的 maxCap 配额。
// 应在处理完 Pop 返回的切片后调用；重复调用安全（outstanding 已为 0 时无操作）。
// 连续多次 Pop 后也可在一次处理完毕后统一调用，会一次性释放全部 outstanding。
// maxCap 为 0 时调用无害，并同步调低 Len 估计值。
func (q *SmartDoubleQueue[T]) ReleaseBatch() {
	q.mu.Lock()
	n := q.outstanding
	q.outstanding = 0
	q.mu.Unlock()
	if n > 0 {
		atomic.AddInt64(&q.count, -int64(n))
	}
}

// Signal 返回用于通知消费方有新数据写入的只读通道。
// 仅在 isNotify=true 时有效。
func (q *SmartDoubleQueue[T]) Signal() <-chan struct{} {
	return q.signal
}

// Close 关闭队列；不向已注册的 Signal 接收方关闭通道（避免与 Enqueue 通知竞态 panic）。
// 关闭后 Enqueue 将返回 false，Pop 在数据消费完后返回 (nil, false)。
// 关闭后仍应照常 ReleaseBatch，以释放 maxCap 配额并校正 Len。
// 若启用了通知，会非阻塞地向 Signal 发送一次唤醒，便于消费方退出 select。
func (q *SmartDoubleQueue[T]) Close() {
	if !atomic.CompareAndSwapInt32(&q.closed, 0, 1) {
		return
	}
	if q.isNotify {
		select {
		case q.signal <- struct{}{}:
		default:
		}
	}
}

// Len 返回元素总数估计值，语义随 maxCap 模式而异：
//   - maxCap > 0：内部缓冲 + outstanding（与 Enqueue 容量检查一致，Pop 后须 ReleaseBatch 才下降）
//   - maxCap == 0：近似内部缓冲（Pop 时即扣减，不含调用方手中 batch）
//
// 适合作为监控指标使用。
func (q *SmartDoubleQueue[T]) Len() int64 {
	return atomic.LoadInt64(&q.count)
}
