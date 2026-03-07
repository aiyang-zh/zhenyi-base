package zencrypt

import (
	"bytes"
	"testing"
)

// ============================================================
// Base64 (base64.go)
// ============================================================

func TestBase64_RoundTrip(t *testing.T) {
	data := []byte("hello world 你好世界")
	encoded := EncodeToString(data)
	decoded, err := DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(data, decoded) {
		t.Errorf("round trip failed: got %s", decoded)
	}
}

func TestBase64_EmptyData(t *testing.T) {
	encoded := EncodeToString([]byte{})
	decoded, err := DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected empty, got %v", decoded)
	}
}

func TestBase64_BinaryData(t *testing.T) {
	data := []byte{0x00, 0xFF, 0x7F, 0x80, 0x01}
	encoded := EncodeToString(data)
	decoded, err := DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(data, decoded) {
		t.Error("binary round trip failed")
	}
}

func TestBase64_InvalidInput(t *testing.T) {
	_, err := DecodeString("!!!invalid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func BenchmarkBase64_Encode(b *testing.B) {
	data := make([]byte, 1024)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeToString(data)
	}
}

func BenchmarkBase64_Decode(b *testing.B) {
	data := make([]byte, 1024)
	encoded := EncodeToString(data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DecodeString(encoded)
	}
}
