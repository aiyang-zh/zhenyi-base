package zencrypt

import (
	"testing"
)

// ============================================================
// Bcrypt (bcrypt.go)
// ============================================================

func TestBcrypt_Encrypt(t *testing.T) {
	b := NewBcrypt()
	hash, err := b.Encrypt("password123")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if len(hash) == 0 {
		t.Error("hash should not be empty")
	}
}

func TestBcrypt_NonDeterministic(t *testing.T) {
	b := NewBcrypt()
	h1, _ := b.Encrypt("password")
	h2, _ := b.Encrypt("password")
	if h1 == h2 {
		t.Error("bcrypt should produce different hashes due to random salt")
	}
}

func TestBcrypt_CompareHashAndPassword_Match(t *testing.T) {
	bc := NewBcrypt()
	hash, _ := bc.Encrypt("mypassword")
	if !bc.CompareHashAndPassword("mypassword", hash) {
		t.Error("should match with correct password")
	}
}

func TestBcrypt_CompareHashAndPassword_Mismatch(t *testing.T) {
	bc := NewBcrypt()
	hash, _ := bc.Encrypt("mypassword")
	if bc.CompareHashAndPassword("wrongpassword", hash) {
		t.Error("should not match with wrong password")
	}
}

func TestBcrypt_CompareHashAndPassword_InvalidHash(t *testing.T) {
	bc := NewBcrypt()
	if bc.CompareHashAndPassword("password", "!!!invalid!!!") {
		t.Error("should return false for invalid hash")
	}
}

func BenchmarkBcrypt_Encrypt(b *testing.B) {
	bc := NewBcrypt()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bc.Encrypt("benchmark_password")
	}
}
