package zencrypt

import (
	"bytes"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"testing"
)

// ============================================================
// BaseEncrypt (enctypt.go)
// ============================================================

func TestBaseEncrypt_EncryptPassthrough(t *testing.T) {
	var enc ziface.IEncrypt = &BaseEncrypt{}
	data := []byte("hello world")
	result, err := enc.Encrypt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, result) {
		t.Error("BaseEncrypt.Encrypt should return data unchanged")
	}
}

func TestBaseEncrypt_DecryptPassthrough(t *testing.T) {
	var enc ziface.IEncrypt = &BaseEncrypt{}
	data := []byte("hello world")
	result, err := enc.Decrypt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data, result) {
		t.Error("BaseEncrypt.Decrypt should return data unchanged")
	}
}

func TestBaseEncrypt_RoundTrip(t *testing.T) {
	var enc ziface.IEncrypt = &BaseEncrypt{}
	original := []byte{0x00, 0xFF, 0x7F, 0x80}
	encrypted, _ := enc.Encrypt(original)
	decrypted, _ := enc.Decrypt(encrypted)
	if !bytes.Equal(original, decrypted) {
		t.Error("round trip should preserve data")
	}
}

func TestBaseEncrypt_NilData(t *testing.T) {
	var enc ziface.IEncrypt = &BaseEncrypt{}
	result, err := enc.Encrypt(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("nil input should return nil")
	}
}

func BenchmarkBaseEncrypt_Encrypt(b *testing.B) {
	enc := &BaseEncrypt{}
	data := make([]byte, 1024)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		enc.Encrypt(data)
	}
}
