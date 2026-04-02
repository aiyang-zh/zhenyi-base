package zqueue

import (
	"reflect"
	"testing"
)

// FuzzUnboundedSPSCQueueOps fuzz UnboundedSPSC in a single-producer/single-consumer setting.
// We validate FIFO order using Enqueue / EnqueueBatch and DequeueBatch.
func FuzzUnboundedSPSCQueueOps(f *testing.F) {
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}

		q := NewUnboundedSPSC[int]()
		expected := make([]int, 0, 256)

		var batchBuf [32]int
		var outBuf [64]int

		pos := 1
		rounds := len(data) - pos
		if rounds > 250 {
			rounds = 250
		}

		for r := 0; r < rounds && pos < len(data); r++ {
			op := data[pos] % 3
			pos++

			switch op {
			case 0: // Enqueue
				if pos >= len(data) {
					return
				}
				val := int(int8(data[pos]))
				pos++
				q.Enqueue(val)
				expected = append(expected, val)

			case 1: // EnqueueBatch
				if pos >= len(data) {
					return
				}
				bsz := int(data[pos]%8) + 1
				pos++
				if bsz > len(batchBuf) {
					bsz = len(batchBuf)
				}
				if pos+bsz > len(data) {
					return
				}
				for i := 0; i < bsz; i++ {
					batchBuf[i] = int(int8(data[pos]))
					pos++
				}
				n := q.EnqueueBatch(batchBuf[:bsz])
				if n != bsz {
					t.Fatalf("UnboundedSPSC.EnqueueBatch returned n=%d want=%d", n, bsz)
				}
				expected = append(expected, batchBuf[:bsz]...)

			case 2: // DequeueBatch
				if pos >= len(data) {
					return
				}
				lim := int(data[pos] % 16) // 0..15
				pos++
				if lim == 0 {
					continue
				}
				if lim > len(outBuf) {
					lim = len(outBuf)
				}

				n := q.DequeueBatch(outBuf[:lim])
				if n > len(expected) {
					t.Fatalf("dequeue returned n=%d expected len=%d", n, len(expected))
				}
				if !reflect.DeepEqual(outBuf[:n], expected[:n]) {
					t.Fatalf("dequeue mismatch: got=%v want=%v", outBuf[:n], expected[:n])
				}
				expected = expected[n:]
			}

			// Basic empty invariant
			if len(expected) == 0 && !q.Empty() {
				t.Fatalf("queue should be empty but Empty()==false")
			}
		}
	})
}

// FuzzUnboundedMPSCQueueOps fuzz UnboundedMPSC using a single producer (still exercises node/next pointers).
// It validates FIFO order under EnqueueBatch and DequeueBatch.
func FuzzUnboundedMPSCQueueOps(f *testing.F) {
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}

		q := NewUnboundedMPSC[int]()
		expected := make([]int, 0, 256)

		var batchBuf [32]int
		var outBuf [64]int

		pos := 1
		rounds := len(data) - pos
		if rounds > 300 {
			rounds = 300
		}

		for r := 0; r < rounds && pos < len(data); r++ {
			op := data[pos] % 3
			pos++

			switch op {
			case 0: // Enqueue
				if pos >= len(data) {
					return
				}
				val := int(int8(data[pos]))
				pos++
				q.Enqueue(val)
				expected = append(expected, val)

			case 1: // EnqueueBatch
				if pos >= len(data) {
					return
				}
				bsz := int(data[pos]%10) + 1
				pos++
				if bsz > len(batchBuf) {
					bsz = len(batchBuf)
				}
				if pos+bsz > len(data) {
					return
				}
				for i := 0; i < bsz; i++ {
					batchBuf[i] = int(int8(data[pos]))
					pos++
				}
				q.EnqueueBatch(batchBuf[:bsz])
				expected = append(expected, batchBuf[:bsz]...)

			case 2: // DequeueBatch
				if pos >= len(data) {
					return
				}
				lim := int(data[pos] % 16) // 0..15
				pos++
				if lim == 0 {
					continue
				}
				if lim > len(outBuf) {
					lim = len(outBuf)
				}

				n := q.DequeueBatch(outBuf[:lim])
				if n > len(expected) {
					t.Fatalf("dequeue returned n=%d expected len=%d", n, len(expected))
				}
				if !reflect.DeepEqual(outBuf[:n], expected[:n]) {
					t.Fatalf("dequeue mismatch: got=%v want=%v", outBuf[:n], expected[:n])
				}
				expected = expected[n:]
			}

			if len(expected) == 0 && !q.Empty() {
				t.Fatalf("queue should be empty but Empty()==false")
			}
		}
	})
}
