package zqueue

import (
	"reflect"
	"testing"
)

// FuzzSPSCQueueOps fuzz SPSCQueue using non-blocking Try* APIs.
// Semantics model is a FIFO slice; only operations that succeed are applied to the model.
func FuzzSPSCQueueOps(f *testing.F) {
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}

		capacity := 2 + int(data[0]%32)
		q := NewSPSCQueue[int](capacity)
		expected := make([]int, 0, 128)

		var buf [16]int
		var out [16]int

		pos := 1
		rounds := len(data) - pos
		if rounds > 200 {
			rounds = 200
		}

		for r := 0; r < rounds && pos < len(data); r++ {
			op := data[pos] % 3
			pos++

			switch op {
			case 0: // TryEnqueue
				if pos >= len(data) {
					return
				}
				val := int(int8(data[pos]))
				pos++
				if q.TryEnqueue(val) {
					expected = append(expected, val)
				}

			case 1: // TryEnqueueBatch (atomic all-or-nothing for SPSC Try*)
				if pos >= len(data) {
					return
				}
				bsz := int(data[pos]%8) + 1
				pos++
				if bsz > len(buf) {
					bsz = len(buf)
				}
				for i := 0; i < bsz && pos < len(data); i++ {
					buf[i] = int(int8(data[pos]))
					pos++
				}
				batch := buf[:bsz]
				if q.TryEnqueueBatch(batch) {
					expected = append(expected, batch...)
				}

			case 2: // TryDequeueBatch
				if pos >= len(data) {
					return
				}
				l := int(data[pos] % 8)
				pos++
				if l == 0 {
					continue
				}
				if l > len(out) {
					l = len(out)
				}

				n, ok := q.TryDequeueBatch(out[:l])
				if !ok {
					t.Fatalf("TryDequeueBatch returned ok=false (queue unexpectedly closed)")
				}
				if n == 0 {
					if len(expected) != 0 {
						t.Fatalf("dequeue n=0 but expected non-empty: got=%d", len(expected))
					}
					continue
				}
				if n > len(expected) {
					t.Fatalf("dequeue returned more than expected: got=%d expected=%d", n, len(expected))
				}
				if !reflect.DeepEqual(out[:n], expected[:n]) {
					t.Fatalf("dequeue mismatch: got=%v want=%v", out[:n], expected[:n])
				}
				expected = expected[n:]
			}
		}
	})
}

// FuzzMPSCQueueOps fuzz MPSCQueue using single goroutine and non-blocking Try* APIs.
// We still validate FIFO semantics because MPSCQueue is single-consumer, and we use one producer here.
func FuzzMPSCQueueOps(f *testing.F) {
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}

		capacity := 4 + int(data[0]%64)
		q := NewMPSCQueue[int](capacity)
		expected := make([]int, 0, 256)

		var batchBuf [16]int
		var outBuf [16]int

		pos := 1
		rounds := len(data) - pos
		if rounds > 250 {
			rounds = 250
		}

		for r := 0; r < rounds && pos < len(data); r++ {
			op := data[pos] % 3
			pos++

			switch op {
			case 0: // TryEnqueue
				if pos >= len(data) {
					return
				}
				val := int(int8(data[pos]))
				pos++
				if q.TryEnqueue(val) {
					expected = append(expected, val)
				}

			case 1: // TryEnqueueBatch
				if pos >= len(data) {
					return
				}
				bsz := int(data[pos]%12) + 1
				pos++
				if bsz > len(batchBuf) {
					bsz = len(batchBuf)
				}
				for i := 0; i < bsz && pos < len(data); i++ {
					batchBuf[i] = int(int8(data[pos]))
					pos++
				}
				batch := batchBuf[:bsz]
				n := q.TryEnqueueBatch(batch)
				if n > 0 {
					expected = append(expected, batch[:n]...)
				}

			case 2: // DequeueBatch
				if pos >= len(data) {
					return
				}
				l := int(data[pos] % 12)
				pos++
				if l == 0 {
					continue
				}
				if l > len(outBuf) {
					l = len(outBuf)
				}

				n := q.DequeueBatch(outBuf[:l])
				if n == 0 {
					if len(expected) != 0 {
						t.Fatalf("dequeue n=0 but expected non-empty: got=%d", len(expected))
					}
					continue
				}
				if n > len(expected) {
					t.Fatalf("dequeue returned more than expected: got=%d expected=%d", n, len(expected))
				}
				if !reflect.DeepEqual(outBuf[:n], expected[:n]) {
					t.Fatalf("dequeue mismatch: got=%v want=%v", outBuf[:n], expected[:n])
				}
				expected = expected[n:]
			}
		}
	})
}
