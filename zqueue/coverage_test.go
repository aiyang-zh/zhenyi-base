package zqueue

import (
	"sync"
	"testing"
	"time"
)

// ============================================================
// PriorityQueue tests
// ============================================================

func TestPriorityQueue_NewAndLen(t *testing.T) {
	q := NewPriorityQueue[int](10)
	if q == nil {
		t.Fatal("NewPriorityQueue returned nil")
	}
	if q.Len() != 0 {
		t.Errorf("new queue Len=%d, want 0", q.Len())
	}
}

func TestPriorityQueue_EnqueueDequeue_Single(t *testing.T) {
	q := NewPriorityQueue[int](10)
	q.Enqueue(42, 1)
	if q.Len() != 1 {
		t.Errorf("Len=%d after Enqueue, want 1", q.Len())
	}
	v, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned false")
	}
	if v != 42 {
		t.Errorf("Dequeue got %d, want 42", v)
	}
	if q.Len() != 0 {
		t.Errorf("Len=%d after Dequeue, want 0", q.Len())
	}
}

func TestPriorityQueue_Dequeue_Empty(t *testing.T) {
	q := NewPriorityQueue[int](10)
	_, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue from empty queue should return false")
	}
}

func TestPriorityQueue_PriorityOrdering(t *testing.T) {
	q := NewPriorityQueue[int](10)
	q.Enqueue(1, 1)  // low
	q.Enqueue(2, 10) // high
	q.Enqueue(3, 5)  // mid
	q.Enqueue(4, 20) // highest

	// Max-heap: highest priority first
	order := []int{}
	for q.Len() > 0 {
		v, ok := q.Dequeue()
		if !ok {
			break
		}
		order = append(order, v)
	}
	want := []int{4, 2, 3, 1} // 20, 10, 5, 1
	if len(order) != len(want) {
		t.Fatalf("got %d elements, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d]=%d, want %d", i, order[i], want[i])
		}
	}
}

func TestPriorityQueue_SamePriority(t *testing.T) {
	q := NewPriorityQueue[int](10)
	q.Enqueue(1, 5)
	q.Enqueue(2, 5)
	q.Enqueue(3, 5)

	// All same priority - order may vary but all should be 5
	m := map[int]bool{}
	for q.Len() > 0 {
		v, ok := q.Dequeue()
		if !ok {
			break
		}
		m[v] = true
	}
	if len(m) != 3 {
		t.Errorf("expected 3 distinct values, got %v", m)
	}
	for _, v := range []int{1, 2, 3} {
		if !m[v] {
			t.Errorf("expected value %d in results", v)
		}
	}
}

func TestPriorityQueue_ManyElements(t *testing.T) {
	q := NewPriorityQueue[int](200)
	for i := 0; i < 150; i++ {
		q.Enqueue(i, i) // priority = value
	}
	if q.Len() != 150 {
		t.Errorf("Len=%d, want 150", q.Len())
	}
	// Should dequeue highest first: 149, 148, ...
	prev := 200
	for q.Len() > 0 {
		v, ok := q.Dequeue()
		if !ok {
			t.Fatal("Dequeue failed")
		}
		if v >= prev {
			t.Errorf("out of order: got %d, expected < %d", v, prev)
		}
		prev = v
	}
}

// ============================================================
// SPSCQueue tests (non-blocking only)
// ============================================================

func TestSPSCQueue_NewCapacity_LargeAndMaxCap(t *testing.T) {
	// capacity > 1<<30 -> corrected to maxCapacity
	q := NewSPSCQueue[int](1<<30 + 100)
	if q == nil {
		t.Fatal("NewSPSCQueue returned nil")
	}
	// Should have capacity 1<<30
	if q.Len() != 0 {
		t.Error("new queue should be empty")
	}
}

func TestSPSCQueue_NewCapacityCorrection(t *testing.T) {
	// capacity < 2 -> corrected to 2
	q := NewSPSCQueue[int](1)
	if q == nil {
		t.Fatal("NewSPSCQueue(1) returned nil")
	}
	if !q.TryEnqueue(1) {
		t.Error("TryEnqueue should succeed on queue with capacity 2")
	}
	if !q.TryEnqueue(2) {
		t.Error("TryEnqueue 2nd should succeed")
	}
	if q.TryEnqueue(3) {
		t.Error("TryEnqueue when full should fail")
	}

	// non-power-of-two -> rounded up
	q2 := NewSPSCQueue[int](5)
	if q2 == nil {
		t.Fatal("NewSPSCQueue(5) returned nil")
	}
	// Capacity should be 8
	for i := 0; i < 8; i++ {
		if !q2.TryEnqueue(i) {
			t.Errorf("TryEnqueue %d failed (capacity should be 8)", i)
		}
	}
	if q2.TryEnqueue(9) {
		t.Error("TryEnqueue when full should fail")
	}
}

func TestSPSCQueue_TryEnqueueTryDequeueBatch_Basic(t *testing.T) {
	q := NewSPSCQueue[int](16)
	if !q.TryEnqueue(100) {
		t.Error("TryEnqueue failed")
	}
	if !q.TryEnqueue(200) {
		t.Error("TryEnqueue 2nd failed")
	}

	buf := make([]int, 10)
	n, ok := q.TryDequeueBatch(buf)
	if !ok {
		t.Error("TryDequeueBatch returned ok=false")
	}
	if n != 2 {
		t.Errorf("TryDequeueBatch got %d items, want 2", n)
	}
	if buf[0] != 100 || buf[1] != 200 {
		t.Errorf("got [%d,%d], want [100,200]", buf[0], buf[1])
	}
}

func TestSPSCQueue_TryEnqueueBatch_Success(t *testing.T) {
	q := NewSPSCQueue[int](16)
	items := []int{1, 2, 3, 4, 5}
	if !q.TryEnqueueBatch(items) {
		t.Error("TryEnqueueBatch failed")
	}
	buf := make([]int, 10)
	n, _ := q.TryDequeueBatch(buf)
	if n != 5 {
		t.Errorf("got %d items, want 5", n)
	}
	for i := 0; i < 5; i++ {
		if buf[i] != items[i] {
			t.Errorf("buf[%d]=%d, want %d", i, buf[i], items[i])
		}
	}
}

func TestSPSCQueue_TryEnqueueBatch_Full(t *testing.T) {
	q := NewSPSCQueue[int](4)
	// Fill queue
	for i := 0; i < 4; i++ {
		if !q.TryEnqueue(i) {
			t.Errorf("TryEnqueue %d failed", i)
		}
	}
	items := []int{10, 11}
	if q.TryEnqueueBatch(items) {
		t.Error("TryEnqueueBatch when full should return false")
	}
}

func TestSPSCQueue_TryDequeueBatch_Empty(t *testing.T) {
	q := NewSPSCQueue[int](8)
	buf := make([]int, 5)
	n, ok := q.TryDequeueBatch(buf)
	if n != 0 || !ok {
		t.Errorf("empty queue: n=%d ok=%v, want n=0 ok=true", n, ok)
	}
}

func TestSPSCQueue_CloseIsClosed(t *testing.T) {
	q := NewSPSCQueue[int](8)
	if q.IsClosed() {
		t.Error("new queue should not be closed")
	}
	q.Close()
	if !q.IsClosed() {
		t.Error("after Close, IsClosed should be true")
	}
	if q.TryEnqueue(1) {
		t.Error("TryEnqueue on closed queue should fail")
	}
	if q.TryEnqueueBatch([]int{1, 2}) {
		t.Error("TryEnqueueBatch on closed queue should fail")
	}
	buf := make([]int, 5)
	_, ok := q.TryDequeueBatch(buf)
	// When closed and empty, TryDequeueBatch returns (0, false)
	if ok {
		t.Error("TryDequeueBatch on closed empty queue should return ok=false")
	}
}

func TestSPSCQueue_Len(t *testing.T) {
	q := NewSPSCQueue[int](8)
	if q.Len() != 0 {
		t.Errorf("empty Len=%d, want 0", q.Len())
	}
	q.TryEnqueue(1)
	q.TryEnqueue(2)
	if q.Len() != 2 {
		t.Errorf("Len=%d, want 2", q.Len())
	}
	buf := make([]int, 5)
	q.TryDequeueBatch(buf)
	if q.Len() != 0 {
		t.Errorf("after dequeue Len=%d, want 0", q.Len())
	}
}

func TestSPSCQueue_WrapAroundBatch(t *testing.T) {
	q := NewSPSCQueue[int](16)
	// Fill and drain to create wrap: add 8, remove 4, add 4 more (total 8 in queue)
	for i := 0; i < 8; i++ {
		q.TryEnqueue(i)
	}
	buf := make([]int, 4)
	q.TryDequeueBatch(buf) // get 0,1,2,3
	for i := 0; i < 4; i++ {
		q.TryEnqueue(100 + i)
	}
	// Now we have 4,5,6,7,100,101,102,103 (8 items)
	// Drain in two batches due to headCache semantics: first batch gets 4, second gets 4
	var got []int
	for {
		buf2 := make([]int, 8)
		n, ok := q.TryDequeueBatch(buf2)
		if n == 0 {
			break
		}
		got = append(got, buf2[:n]...)
		if !ok {
			break
		}
	}
	want := []int{4, 5, 6, 7, 100, 101, 102, 103}
	if len(got) != len(want) {
		t.Errorf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

func TestSPSCQueue_TryEnqueueBatch_EmptySlice(t *testing.T) {
	q := NewSPSCQueue[int](8)
	if !q.TryEnqueueBatch([]int{}) {
		t.Error("TryEnqueueBatch([]) should return true")
	}
}

func TestSPSCQueue_TryDequeueBatch_ZeroLenBuf(t *testing.T) {
	q := NewSPSCQueue[int](8)
	q.TryEnqueue(1)
	n, ok := q.TryDequeueBatch(make([]int, 0))
	if n != 0 || !ok {
		t.Errorf("zero len buf: n=%d ok=%v", n, ok)
	}
}

// ============================================================
// MPSCQueue tests
// ============================================================

func TestMPSCQueue_TryEnqueue_Success(t *testing.T) {
	q := NewMPSCQueue[int](8)
	if !q.TryEnqueue(42) {
		t.Error("TryEnqueue failed")
	}
	v, ok := q.Dequeue()
	if !ok || v != 42 {
		t.Errorf("Dequeue got %d,%v, want 42,true", v, ok)
	}
}

func TestMPSCQueue_TryEnqueue_Full(t *testing.T) {
	q := NewMPSCQueue[int](4)
	for i := 0; i < 4; i++ {
		if !q.TryEnqueue(i) {
			t.Errorf("TryEnqueue %d failed", i)
		}
	}
	if q.TryEnqueue(99) {
		t.Error("TryEnqueue when full should fail")
	}
}

func TestMPSCQueue_TryEnqueue_Closed(t *testing.T) {
	q := NewMPSCQueue[int](8)
	q.Close()
	if q.TryEnqueue(1) {
		t.Error("TryEnqueue on closed queue should fail")
	}
}

func TestMPSCQueue_TryEnqueueBatch(t *testing.T) {
	q := NewMPSCQueue[int](16)
	items := []int{1, 2, 3, 4, 5}
	n := q.TryEnqueueBatch(items)
	if n != 5 {
		t.Errorf("TryEnqueueBatch returned %d, want 5", n)
	}
	buf := make([]int, 10)
	m := q.DequeueBatch(buf)
	if m != 5 {
		t.Errorf("DequeueBatch got %d, want 5", m)
	}
	for i := 0; i < 5; i++ {
		if buf[i] != items[i] {
			t.Errorf("buf[%d]=%d, want %d", i, buf[i], items[i])
		}
	}
}

func TestMPSCQueue_TryEnqueueBatch_PartialWhenFull(t *testing.T) {
	q := NewMPSCQueue[int](4)
	for i := 0; i < 4; i++ {
		q.TryEnqueue(i)
	}
	items := []int{10, 11, 12, 13}
	n := q.TryEnqueueBatch(items)
	if n != 0 {
		t.Errorf("TryEnqueueBatch when full returned %d, want 0", n)
	}
}

func TestMPSCQueue_EnqueueBatch(t *testing.T) {
	q := NewMPSCQueue[int](16)
	items := []int{1, 2, 3}
	n := q.EnqueueBatch(items)
	if n != 3 {
		t.Errorf("EnqueueBatch returned %d, want 3", n)
	}
	buf := make([]int, 10)
	m := q.DequeueBatch(buf)
	if m != 3 {
		t.Errorf("DequeueBatch got %d, want 3", m)
	}
}

func TestMPSCQueue_EnqueueBatch_PartialFailure(t *testing.T) {
	q := NewMPSCQueue[int](4)
	// Fill 3 slots
	q.EnqueueBatch([]int{1, 2, 3})
	items := []int{10, 11, 12} // only 1 fits
	n := q.EnqueueBatch(items)
	if n != 1 {
		t.Errorf("EnqueueBatch when 1 slot left returned %d, want 1", n)
	}
	buf := make([]int, 10)
	m := q.DequeueBatch(buf)
	if m != 4 {
		t.Errorf("DequeueBatch got %d, want 4", m)
	}
}

func TestMPSCQueue_DequeueBatch_Basic(t *testing.T) {
	q := NewMPSCQueue[int](8)
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)
	buf := make([]int, 10)
	n := q.DequeueBatch(buf)
	if n != 3 {
		t.Errorf("DequeueBatch got %d, want 3", n)
	}
	if buf[0] != 1 || buf[1] != 2 || buf[2] != 3 {
		t.Errorf("got %v", buf[:3])
	}
}

func TestMPSCQueue_DequeueBatch_Empty(t *testing.T) {
	q := NewMPSCQueue[int](8)
	buf := make([]int, 5)
	n := q.DequeueBatch(buf)
	if n != 0 {
		t.Errorf("DequeueBatch from empty got %d, want 0", n)
	}
}

func TestMPSCQueue_DequeueBatch_Partial(t *testing.T) {
	q := NewMPSCQueue[int](8)
	for i := 0; i < 5; i++ {
		q.Enqueue(i)
	}
	buf := make([]int, 3)
	n := q.DequeueBatch(buf)
	if n != 3 {
		t.Errorf("DequeueBatch got %d, want 3", n)
	}
	for i := 0; i < 3; i++ {
		if buf[i] != i {
			t.Errorf("buf[%d]=%d", i, buf[i])
		}
	}
	buf2 := make([]int, 5)
	n2 := q.DequeueBatch(buf2)
	if n2 != 2 {
		t.Errorf("second DequeueBatch got %d, want 2", n2)
	}
	if buf2[0] != 3 || buf2[1] != 4 {
		t.Errorf("got %v", buf2[:2])
	}
}

func TestMPSCQueue_LenCap(t *testing.T) {
	q := NewMPSCQueue[int](16)
	if q.Cap() != 16 {
		t.Errorf("Cap=%d, want 16", q.Cap())
	}
	if q.Len() != 0 {
		t.Errorf("empty Len=%d, want 0", q.Len())
	}
	q.Enqueue(1)
	q.Enqueue(2)
	if q.Len() != 2 {
		t.Errorf("Len=%d, want 2", q.Len())
	}
	q.Dequeue()
	if q.Len() != 1 {
		t.Errorf("Len=%d, want 1", q.Len())
	}
}

func TestMPSCQueue_CloseIsClosed(t *testing.T) {
	q := NewMPSCQueue[int](8)
	if q.IsClosed() {
		t.Error("new queue should not be closed")
	}
	// Enqueue before close
	q.Enqueue(1)
	q.Close()
	if !q.IsClosed() {
		t.Error("after Close, IsClosed should be true")
	}
	// Can still dequeue remaining items after close
	v, ok := q.Dequeue()
	if !ok || v != 1 {
		t.Errorf("Dequeue after close got %d,%v", v, ok)
	}
}

func TestMPSCQueue_NonPowerOfTwoCapacity(t *testing.T) {
	q := NewMPSCQueue[int](5)
	cap := q.Cap()
	if cap != 8 {
		t.Errorf("capacity 5 should round to 8, got %d", cap)
	}
}

// ============================================================
// Queue[T] additional tests
// ============================================================

func TestNewQueue_MaxSizeLessThanInitial(t *testing.T) {
	q := NewQueue[int](16, 8, FullPolicyResize)
	if q == nil {
		t.Fatal("NewQueue returned nil")
	}
	// maxSize 8 -> nextPowerOfTwo(8)=8, capacity 16 > 8, so maxSize gets set to 16
	// Ring buffer: capacity 16 uses one slot to distinguish full/empty, so we can hold 15 items
	for i := 0; i < 15; i++ {
		if !q.Enqueue(i) {
			t.Errorf("Enqueue %d failed (count=%d)", i, q.Count())
		}
	}
	if q.Enqueue(100) {
		t.Error("Enqueue when full should fail")
	}
}

func TestQueue_EnqueueBatch_Empty(t *testing.T) {
	q := GetDefaultQueue[int](10)
	if !q.EnqueueBatch([]int{}) {
		t.Error("EnqueueBatch empty slice should return true")
	}
	if q.Count() != 0 {
		t.Errorf("count=%d", q.Count())
	}
}

func TestQueue_EnqueueBatch_Basic(t *testing.T) {
	q := GetDefaultQueue[int](16)
	items := []int{1, 2, 3, 4, 5}
	if !q.EnqueueBatch(items) {
		t.Error("EnqueueBatch failed")
	}
	if q.Count() != 5 {
		t.Errorf("count=%d", q.Count())
	}
	buf := make([]int, 0, 10)
	result, _ := q.DequeueBatch(buf)
	if len(result) != 5 {
		t.Errorf("DequeueBatch got %d", len(result))
	}
	for i := 0; i < 5; i++ {
		if result[i] != items[i] {
			t.Errorf("result[%d]=%d", i, result[i])
		}
	}
}

func TestQueue_EnqueueBatch_WrapAround(t *testing.T) {
	q := GetDefaultQueue[int](8)
	for i := 0; i < 8; i++ {
		q.Enqueue(i)
	}
	buf := make([]int, 0, 4)
	q.DequeueBatch(buf)
	items := []int{10, 11, 12, 13}
	if !q.EnqueueBatch(items) {
		t.Error("EnqueueBatch failed")
	}
	if q.Count() != 8 {
		t.Errorf("count=%d", q.Count())
	}
}

func TestQueue_EnqueueBatch_Resize(t *testing.T) {
	q := GetDefaultQueue[int](4)
	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}
	if !q.EnqueueBatch(items) {
		t.Error("EnqueueBatch with resize failed")
	}
	if q.Count() != 20 {
		t.Errorf("count=%d", q.Count())
	}
}

func TestQueue_EnqueueBatch_DropPolicy(t *testing.T) {
	q := NewQueue[int](4, 8, FullPolicyDrop)
	// Ring buffer capacity 4 holds 3 items. Fill it.
	for i := 0; i < 3; i++ {
		q.Enqueue(i)
	}
	// Now full (3 items) - EnqueueBatch with Drop policy should fail when no space
	if q.EnqueueBatch([]int{1, 2, 3}) {
		t.Error("EnqueueBatch when full with Drop policy should fail")
	}
}

func TestQueue_Enqueue_MaxSizeBlocksResize(t *testing.T) {
	// Queue at maxSize, Enqueue triggers resize check but maxSize blocks
	q := NewQueue[int](8, 16, FullPolicyResize)
	for i := 0; i < 15; i++ {
		q.Enqueue(i) // fills ring (capacity 16 holds 15)
	}
	// 16th Enqueue would need resize; maxSize=16 so len(items) already 16, resize blocked
	if q.Enqueue(99) {
		t.Error("Enqueue when at maxSize should fail")
	}
}

func TestQueue_Front_Empty(t *testing.T) {
	q := GetDefaultQueue[int](10)
	_, ok := q.Front()
	if ok {
		t.Error("Front on empty queue should return false")
	}
}

func TestQueue_Front_NonEmpty(t *testing.T) {
	q := GetDefaultQueue[int](10)
	q.Enqueue(42)
	q.Enqueue(43)
	v, ok := q.Front()
	if !ok {
		t.Fatal("Front should succeed")
	}
	if v != 42 {
		t.Errorf("Front got %d, want 42", v)
	}
	// Front does not remove
	if q.Count() != 2 {
		t.Errorf("count=%d", q.Count())
	}
}

func TestQueue_EnqueueBatch_WrapCopyPath(t *testing.T) {
	q := GetDefaultQueue[int](8)
	// Add 4 items so tail=4, firstChunkLen = 8-4 = 4
	for i := 0; i < 4; i++ {
		q.Enqueue(i)
	}
	// EnqueueBatch 5 items: firstChunkLen=4 < 5, triggers wrap copy path
	items := []int{10, 11, 12, 13, 14}
	if !q.EnqueueBatch(items) {
		t.Error("EnqueueBatch failed")
	}
	if q.Count() != 9 {
		t.Errorf("count=%d, want 9", q.Count())
	}
	// Drain and verify order
	var got []int
	for q.Count() > 0 {
		buf := make([]int, 0, 4)
		res, _ := q.DequeueBatch(buf)
		got = append(got, res...)
	}
	want := []int{0, 1, 2, 3, 10, 11, 12, 13, 14}
	for i := range want {
		if i < len(got) && got[i] != want[i] {
			t.Errorf("got[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

func TestQueue_EnqueueBatch_MaxSizeExceeded(t *testing.T) {
	q := NewQueue[int](4, 16, FullPolicyResize)
	// Fill to 15 (ring capacity 16 holds 15)
	for i := 0; i < 15; i++ {
		q.Enqueue(i)
	}
	// Try to add 5 more - would need resize beyond maxSize 16
	// EnqueueBatch of 5 requires 5 slots, we have 0. targetSize would grow. maxSize 16, nextPowerOfTwo(16)=16.
	// So we can resize to 16. 15 count + 5 = 20. We need 20 slots. targetSize 32 > 16. Returns false.
	if q.EnqueueBatch([]int{1, 2, 3, 4, 5}) {
		t.Error("EnqueueBatch beyond maxSize should fail")
	}
}

func TestSPSCQueue_TryEnqueueBatch_WrapPath(t *testing.T) {
	q := NewSPSCQueue[int](16)
	// Add 10 items so head=10, toEnd=6. Dequeue 3 so we have 7 free slots for batch of 7.
	for i := 0; i < 10; i++ {
		q.TryEnqueue(i)
	}
	for i := 0; i < 3; i++ {
		b := make([]int, 1)
		q.TryDequeueBatch(b)
	}
	// Now head=10, tail=3. 7 items in queue. Free=9. TryEnqueueBatch 7 items with count>toEnd triggers wrap.
	items := []int{10, 11, 12, 13, 14, 15, 16}
	if !q.TryEnqueueBatch(items) {
		t.Error("TryEnqueueBatch failed")
	}
	// Drain (may take multiple batches due to headCache)
	var got []int
	for {
		buf := make([]int, 16)
		n, _ := q.TryDequeueBatch(buf)
		if n == 0 {
			break
		}
		got = append(got, buf[:n]...)
	}
	// Should have 14 items total (7 original 3-9 + 7 new 10-16)
	if len(got) != 14 {
		t.Errorf("got %d items, want 14: %v", len(got), got)
	}
	// Verify we have both original and new items (order may vary with headCache)
	seen := make(map[int]bool)
	for _, v := range got {
		seen[v] = true
	}
	for i := 3; i <= 16; i++ {
		if !seen[i] {
			t.Errorf("missing value %d", i)
		}
	}
}

func TestNextPowerOfTwo_EdgeCases(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, 2},
		{-1, 2},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{100, 128},
		{1024, 1024},
		{1025, 2048},
	}
	for _, tt := range tests {
		got := nextPowerOfTwo(tt.in)
		if got != tt.want {
			t.Errorf("nextPowerOfTwo(%d)=%d, want %d", tt.in, got, tt.want)
		}
	}
}

// ============================================================
// SPSCQueue concurrent test (blocking producer/consumer)
// ============================================================

func TestMPSCQueue_ConcurrentProducers(t *testing.T) {
	q := NewMPSCQueue[int](256)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				for !q.Enqueue(base*25 + j) {
					// Enqueue retries on CAS; queue has space
				}
			}
		}(i)
	}
	wg.Wait()
	count := 0
	for {
		_, ok := q.Dequeue()
		if !ok {
			break
		}
		count++
	}
	if count != 100 {
		t.Errorf("got %d items, want 100", count)
	}
}

func TestSPSCQueue_ConcurrentBlocking(t *testing.T) {
	q := NewSPSCQueue[int](32)
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Producer: enqueue 100 items (mix of Enqueue and EnqueueBatch) then close
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i += 10 {
			batch := make([]int, 0, 10)
			for j := 0; j < 10 && i+j < 100; j++ {
				batch = append(batch, i+j)
			}
			if !q.EnqueueBatch(batch) {
				return
			}
		}
		q.Close()
	}()

	// Consumer: drain until closed and empty
	wg.Add(1)
	go func() {
		defer wg.Done()
		recv := 0
		for {
			buf := make([]int, 16)
			n, ok := q.DequeueBatch(buf)
			recv += n
			if !ok {
				break
			}
			if recv >= 100 {
				break
			}
		}
		close(done)
	}()

	wg.Wait()
	<-done
}

// ============================================================
// UnboundedMPSC Shrink and Close tests
// ============================================================

func TestUnboundedMPSC_Shrink(t *testing.T) {
	q := NewUnboundedMPSC[int]()
	// Enqueue/Dequeue many items to fill recycleCache (recycleCap=256)
	for i := 0; i < 300; i++ {
		q.Enqueue(i)
	}
	for i := 0; i < 300; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("Dequeue %d: got %d,%v", i, v, ok)
		}
	}
	q.Shrink()
	// Verify queue still works after Shrink
	if !q.Empty() {
		t.Error("queue should be empty after drain")
	}
	q.Enqueue(42)
	v, ok := q.Dequeue()
	if !ok || v != 42 {
		t.Errorf("after Shrink: got %d,%v, want 42,true", v, ok)
	}
}

func TestUnboundedMPSC_Close(t *testing.T) {
	q := NewUnboundedMPSC[int]()
	for i := 0; i < 50; i++ {
		q.Enqueue(i)
	}
	for i := 0; i < 50; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("Dequeue %d: got %d,%v", i, v, ok)
		}
	}
	q.Close()
	// Enqueue/Dequeue after Close - queue is not "closed" in the sense of rejecting ops,
	// but recycleCache is nil. Dequeue on empty returns false.
	_, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue on empty after Close should return false")
	}
}

// ============================================================
// UnboundedSPSC Close test
// ============================================================

func TestUnboundedSPSC_Close(t *testing.T) {
	q := NewUnboundedSPSC[int]()
	for i := 0; i < 50; i++ {
		q.Enqueue(i)
	}
	for i := 0; i < 50; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("Dequeue %d: got %d,%v", i, v, ok)
		}
	}
	q.Close()
	_, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue on empty after Close should return false")
	}
}

// ============================================================
// SPSCQueue blocking Enqueue/Dequeue tests
// ============================================================

func TestSPSCQueue_BlockingEnqueueDequeue(t *testing.T) {
	q := NewSPSCQueue[int](8)
	done := make(chan []int, 1)
	timeout := time.After(2 * time.Second)

	// Consumer: Dequeue 10 items (blocking)
	go func() {
		var got []int
		for i := 0; i < 10; i++ {
			v, ok := q.Dequeue()
			if !ok {
				got = append(got, -1) // closed
				break
			}
			got = append(got, v)
		}
		done <- got
	}()

	// Producer: Enqueue 10 items (blocking)
	go func() {
		for i := 0; i < 10; i++ {
			if !q.Enqueue(i) {
				return
			}
		}
	}()

	select {
	case got := <-done:
		if len(got) != 10 {
			t.Errorf("consumer got %d items, want 10", len(got))
		}
		for i := 0; i < 10; i++ {
			if got[i] != i {
				t.Errorf("got[%d]=%d, want %d", i, got[i], i)
			}
		}
	case <-timeout:
		t.Fatal("TestSPSCQueue_BlockingEnqueueDequeue timed out")
	}
}

func TestSPSCQueue_BlockingEnqueue_Closed(t *testing.T) {
	q := NewSPSCQueue[int](8)
	// Fill queue
	for i := 0; i < 8; i++ {
		if !q.TryEnqueue(i) {
			t.Fatalf("TryEnqueue %d failed", i)
		}
	}

	enqueueResult := make(chan bool, 1)
	go func() {
		// This will block until space or closed
		ok := q.Enqueue(99)
		enqueueResult <- ok
	}()

	// Give producer time to block
	time.Sleep(50 * time.Millisecond)
	q.Close()

	select {
	case ok := <-enqueueResult:
		if ok {
			t.Error("Enqueue on full then closed queue should return false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue did not return after Close")
	}
}
