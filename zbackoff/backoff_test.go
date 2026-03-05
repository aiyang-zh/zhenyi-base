package zbackoff

import (
	"testing"
	"time"
)

// ============================================================
// Backoff 分支逻辑
// ============================================================

func TestBackoff_SpinPhase(t *testing.T) {
	// k < first → procyield（自旋），应几乎不耗时
	start := time.Now()
	for k := 0; k < 5; k++ {
		Backoff(k, 5, 10, time.Millisecond)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("spin phase took too long: %v", elapsed)
	}
}

func TestBackoff_GoschedPhase(t *testing.T) {
	// first <= k < second → Gosched，应几乎不耗时
	start := time.Now()
	for k := 5; k < 10; k++ {
		Backoff(k, 5, 10, time.Millisecond)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("gosched phase took too long: %v", elapsed)
	}
}

func TestBackoff_SleepPhase(t *testing.T) {
	// k >= second → time.Sleep，应至少耗时 t
	start := time.Now()
	Backoff(10, 5, 10, 50*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Errorf("sleep phase too short: %v, expected ~50ms", elapsed)
	}
}

func TestBackoff_BoundaryValues(t *testing.T) {
	// k == first-1 → 最后一次 spin
	Backoff(4, 5, 10, time.Millisecond)

	// k == first → 第一次 gosched
	Backoff(5, 5, 10, time.Millisecond)

	// k == second-1 → 最后一次 gosched
	Backoff(9, 5, 10, time.Millisecond)

	// k == second → 第一次 sleep
	start := time.Now()
	Backoff(10, 5, 10, 20*time.Millisecond)
	if time.Since(start) < 15*time.Millisecond {
		t.Error("boundary: should have slept")
	}
}

func TestBackoff_ZeroThresholds(t *testing.T) {
	// first=0, second=0 → 所有 k 都走 sleep
	start := time.Now()
	Backoff(0, 0, 0, 20*time.Millisecond)
	if time.Since(start) < 15*time.Millisecond {
		t.Error("zero thresholds: should have slept")
	}
}

func TestBackoff_LargeK(t *testing.T) {
	// 非常大的 k 值不应 panic
	start := time.Now()
	Backoff(1000000, 5, 10, time.Millisecond)
	if time.Since(start) < time.Millisecond/2 {
		t.Error("large k: should have slept")
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkBackoff_Spin(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Backoff(0, 5, 10, time.Millisecond)
	}
}

func BenchmarkBackoff_Gosched(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Backoff(5, 5, 10, time.Millisecond)
	}
}

func BenchmarkBackoff_AdaptiveLoop(b *testing.B) {
	// 模拟实际使用：从 spin 到 gosched 的完整循环
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for k := 0; k < 10; k++ {
			Backoff(k, 5, 10, time.Millisecond)
		}
	}
}
