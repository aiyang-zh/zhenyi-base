package zencrypt

import (
	"encoding/hex"
	"testing"
)

func TestSM3_Basic(t *testing.T) {
	hash := SM3("abc")
	if len(hash) != 32 {
		t.Fatalf("SM3 hash should be 32 bytes, got %d", len(hash))
	}

	expected := "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0"
	got := hex.EncodeToString(hash)
	if got != expected {
		t.Fatalf("SM3(\"abc\") = %s, want %s", got, expected)
	}
}

func TestSM3_Empty(t *testing.T) {
	hash := SM3("")
	if len(hash) != 32 {
		t.Fatalf("SM3 hash should be 32 bytes, got %d", len(hash))
	}
}

func TestSM3_Deterministic(t *testing.T) {
	h1 := SM3("test data")
	h2 := SM3("test data")
	if hex.EncodeToString(h1) != hex.EncodeToString(h2) {
		t.Fatal("SM3 should be deterministic")
	}
}

func TestSM3_DifferentInputs(t *testing.T) {
	h1 := SM3("data1")
	h2 := SM3("data2")
	if hex.EncodeToString(h1) == hex.EncodeToString(h2) {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestSM3Bytes(t *testing.T) {
	h1 := SM3("hello")
	h2 := SM3Bytes([]byte("hello"))
	if hex.EncodeToString(h1) != hex.EncodeToString(h2) {
		t.Fatal("SM3 and SM3Bytes should produce same result for same input")
	}
}

func TestSM3Hex(t *testing.T) {
	hashHex := SM3Hex("abc")
	if len(hashHex) != 64 {
		t.Fatalf("SM3Hex should return 64 hex chars, got %d", len(hashHex))
	}

	hash := SM3("abc")
	expected := hex.EncodeToString(hash)
	if hashHex != expected {
		t.Fatalf("SM3Hex mismatch: got %s, want %s", hashHex, expected)
	}
}

func BenchmarkSM3_Short(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SM3("short input")
	}
}

func BenchmarkSM3_256B(b *testing.B) {
	data := string(make([]byte, 256))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM3(data)
	}
}

func BenchmarkSM3Bytes_1KB(b *testing.B) {
	data := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM3Bytes(data)
	}
}
