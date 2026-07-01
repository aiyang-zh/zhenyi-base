package zqueue

import (
	"reflect"
	"testing"
)

// FuzzQueueOps tests bounded Queue[int] semantics under random operation sequences.
// Focus:
// - 不 panic
// - 与模型 expected 保持一致（入队成功才进入模型；出队返回与模型前缀一致）
func FuzzQueueOps(f *testing.F) {
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Need at least 4 bytes because we may read data[3] when data[2]%4 == 0.
		if len(data) < 4 {
			return
		}

		initial := 2 + int(data[0]%32) // >=2
		drop := data[1]%2 == 0
		policy := FullPolicyResize
		if drop {
			policy = FullPolicyDrop
		}

		// Small maxSize to increase drop/false paths; 0 means unlimited.
		maxSize := 0
		if data[2]%4 == 0 {
			maxSize = 2 + int(data[3]%64)
		}
		if maxSize > 256 {
			maxSize = 256
		}

		q := NewQueue[int](initial, maxSize, policy)

		expected := make([]int, 0, 64)
		buf := make([]int, 0, 64) // used for DequeueBatch
		closed := false

		maxOps := len(data)
		if maxOps > 200 {
			maxOps = 200
		}

		// ops decode layout:
		// op = data[i] % 6
		// then we may consume 1-3 extra bytes for the specific op's parameters/values.
		for i := 3; i < maxOps && i < len(data); i++ {
			op := data[i] % 6

			checkCount := func() {
				if got := q.Count(); got != len(expected) {
					t.Fatalf("q.Count mismatch: got=%d want=%d (policy=%d initial=%d maxSize=%d closed=%v)", got, len(expected), policy, initial, maxSize, closed)
				}
			}

			switch op {
			case 0: // Enqueue
				val := int(int8(data[i]))
				ok := q.Enqueue(val)
				if closed {
					if ok {
						t.Fatalf("Enqueue succeeded after Close")
					}
				} else if ok {
					expected = append(expected, val)
				}
				checkCount()

			case 1: // EnqueueBatch
				if i+1 >= len(data) {
					continue
				}
				bsz := int(data[i+1]%8) + 1
				i++

				batch := make([]int, 0, bsz)
				for k := 0; k < bsz; k++ {
					if i+1 >= len(data) {
						break
					}
					i++
					batch = append(batch, int(int8(data[i])))
				}

				if len(batch) == 0 {
					if !q.EnqueueBatch(batch) {
						t.Fatal("EnqueueBatch empty slice should return true")
					}
					checkCount()
					continue
				}

				ok := q.EnqueueBatch(batch)
				if closed {
					if ok {
						t.Fatalf("EnqueueBatch succeeded after Close")
					}
				} else if ok {
					expected = append(expected, batch...)
				}
				checkCount()

			case 2: // DequeueBatch
				limit := int(data[i]) % cap(buf)
				if limit == 0 {
					limit = 1
				}
				tmp := buf[:0:limit]

				gotSlice, _ := q.DequeueBatch(tmp)
				need := len(gotSlice)
				if need > len(expected) {
					t.Fatalf("dequeue batch returned more than expected: got=%d expected=%d", need, len(expected))
				}
				wantSlice := expected[:need]
				if !reflect.DeepEqual(gotSlice, wantSlice) {
					t.Fatalf("dequeue batch mismatch: got=%v want=%v", gotSlice, wantSlice)
				}
				expected = expected[need:]
				checkCount()

			case 3: // Front
				front, ok := q.Front()
				if len(expected) == 0 {
					if ok {
						t.Fatalf("Front should be empty but ok=true, front=%v", front)
					}
				} else {
					if !ok {
						t.Fatalf("Front should be non-empty but ok=false")
					}
					if front != expected[0] {
						t.Fatalf("Front mismatch: got=%d want=%d", front, expected[0])
					}
				}
				checkCount()

			case 4: // Dequeue
				got, ok := q.Dequeue()
				if len(expected) == 0 {
					if ok {
						t.Fatalf("Dequeue should be empty but ok=true, got=%v", got)
					}
				} else {
					if !ok {
						t.Fatalf("Dequeue should succeed but ok=false")
					}
					if got != expected[0] {
						t.Fatalf("Dequeue mismatch: got=%d want=%d", got, expected[0])
					}
					expected = expected[1:]
				}
				checkCount()

			case 5: // Close
				q.Close()
				closed = true
				if !q.IsClosed() {
					t.Fatal("IsClosed should be true after Close")
				}
				checkCount()
			}
		}
	})
}
