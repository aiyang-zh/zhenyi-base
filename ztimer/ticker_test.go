package ztimer

import (
	"testing"
	"time"
)

// ============================================================
// Ticker 池化
// ============================================================

func TestTicker_NewTicker(t *testing.T) {
	ticker := NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	select {
	case <-ticker.C():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ticker did not fire")
	}
}

func TestTicker_ResetTime(t *testing.T) {
	ticker := NewTicker(1 * time.Hour)
	defer ticker.Stop()

	ticker.ResetTime(50 * time.Millisecond)
	select {
	case <-ticker.C():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ticker did not fire after ResetTime")
	}
}

func TestTicker_StopAndReuse(t *testing.T) {
	ticker1 := NewTicker(50 * time.Millisecond)
	ticker1.Stop() // 归还池

	ticker2 := NewTicker(50 * time.Millisecond)
	defer ticker2.Stop()

	select {
	case <-ticker2.C():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("reused ticker did not fire")
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkNewTicker_Stop(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ticker := NewTicker(time.Second)
		ticker.Stop()
	}
}
