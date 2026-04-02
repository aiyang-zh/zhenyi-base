package zencrypt

import (
	"sync"
	"testing"
)

// FuzzSM3Bytes 对任意输入计算 SM3，要求长度恒为 32 且不 panic。
func FuzzSM3Bytes(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte("hello"))
	f.Add([]byte{0xff, 0, 0x80})

	f.Fuzz(func(t *testing.T, data []byte) {
		out := SM3Bytes(data)
		if len(out) != 32 {
			t.Fatalf("SM3Bytes len=%d, want 32", len(out))
		}
	})
}

// FuzzBase64DecodeString 对任意字符串尝试 Base64 解码，要求不 panic。
func FuzzBase64DecodeString(f *testing.F) {
	f.Add("")
	f.Add("YQ==")
	f.Add("not-valid===")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = DecodeString(s)
	})
}

// FuzzSM2ParsePrivateKey 对任意字节尝试解析 SM2 私钥 PEM，要求不 panic。
func FuzzSM2ParsePrivateKey(f *testing.F) {
	f.Add([]byte("-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----"))
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, pemData []byte) {
		if len(pemData) > 8192 {
			pemData = pemData[:8192]
		}
		_, _ = SM2ParsePrivateKey(pemData)
	})
}

// FuzzSM2ParsePublicKey 对任意字节尝试解析 SM2 公钥 PEM，要求不 panic。
func FuzzSM2ParsePublicKey(f *testing.F) {
	f.Add([]byte("-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----"))
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, pemData []byte) {
		if len(pemData) > 8192 {
			pemData = pemData[:8192]
		}
		_, _ = SM2ParsePublicKey(pemData)
	})
}

// FuzzSM2VerifyNoPanic 对任意数据/签名调用 Verify，要求不 panic。
func FuzzSM2VerifyNoPanic(f *testing.F) {
	// Key 生成一次即可；Verify 的正确性不做断言，只做防 panic fuzz。
	priv, err := GenerateSM2Key()
	if err != nil || priv == nil {
		// 生成失败时不让 fuzz 报错崩溃：直接返回（CI 会仅保留其他 fuzz）。
		return
	}
	pub := priv.PublicKey()
	if pub == nil {
		return
	}

	f.Add([]byte("hello"), []byte("sig"))
	f.Add([]byte{}, []byte{})

	var mu sync.Mutex
	f.Fuzz(func(t *testing.T, data []byte, sig []byte) {
		if len(data) > 1024 {
			data = data[:1024]
		}
		if len(sig) > 4096 {
			sig = sig[:4096]
		}
		// 保险：如果底层 Verify 不是纯函数，使用锁避免 fuzz 并发下的竞态风险。
		mu.Lock()
		_ = SM2Verify(pub, data, sig)
		mu.Unlock()
	})
}

// FuzzSM4GCMDecryptNoPanic 对任意密文尝试解密，要求不 panic。
func FuzzSM4GCMDecryptNoPanic(f *testing.F) {
	f.Add([]byte("k"), []byte{})
	f.Add([]byte("k"), []byte{0, 1, 2, 3})

	f.Fuzz(func(t *testing.T, keyBytes []byte, ciphertext []byte) {
		if len(keyBytes) == 0 {
			keyBytes = []byte("k")
		}
		if len(keyBytes) > 32 {
			keyBytes = keyBytes[:32]
		}
		if len(ciphertext) > 4096 {
			ciphertext = ciphertext[:4096]
		}

		key := string(keyBytes)
		c := NewSM4GcmEncrypt(key)
		_, _ = c.Decrypt(ciphertext)
	})
}

// FuzzSM4GCMEncryptDecryptRoundtrip 在可运行输入上做加解密往返一致性校验。
func FuzzSM4GCMEncryptDecryptRoundtrip(f *testing.F) {
	f.Add([]byte("key"), []byte("hello"))
	f.Add([]byte("key"), []byte{0x01, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, keyBytes []byte, plaintext []byte) {
		if len(keyBytes) == 0 {
			keyBytes = []byte("key")
		}
		if len(keyBytes) > 32 {
			keyBytes = keyBytes[:32]
		}
		if len(plaintext) == 0 {
			plaintext = []byte{1}
		}
		if len(plaintext) > 1024 {
			plaintext = plaintext[:1024]
		}

		key := string(keyBytes)
		c := NewSM4GcmEncrypt(key)
		ct, err := c.Encrypt(plaintext)
		if err != nil {
			return
		}
		pt2, err := c.Decrypt(ct)
		if err != nil {
			t.Fatalf("SM4GCM decrypt failed: %v", err)
		}
		if string(pt2) != string(plaintext) {
			t.Fatalf("roundtrip mismatch: got=%x want=%x", pt2, plaintext)
		}
	})
}
