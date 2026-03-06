package zqueue

import (
	"sync"
)

// 1. 去掉 index 字段，因为如果不出于更新优先级的需求，它是多余的开销
// 2. 直接存储值结构体，不再存指针
type queueItem[T any] struct {
	value    T
	priority int
}

type PriorityQueue[T any] struct {
	items []queueItem[T] // 连续内存，缓存更友好
	lock  sync.Mutex
}

func NewPriorityQueue[T any](capacity int) *PriorityQueue[T] {
	return &PriorityQueue[T]{
		items: make([]queueItem[T], 0, capacity), // 预分配容量，减少切片扩容开销
	}
}

func (q *PriorityQueue[T]) Enqueue(v T, priority int) {
	q.lock.Lock()

	// 1. Append 到末尾
	q.items = append(q.items, queueItem[T]{value: v, priority: priority})

	// 2. 上浮 (Up) 逻辑手动实现
	q.up(len(q.items) - 1)

	q.lock.Unlock()
}

func (q *PriorityQueue[T]) Dequeue() (T, bool) {
	q.lock.Lock()
	n := len(q.items)
	if n == 0 {
		q.lock.Unlock()
		var zero T
		return zero, false
	}

	// 1. 获取堆顶
	res := q.items[0].value

	// 2. 将最后一个元素移到堆顶
	q.items[0] = q.items[n-1]
	var zero queueItem[T]
	q.items[n-1] = zero

	q.items = q.items[:n-1]

	// 3. 下沉 (Down) 逻辑手动实现
	q.down(0, n-1)

	q.lock.Unlock()
	return res, true
}

// ----------------------
// 手写堆算法 (内联以避免 interface 开销)
// ----------------------

func (q *PriorityQueue[T]) up(j int) {
	for {
		i := (j - 1) / 2 // parent index
		if i == j || q.items[j].priority <= q.items[i].priority {
			// 如果是最大堆（优先级大在前），这里改成 <=
			// 如果是最小堆（优先级小在前），这里改成 >=
			// 目前逻辑：Child <= Parent，说明不用动了（这是最大堆逻辑：父节点必须大）
			// 修正：我们要实现 Priority 大的在前面 (Max Heap)
			// Parent 必须 >= Child。如果 Child > Parent，则交换。
			if q.items[j].priority <= q.items[i].priority {
				break
			}
		}
		// 交换
		q.items[i], q.items[j] = q.items[j], q.items[i]
		j = i
	}
}

func (q *PriorityQueue[T]) down(i0, n int) {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && q.items[j2].priority > q.items[j1].priority {
			j = j2 // right child is larger
		}
		if q.items[j].priority <= q.items[i].priority {
			break
		}
		q.items[i], q.items[j] = q.items[j], q.items[i]
		i = j
	}
}

func (q *PriorityQueue[T]) Len() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.items)
}
