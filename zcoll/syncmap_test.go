package zcoll

import (
	"sync"
	"testing"
)

func TestSyncMap_LoadStore(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("a", 1)
	v, ok := m.Load("a")
	if !ok || v != 1 {
		t.Fatalf("Load a: ok=%v v=%d", ok, v)
	}
	var zero int
	v, ok = m.Load("missing")
	if ok || v != zero {
		t.Fatalf("Load missing: ok=%v v=%d", ok, v)
	}
}

func TestSyncMap_LoadOrStore(t *testing.T) {
	m := NewSyncMap[string, int]()
	a, loaded := m.LoadOrStore("k", 10)
	if loaded || a != 10 {
		t.Fatalf("first LoadOrStore: loaded=%v a=%d", loaded, a)
	}
	a, loaded = m.LoadOrStore("k", 20)
	if !loaded || a != 10 {
		t.Fatalf("second LoadOrStore: loaded=%v a=%d", loaded, a)
	}
}

func TestSyncMap_LoadAndDelete(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("k", 5)
	v, loaded := m.LoadAndDelete("k")
	if !loaded || v != 5 {
		t.Fatalf("LoadAndDelete: loaded=%v v=%d", loaded, v)
	}
	_, loaded = m.LoadAndDelete("k")
	if loaded {
		t.Fatal("expected not loaded after delete")
	}
}

func TestSyncMap_Delete(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("k", 1)
	m.Delete("k")
	_, ok := m.Load("k")
	if ok {
		t.Fatal("expected key gone")
	}
}

func TestSyncMap_Swap(t *testing.T) {
	m := NewSyncMap[string, int]()
	prev, loaded := m.Swap("k", 1)
	if loaded {
		t.Fatal("first swap should not be loaded")
	}
	var zero int
	if prev != zero {
		t.Fatalf("prev=%d want zero", prev)
	}
	v, ok := m.Load("k")
	if !ok || v != 1 {
		t.Fatalf("after swap: ok=%v v=%d", ok, v)
	}
	prev, loaded = m.Swap("k", 2)
	if !loaded || prev != 1 {
		t.Fatalf("second swap: loaded=%v prev=%d", loaded, prev)
	}
}

func TestSyncMap_CompareAndSwap(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("k", 1)
	if !m.CompareAndSwap("k", 1, 2) {
		t.Fatal("CAS should succeed")
	}
	v, _ := m.Load("k")
	if v != 2 {
		t.Fatalf("v=%d want 2", v)
	}
	if m.CompareAndSwap("k", 1, 3) {
		t.Fatal("CAS should fail")
	}
}

func TestSyncMap_CompareAndDelete(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("k", 1)
	if !m.CompareAndDelete("k", 1) {
		t.Fatal("CAD should succeed")
	}
	_, ok := m.Load("k")
	if ok {
		t.Fatal("key should be gone")
	}
	m.Store("k", 2)
	if m.CompareAndDelete("k", 1) {
		t.Fatal("CAD should fail on wrong old")
	}
}

func TestSyncMap_Range(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	n := 0
	sum := 0
	m.Range(func(k string, v int) bool {
		n++
		sum += v
		return true
	})
	if n != 2 || sum != 3 {
		t.Fatalf("Range: n=%d sum=%d", n, sum)
	}
	n = 0
	m.Range(func(k string, v int) bool {
		n++
		return false
	})
	if n != 1 {
		t.Fatalf("early stop: n=%d", n)
	}
}

func TestSyncMap_Clear(t *testing.T) {
	m := NewSyncMap[string, int]()
	m.Store("a", 1)
	m.Clear()
	_, ok := m.Load("a")
	if ok {
		t.Fatal("expected empty after Clear")
	}
}

func TestSyncMap_Concurrent(t *testing.T) {
	m := NewSyncMap[int, int]()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				k := base*10000 + i
				m.Store(k, k)
				m.Load(k)
			}
		}(g)
	}
	wg.Wait()
}
