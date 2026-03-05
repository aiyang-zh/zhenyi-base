package zid

import (
	"sync"
	"testing"
)

// ============================================================
// FastId
// ============================================================

func TestFastId_Init(t *testing.T) {
	InitFast(0)
	id1 := NextFast()
	id2 := NextFast()
	if id1 == id2 {
		t.Errorf("consecutive IDs should be different: %d == %d", id1, id2)
	}
}

func TestFastId_UniqueIds(t *testing.T) {
	InitFast(1)
	seen := make(map[uint64]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := NextFast()
		if seen[id] {
			t.Fatalf("duplicate ID at iter %d: %d", i, id)
		}
		seen[id] = true
	}
}

func TestFastId_DifferentNodes(t *testing.T) {
	InitFast(1)
	id1 := NextFast()

	InitFast(2)
	id2 := NextFast()

	// 不同节点产生的 ID 高位不同
	if id1 == id2 {
		t.Error("IDs from different nodes should differ")
	}
}

func TestFastId_Concurrent(t *testing.T) {
	InitFast(0)
	const goroutines = 100
	const idsPerGoroutine = 1000

	results := make(chan uint64, goroutines*idsPerGoroutine)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				results <- NextFast()
			}
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[uint64]bool, goroutines*idsPerGoroutine)
	for id := range results {
		if seen[id] {
			t.Fatalf("duplicate ID: %d", id)
		}
		seen[id] = true
	}
}

func TestFastId_NodeIdOverflow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nodeId > NodeMask")
		}
	}()
	InitFast(NodeMask + 1)
}

// ============================================================
// SonyFlake (Next)
// ============================================================

func TestSonyFlake_Next(t *testing.T) {
	Init(0)
	id1 := Next()
	id2 := Next()
	if id1 == 0 || id2 == 0 {
		t.Error("Next() returned 0")
	}
	if id1 == id2 {
		t.Errorf("consecutive IDs should differ: %d", id1)
	}
}

func TestSonyFlake_Unique(t *testing.T) {
	Init(1)
	seen := make(map[uint64]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := Next()
		if seen[id] {
			t.Fatalf("duplicate at iter %d: %d", i, id)
		}
		seen[id] = true
	}
}

// ============================================================
// UUID
// ============================================================

func TestUuid_GenId(t *testing.T) {
	u := NewUuid()
	id := u.GenId()
	if len(id) == 0 {
		t.Fatal("empty UUID")
	}
	// UUID 格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars)
	if len(id) != 36 {
		t.Errorf("unexpected UUID length: %d, val: %s", len(id), id)
	}
}

func TestUuid_Unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewUuid().GenId()
		if seen[id] {
			t.Fatalf("duplicate UUID at iter %d: %s", i, id)
		}
		seen[id] = true
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkNextFast_ReportAllocs(b *testing.B) {
	InitFast(0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NextFast()
	}
}

func BenchmarkNextFast_Parallel(b *testing.B) {
	InitFast(0)
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NextFast()
		}
	})
}

func BenchmarkNext_SonyFlake(b *testing.B) {
	Init(0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Next()
	}
}

func BenchmarkUuid_GenId(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewUuid().GenId()
	}
}
