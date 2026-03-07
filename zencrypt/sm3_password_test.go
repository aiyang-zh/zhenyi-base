package zencrypt

import (
	"testing"
)

func TestSM3Password_Encrypt(t *testing.T) {
	h := NewSM3Password()
	hash, err := h.Encrypt("password123")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("hash should not be empty")
	}
}

func TestSM3Password_RandomSalt(t *testing.T) {
	h := NewSM3Password()
	h1, _ := h.Encrypt("password")
	h2, _ := h.Encrypt("password")
	if h1 == h2 {
		t.Fatal("same password should produce different hashes (random salt)")
	}
}

func TestSM3Password_DifferentPasswords(t *testing.T) {
	h := NewSM3Password()
	h1, _ := h.Encrypt("password1")
	h2, _ := h.Encrypt("password2")
	if h1 == h2 {
		t.Fatal("different passwords should produce different hashes")
	}
}

func TestSM3Password_CompareHashAndPassword_Match(t *testing.T) {
	h := NewSM3Password()
	hash, _ := h.Encrypt("mypassword")
	if !h.CompareHashAndPassword("mypassword", hash) {
		t.Fatal("correct password should match")
	}
}

func TestSM3Password_CompareHashAndPassword_Mismatch(t *testing.T) {
	h := NewSM3Password()
	hash, _ := h.Encrypt("mypassword")
	if h.CompareHashAndPassword("wrongpassword", hash) {
		t.Fatal("wrong password should not match")
	}
}

func TestSM3Password_CompareHashAndPassword_InvalidHash(t *testing.T) {
	h := NewSM3Password()
	if h.CompareHashAndPassword("pass", "not-valid-base64!!!") {
		t.Fatal("invalid hash should not match")
	}
}

func TestSM3Password_CompareHashAndPassword_TooShort(t *testing.T) {
	h := NewSM3Password()
	if h.CompareHashAndPassword("pass", "dG9vc2hvcnQ") {
		t.Fatal("too-short hash should not match")
	}
}

func BenchmarkSM3Password_Encrypt(b *testing.B) {
	h := NewSM3Password()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Encrypt("benchmark_password")
	}
}

func BenchmarkSM3Password_Compare(b *testing.B) {
	h := NewSM3Password()
	hash, _ := h.Encrypt("benchmark_password")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.CompareHashAndPassword("benchmark_password", hash)
	}
}
