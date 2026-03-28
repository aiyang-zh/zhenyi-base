package zencrypt

import (
	"bytes"
	"testing"
)

func TestSM2_GenerateKey(t *testing.T) {
	privKey, err := GenerateSM2Key()
	if err != nil {
		t.Fatalf("GenerateSM2Key failed: %v", err)
	}
	if privKey == nil {
		t.Fatal("private key should not be nil")
	}
	pub := privKey.PublicKey()
	if pub == nil {
		t.Fatal("public key should not be nil")
	}
}

func TestSM2_EncryptDecrypt(t *testing.T) {
	privKey, err := GenerateSM2Key()
	if err != nil {
		t.Fatalf("GenerateSM2Key failed: %v", err)
	}

	plaintext := []byte("Hello SM2 国密加密")
	ciphertext, err := SM2Encrypt(privKey.PublicKey(), plaintext)
	if err != nil {
		t.Fatalf("SM2Encrypt failed: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := SM2Decrypt(privKey, ciphertext)
	if err != nil {
		t.Fatalf("SM2Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestSM2_EncryptDecrypt_VariousSizes(t *testing.T) {
	privKey, _ := GenerateSM2Key()

	for _, size := range []int{1, 16, 64, 256, 1024} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		ciphertext, err := SM2Encrypt(privKey.PublicKey(), data)
		if err != nil {
			t.Fatalf("size %d: encrypt failed: %v", size, err)
		}

		decrypted, err := SM2Decrypt(privKey, ciphertext)
		if err != nil {
			t.Fatalf("size %d: decrypt failed: %v", size, err)
		}

		if !bytes.Equal(decrypted, data) {
			t.Fatalf("size %d: round-trip mismatch", size)
		}
	}
}

func TestSM2_EncryptNilKey(t *testing.T) {
	_, err := SM2Encrypt(nil, []byte("data"))
	if err == nil {
		t.Fatal("expected error for nil public key")
	}
}

func TestSM2_EncryptEmptyData(t *testing.T) {
	privKey, _ := GenerateSM2Key()
	_, err := SM2Encrypt(privKey.PublicKey(), nil)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestSM2_DecryptNilKey(t *testing.T) {
	_, err := SM2Decrypt(nil, []byte("data"))
	if err == nil {
		t.Fatal("expected error for nil private key")
	}
}

func TestSM2_SignVerify(t *testing.T) {
	privKey, _ := GenerateSM2Key()

	data := []byte("data to sign 签名测试")
	signature, err := SM2Sign(privKey, data)
	if err != nil {
		t.Fatalf("SM2Sign failed: %v", err)
	}

	if len(signature) == 0 {
		t.Fatal("signature should not be empty")
	}

	err = SM2Verify(privKey.PublicKey(), data, signature)
	if err != nil {
		t.Fatalf("SM2Verify failed: %v", err)
	}
}

func TestSM2_VerifyTampered(t *testing.T) {
	privKey, _ := GenerateSM2Key()

	data := []byte("original data")
	signature, _ := SM2Sign(privKey, data)

	err := SM2Verify(privKey.PublicKey(), []byte("tampered data"), signature)
	if err == nil {
		t.Fatal("expected error for tampered data")
	}
}

func TestSM2_VerifyWrongKey(t *testing.T) {
	privKey1, _ := GenerateSM2Key()
	privKey2, _ := GenerateSM2Key()

	data := []byte("test data")
	signature, _ := SM2Sign(privKey1, data)

	err := SM2Verify(privKey2.PublicKey(), data, signature)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestSM2_SignNilKey(t *testing.T) {
	_, err := SM2Sign(nil, []byte("data"))
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestSM2_VerifyNilKey(t *testing.T) {
	err := SM2Verify(nil, []byte("data"), []byte("sig"))
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestSM2_MarshalUnmarshalPrivateKey(t *testing.T) {
	privKey, _ := GenerateSM2Key()

	pem, err := SM2MarshalPrivateKey(privKey)
	if err != nil {
		t.Fatalf("marshal private key failed: %v", err)
	}

	parsed, err := SM2ParsePrivateKey(pem)
	if err != nil {
		t.Fatalf("parse private key failed: %v", err)
	}

	data := []byte("round-trip test")
	ciphertext, _ := SM2Encrypt(privKey.PublicKey(), data)
	decrypted, err := SM2Decrypt(parsed, ciphertext)
	if err != nil {
		t.Fatalf("decrypt with parsed key failed: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatal("round-trip with marshaled key failed")
	}
}

func TestSM2_MarshalUnmarshalPublicKey(t *testing.T) {
	privKey, _ := GenerateSM2Key()

	pem, err := SM2MarshalPublicKey(privKey.PublicKey())
	if err != nil {
		t.Fatalf("marshal public key failed: %v", err)
	}

	parsed, err := SM2ParsePublicKey(pem)
	if err != nil {
		t.Fatalf("parse public key failed: %v", err)
	}

	data := []byte("public key round-trip")
	ciphertext, _ := SM2Encrypt(parsed, data)
	decrypted, _ := SM2Decrypt(privKey, ciphertext)
	if !bytes.Equal(decrypted, data) {
		t.Fatal("round-trip with marshaled public key failed")
	}
}

func TestSM2_MarshalNilKey(t *testing.T) {
	_, err := SM2MarshalPrivateKey(nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}

	_, err = SM2MarshalPublicKey(nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func BenchmarkSM2_GenerateKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSM2Key()
	}
}

func BenchmarkSM2_Encrypt_64B(b *testing.B) {
	privKey, _ := GenerateSM2Key()
	data := make([]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM2Encrypt(privKey.PublicKey(), data)
	}
}

func BenchmarkSM2_Decrypt_64B(b *testing.B) {
	privKey, _ := GenerateSM2Key()
	data := make([]byte, 64)
	ciphertext, _ := SM2Encrypt(privKey.PublicKey(), data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM2Decrypt(privKey, ciphertext)
	}
}

func BenchmarkSM2_Sign(b *testing.B) {
	privKey, _ := GenerateSM2Key()
	data := make([]byte, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM2Sign(privKey, data)
	}
}

func BenchmarkSM2_Verify(b *testing.B) {
	privKey, _ := GenerateSM2Key()
	data := make([]byte, 256)
	sig, _ := SM2Sign(privKey, data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SM2Verify(privKey.PublicKey(), data, sig)
	}
}
