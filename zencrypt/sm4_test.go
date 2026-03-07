package zencrypt

import (
	"bytes"
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"testing"
)

func TestSM4CBC_EncryptDecrypt(t *testing.T) {
	cipher := NewSM4Encrypt("test-key-12345")

	plaintext := []byte("Hello SM4 国密加密")
	encrypted, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted should differ from plaintext")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestSM4CBC_DifferentIV(t *testing.T) {
	cipher := NewSM4Encrypt("test-key")
	plaintext := []byte("same data")

	c1, _ := cipher.Encrypt(plaintext)
	c2, _ := cipher.Encrypt(plaintext)

	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions of the same data should produce different ciphertexts (random IV)")
	}
}

func TestSM4CBC_EmptyData(t *testing.T) {
	cipher := NewSM4Encrypt("key")
	_, err := cipher.Encrypt(nil)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestSM4CBC_ShortCiphertext(t *testing.T) {
	cipher := NewSM4Encrypt("key")
	_, err := cipher.Decrypt([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestSM4CBC_TamperedData(t *testing.T) {
	cipher := NewSM4Encrypt("key")
	plaintext := []byte("important data here!!")
	encrypted, _ := cipher.Encrypt(plaintext)

	encrypted[len(encrypted)-1] ^= 0xff

	_, err := cipher.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error for tampered data")
	}
}

func TestSM4GCM_EncryptDecrypt(t *testing.T) {
	cipher := NewSM4GcmEncrypt("test-key-12345")

	plaintext := []byte("Hello SM4-GCM 国密加密")
	encrypted, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted should differ from plaintext")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestSM4GCM_DifferentNonce(t *testing.T) {
	cipher := NewSM4GcmEncrypt("test-key")
	plaintext := []byte("same data")

	c1, _ := cipher.Encrypt(plaintext)
	c2, _ := cipher.Encrypt(plaintext)

	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions should produce different ciphertexts (random nonce)")
	}
}

func TestSM4GCM_EmptyData(t *testing.T) {
	cipher := NewSM4GcmEncrypt("key")
	_, err := cipher.Encrypt(nil)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestSM4GCM_TamperedData(t *testing.T) {
	cipher := NewSM4GcmEncrypt("key")
	plaintext := []byte("important data")
	encrypted, _ := cipher.Encrypt(plaintext)

	encrypted[len(encrypted)-1] ^= 0xff

	_, err := cipher.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error for tampered GCM data")
	}
}

func TestSM4GCM_ShortCiphertext(t *testing.T) {
	cipher := NewSM4GcmEncrypt("key")
	_, err := cipher.Decrypt([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestSM4_IEncryptInterface(t *testing.T) {
	var _ ziface.IEncrypt = &SM4Encrypt{}
	var _ ziface.IEncrypt = &SM4GcmEncrypt{}
}

func TestSM4CBC_VariousDataSizes(t *testing.T) {
	cipher := NewSM4Encrypt("key")
	for _, size := range []int{1, 15, 16, 17, 64, 256, 1024, 4096} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		encrypted, err := cipher.Encrypt(data)
		if err != nil {
			t.Fatalf("size %d: Encrypt failed: %v", size, err)
		}
		decrypted, err := cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("size %d: Decrypt failed: %v", size, err)
		}
		if !bytes.Equal(decrypted, data) {
			t.Fatalf("size %d: round-trip mismatch", size)
		}
	}
}

func TestSM4GCM_VariousDataSizes(t *testing.T) {
	cipher := NewSM4GcmEncrypt("key")
	for _, size := range []int{1, 15, 16, 17, 64, 256, 1024, 4096} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		encrypted, err := cipher.Encrypt(data)
		if err != nil {
			t.Fatalf("size %d: Encrypt failed: %v", size, err)
		}
		decrypted, err := cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("size %d: Decrypt failed: %v", size, err)
		}
		if !bytes.Equal(decrypted, data) {
			t.Fatalf("size %d: round-trip mismatch", size)
		}
	}
}

func BenchmarkSM4CBC_Encrypt_64B(b *testing.B) {
	cipher := NewSM4Encrypt("bench-key")
	data := make([]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Encrypt(data)
	}
}

func BenchmarkSM4CBC_Decrypt_64B(b *testing.B) {
	cipher := NewSM4Encrypt("bench-key")
	data := make([]byte, 64)
	encrypted, _ := cipher.Encrypt(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Decrypt(encrypted)
	}
}

func BenchmarkSM4GCM_Encrypt_64B(b *testing.B) {
	cipher := NewSM4GcmEncrypt("bench-key")
	data := make([]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Encrypt(data)
	}
}

func BenchmarkSM4GCM_Decrypt_64B(b *testing.B) {
	cipher := NewSM4GcmEncrypt("bench-key")
	data := make([]byte, 64)
	encrypted, _ := cipher.Encrypt(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Decrypt(encrypted)
	}
}

func BenchmarkSM4GCM_Encrypt_512B(b *testing.B) {
	cipher := NewSM4GcmEncrypt("bench-key")
	data := make([]byte, 512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cipher.Encrypt(data)
	}
}
