// 压测对比：PriorityQueue（priority.go 手写 up/down）vs container/heap（标准库 Fix 上浮/下沉）。
// 结果见 benchmark_results/priority_bench.md 与 priority_bench_*.txt。
package zqueue

import (
	"container/heap"
	"testing"
)

// --- container/heap：标准库 heap.Fix（非手写 up/down）---

type benchStdItem struct {
	value    int
	priority int
}

type benchStdHeap []benchStdItem

func (h benchStdHeap) Len() int { return len(h) }

func (h benchStdHeap) Less(i, j int) bool { return h[i].priority > h[j].priority }

func (h benchStdHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *benchStdHeap) Push(x any) {
	*h = append(*h, x.(benchStdItem))
}

func (h *benchStdHeap) Pop() any {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

func newBenchStdHeap(capacity int) *benchStdHeap {
	h := make(benchStdHeap, 0, capacity)
	return &h
}

func (h *benchStdHeap) enqueue(v, priority int) {
	heap.Push(h, benchStdItem{value: v, priority: priority})
}

func (h *benchStdHeap) dequeue() (int, bool) {
	if h.Len() == 0 {
		return 0, false
	}
	it := heap.Pop(h).(benchStdItem)
	return it.value, true
}

func benchmarkPriorityFillSize(b *testing.B, size int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := NewPriorityQueue[int](size)
		for j := 0; j < size; j++ {
			q.Enqueue(j, j%17)
		}
		for q.Len() > 0 {
			_, _ = q.Dequeue()
		}
	}
}

func benchmarkStdFillSize(b *testing.B, size int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := newBenchStdHeap(size)
		for j := 0; j < size; j++ {
			h.enqueue(j, j%17)
		}
		for h.Len() > 0 {
			_, _ = h.dequeue()
		}
	}
}

// BenchmarkPriorityQueue_EnqueueDequeue priority.go 手写 up/down（含 Mutex）。
func BenchmarkPriorityQueue_EnqueueDequeue(b *testing.B) {
	benchmarkPriorityFillSize(b, 256)
}

// BenchmarkStdHeap_EnqueueDequeue container/heap Fix（非手写 up/down）。
func BenchmarkStdHeap_EnqueueDequeue(b *testing.B) {
	benchmarkStdFillSize(b, 256)
}

func BenchmarkPriorityQueue_EnqueueDequeue_Size(b *testing.B) {
	for _, size := range []int{64, 256, 1024, 4096} {
		b.Run(intSizeName(size), func(b *testing.B) {
			benchmarkPriorityFillSize(b, size)
		})
	}
}

func BenchmarkStdHeap_EnqueueDequeue_Size(b *testing.B) {
	for _, size := range []int{64, 256, 1024, 4096} {
		b.Run(intSizeName(size), func(b *testing.B) {
			benchmarkStdFillSize(b, size)
		})
	}
}

func benchmarkPriorityEnqueueOnly(b *testing.B, size int) {
	q := NewPriorityQueue[int](size)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i, i%31)
		if q.Len() >= size {
			for q.Len() > 0 {
				_, _ = q.Dequeue()
			}
		}
	}
}

func benchmarkStdEnqueueOnly(b *testing.B, size int) {
	h := newBenchStdHeap(size)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.enqueue(i, i%31)
		if h.Len() >= size {
			for h.Len() > 0 {
				_, _ = h.dequeue()
			}
		}
	}
}

func BenchmarkPriorityQueue_EnqueueOnly(b *testing.B) {
	benchmarkPriorityEnqueueOnly(b, 1024)
}

func BenchmarkStdHeap_EnqueueOnly(b *testing.B) {
	benchmarkStdEnqueueOnly(b, 1024)
}

func benchmarkPriorityDequeueOnly(b *testing.B, size int) {
	q := NewPriorityQueue[int](size)
	for i := 0; i < size; i++ {
		q.Enqueue(i, i%31)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if q.Len() == 0 {
			for j := 0; j < size; j++ {
				q.Enqueue(j, j%31)
			}
		}
		_, _ = q.Dequeue()
	}
}

func benchmarkStdDequeueOnly(b *testing.B, size int) {
	h := newBenchStdHeap(size)
	for i := 0; i < size; i++ {
		h.enqueue(i, i%31)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if h.Len() == 0 {
			for j := 0; j < size; j++ {
				h.enqueue(j, j%31)
			}
		}
		_, _ = h.dequeue()
	}
}

func BenchmarkPriorityQueue_DequeueOnly(b *testing.B) {
	benchmarkPriorityDequeueOnly(b, 1024)
}

func BenchmarkStdHeap_DequeueOnly(b *testing.B) {
	benchmarkStdDequeueOnly(b, 1024)
}

func intSizeName(n int) string {
	switch n {
	case 64:
		return "64"
	case 256:
		return "256"
	case 1024:
		return "1K"
	case 4096:
		return "4K"
	default:
		return "other"
	}
}
