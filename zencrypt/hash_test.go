package zencrypt

import (
	"testing"
)

// ============================================================
// FnvHash (hash.go)
// ============================================================

func TestFnvHash_Basic(t *testing.T) {
	hash, err := FnvHash("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == 0 {
		t.Error("hash should not be 0")
	}
}

func TestFnvHash_Deterministic(t *testing.T) {
	h1, _ := FnvHash("test_input")
	h2, _ := FnvHash("test_input")
	if h1 != h2 {
		t.Errorf("same input should produce same hash: %d != %d", h1, h2)
	}
}

func TestFnvHash_Different(t *testing.T) {
	h1, _ := FnvHash("abc")
	h2, _ := FnvHash("def")
	if h1 == h2 {
		t.Error("different inputs should (likely) produce different hashes")
	}
}

func TestFnvHash_EmptyString(t *testing.T) {
	_, err := FnvHash("")
	if err != nil {
		t.Fatalf("empty string should not cause error: %v", err)
	}
}

func BenchmarkFnvHash(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		FnvHash("benchmark_input_string")
	}
}
