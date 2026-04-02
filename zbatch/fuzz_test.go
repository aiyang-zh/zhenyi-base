package zbatch

import (
	"testing"
	"time"
)

func FuzzFastAdaptiveBatcher(f *testing.F) {
	// Seed with a valid-ish configuration.
	f.Add(uint16(10), uint16(200), uint16(5))

	f.Fuzz(func(t *testing.T, minB, maxB, targetMs uint16) {
		min := int(minB%64) + 1
		max := int(maxB%256) + 1
		if max < min {
			min, max = max, min
		}
		target := time.Duration(int(targetMs%50)+1) * time.Millisecond

		b := NewFastAdaptiveBatcher(min, max, target)

		ops := 0
		if len(t.Name()) > 0 {
			// keep ops deterministic-ish without using random.
			ops = 128
		}

		// Drive with a sequence derived from parameters; ensure no panic and invariants hold.
		for i := 0; i < ops; i++ {
			lastFetch := int64(int8(i % 17)) // may be negative
			if lastFetch < 0 {
				lastFetch = -lastFetch
			}

			_ = b.GetBatchSize(lastFetch)

			// record latency in [0, 50ms] range (bounded)
			latMs := int64(int8((i * 3) % 51))
			if latMs < 0 {
				latMs = -latMs
			}
			b.RecordLatency(time.Duration(latMs) * time.Microsecond)
		}

		cb := b.GetCurrentBatch()
		if cb < min || cb > max {
			t.Fatalf("currentBatch out of bounds: got=%d want in [%d,%d]", cb, min, max)
		}

		// Reset must keep invariants.
		b.Reset()
		cb2 := b.GetCurrentBatch()
		if cb2 < min || cb2 > max {
			t.Fatalf("currentBatch after Reset out of bounds: got=%d want in [%d,%d]", cb2, min, max)
		}
		if b.GetAvgLatency() < 0 {
			t.Fatalf("avg latency negative: %v", b.GetAvgLatency())
		}
	})
}
