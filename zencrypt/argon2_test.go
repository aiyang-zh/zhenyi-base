package zencrypt

import (
	"testing"
)

// ============================================================
// Argon2 (argon2.go)
// ============================================================

func TestArgon2_Encrypt(t *testing.T) {
	a := NewArgon2()
	hash, err := a.Encrypt("password123")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if len(hash) == 0 {
		t.Error("hash should not be empty")
	}
}

func TestArgon2_RandomSalt(t *testing.T) {
	a := NewArgon2()
	h1, _ := a.Encrypt("password")
	h2, _ := a.Encrypt("password")
	if h1 == h2 {
		t.Error("same password should produce different hashes due to random salt")
	}
}

func TestArgon2_DifferentPasswords(t *testing.T) {
	a := NewArgon2()
	h1, _ := a.Encrypt("password1")
	h2, _ := a.Encrypt("password2")
	if h1 == h2 {
		t.Error("different passwords should produce different hashes")
	}
}

func TestArgon2_CompareHashAndPassword_Match(t *testing.T) {
	a := NewArgon2()
	hash, _ := a.Encrypt("mypassword")
	if !a.CompareHashAndPassword("mypassword", hash) {
		t.Error("should match with correct password")
	}
}

func TestArgon2_CompareHashAndPassword_Mismatch(t *testing.T) {
	a := NewArgon2()
	hash, _ := a.Encrypt("mypassword")
	if a.CompareHashAndPassword("wrongpassword", hash) {
		t.Error("should not match with wrong password")
	}
}

func TestArgon2_CompareHashAndPassword_InvalidHash(t *testing.T) {
	a := NewArgon2()
	if a.CompareHashAndPassword("password", "!!!invalid!!!") {
		t.Error("should return false for invalid hash")
	}
}

func TestArgon2_CompareHashAndPassword_TooShort(t *testing.T) {
	a := NewArgon2()
	if a.CompareHashAndPassword("password", "c2hvcnQ") {
		t.Error("should return false for hash shorter than salt size")
	}
}

func BenchmarkArgon2_Encrypt(b *testing.B) {
	a := NewArgon2()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.Encrypt("benchmark_password")
	}
}
