package zencrypt

import (
	"bytes"
	"testing"
)

// ============================================================
// SHA (sha.go)
// ============================================================

func TestSha256_Basic(t *testing.T) {
	hash := Sha256("hello")
	if len(hash) != 32 {
		t.Errorf("SHA-256 should produce 32 bytes, got %d", len(hash))
	}
}

func TestSha256_Deterministic(t *testing.T) {
	h1 := Sha256("test")
	h2 := Sha256("test")
	if !bytes.Equal(h1, h2) {
		t.Error("same input should produce same hash")
	}
}

func TestSha256_Different(t *testing.T) {
	h1 := Sha256("abc")
	h2 := Sha256("def")
	if bytes.Equal(h1, h2) {
		t.Error("different inputs should produce different hashes")
	}
}

func TestSha256_Empty(t *testing.T) {
	hash := Sha256("")
	if len(hash) != 32 {
		t.Error("empty string should still produce 32-byte hash")
	}
}

func BenchmarkSha256(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Sha256("benchmark_input_string_for_sha256")
	}
}
