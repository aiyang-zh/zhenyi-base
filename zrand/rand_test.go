package zrand

import (
	"math"
	"testing"
)

// ============================================================
// RandomList
// ============================================================

func TestRandomList_Int(t *testing.T) {
	list := []int{10, 20, 30, 40, 50}
	seen := make(map[int]bool)
	for i := 0; i < 1000; i++ {
		v := RandomList(list)
		seen[v] = true
	}
	// 1000 次应该覆盖到大部分元素
	if len(seen) < 3 {
		t.Errorf("expected more variety, got %d unique values", len(seen))
	}
}

func TestRandomList_String(t *testing.T) {
	list := []string{"a", "b", "c"}
	v := RandomList(list)
	found := false
	for _, s := range list {
		if s == v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RandomList returned %q, not in list", v)
	}
}

func TestRandomList_SingleElement(t *testing.T) {
	list := []int{42}
	v := RandomList(list)
	if v != 42 {
		t.Errorf("single element: got %d, want 42", v)
	}
}

// ============================================================
// Range — 整数
// ============================================================

func TestRange_Int(t *testing.T) {
	for i := 0; i < 1000; i++ {
		v := Range(10, 20)
		if v < 10 || v >= 20 {
			t.Fatalf("Range(10,20) returned %d, expected [10,20)", v)
		}
	}
}

func TestRange_Int_Reversed(t *testing.T) {
	// min > max 应自动交换
	for i := 0; i < 100; i++ {
		v := Range(20, 10)
		if v < 10 || v >= 20 {
			t.Fatalf("Range(20,10) returned %d, expected [10,20)", v)
		}
	}
}

func TestRange_Int_Equal(t *testing.T) {
	v := Range(5, 5)
	if v != 5 {
		t.Errorf("Range(5,5): got %d, want 5", v)
	}
}

func TestRange_Int32(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := Range[int32](0, 100)
		if v < 0 || v >= 100 {
			t.Fatalf("Range[int32](0,100): got %d", v)
		}
	}
}

func TestRange_Uint32(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := Range[uint32](50, 100)
		if v < 50 || v >= 100 {
			t.Fatalf("Range[uint32](50,100): got %d", v)
		}
	}
}

// ============================================================
// Range — 浮点数
// ============================================================

func TestRange_Float64(t *testing.T) {
	for i := 0; i < 1000; i++ {
		v := Range(1.0, 10.0)
		if v < 1.0 || v >= 10.0 {
			t.Fatalf("Range(1.0,10.0) returned %f", v)
		}
	}
}

func TestRange_Float64_Reversed(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := Range(10.0, 1.0)
		if v < 1.0 || v >= 10.0 {
			t.Fatalf("Range(10.0,1.0) returned %f", v)
		}
	}
}

func TestRange_Float64_Equal(t *testing.T) {
	v := Range(3.14, 3.14)
	if v != 3.14 {
		t.Errorf("Range(3.14,3.14): got %f", v)
	}
}

func TestRange_Float32(t *testing.T) {
	for i := 0; i < 100; i++ {
		v := Range[float32](0, 1.0)
		if v < 0 || v >= 1.0 {
			t.Fatalf("Range[float32](0,1): got %f", v)
		}
	}
}

// ============================================================
// 均匀性检验（简单 chi-squared）
// ============================================================

func TestRange_Int_Distribution(t *testing.T) {
	const trials = 100000
	const min, max = 0, 10
	bins := make([]int, max-min)

	for i := 0; i < trials; i++ {
		v := Range(min, max)
		bins[v-min]++
	}

	expected := float64(trials) / float64(max-min)
	for i, count := range bins {
		ratio := float64(count) / expected
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("bin %d: count=%d, expected ~%.0f, ratio=%.2f (>20%% deviation)",
				i, count, expected, ratio)
		}
	}
}

func TestRange_Float64_Distribution(t *testing.T) {
	const trials = 100000
	const buckets = 10
	bins := make([]int, buckets)

	for i := 0; i < trials; i++ {
		v := Range(0.0, 10.0)
		idx := int(math.Floor(v))
		if idx >= buckets {
			idx = buckets - 1
		}
		bins[idx]++
	}

	expected := float64(trials) / float64(buckets)
	for i, count := range bins {
		ratio := float64(count) / expected
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("bucket %d: count=%d, expected ~%.0f, ratio=%.2f",
				i, count, expected, ratio)
		}
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkRandomList(b *testing.B) {
	list := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RandomList(list)
	}
}

func BenchmarkRange_Int(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Range(0, 100)
	}
}

func BenchmarkRange_Float64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Range(0.0, 100.0)
	}
}

func BenchmarkRange_Int32(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Range[int32](0, 1000)
	}
}
