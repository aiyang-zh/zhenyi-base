package zencrypt

import (
	"bytes"
	"testing"
)

// TestAesEncrypt_BasicEncryptDecrypt 测试基本的加解密功能
func TestAesEncrypt_BasicEncryptDecrypt(t *testing.T) {
	aes := NewAesEncrypt("test-secret-key-123")

	plaintext := []byte("Hello, World! This is a test message.")

	// 加密
	ciphertext, err := aes.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 解密
	decrypted, err := aes.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data mismatch.\nOriginal: %s\nDecrypted: %s", plaintext, decrypted)
	}
}

// TestAesEncrypt_RandomIV 测试IV随机性（关键安全特性）
func TestAesEncrypt_RandomIV(t *testing.T) {
	aes := NewAesEncrypt("test-key")
	plaintext := []byte("same message")

	// 加密两次相同的消息
	ciphertext1, _ := aes.Encrypt(plaintext)
	ciphertext2, _ := aes.Encrypt(plaintext)

	// 密文应该不同（因为IV不同）
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Same plaintext produced identical ciphertext - IV not random!")
	}

	// 但解密后都应该得到相同的明文
	decrypted1, _ := aes.Decrypt(ciphertext1)
	decrypted2, _ := aes.Decrypt(ciphertext2)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decryption failed for random IV test")
	}
}

// TestAesEncrypt_InvalidPadding 测试无效填充的检测
func TestAesEncrypt_InvalidPadding(t *testing.T) {
	aes := NewAesEncrypt("test-key")

	t.Run("padding length invalid", func(t *testing.T) {
		// 构造填充长度非法的数据（32字节：16字节IV + 16字节数据）
		invalidData := make([]byte, 32)
		copy(invalidData[:16], []byte("1234567890123456"))
		copy(invalidData[16:], []byte("invalid-padding!")) // 最后字节是'!'(33)，超过16

		_, err := aes.Decrypt(invalidData)
		if err == nil {
			t.Error("Expected error for invalid padding length, got nil")
		}
		t.Logf("Got expected error for invalid length: %v", err)
	})

	t.Run("padding content inconsistent", func(t *testing.T) {
		// 构造填充内容不一致的数据
		// 最后一个字节是3，但倒数第二、三个字节不是3
		invalidData := make([]byte, 32)
		copy(invalidData[:16], []byte("1234567890123456")) // IV
		copy(invalidData[16:29], []byte("some data here")) // 13字节数据
		invalidData[29] = 0x03                             // 声称填充3字节
		invalidData[30] = 0x02                             // 但内容不是3
		invalidData[31] = 0x03

		_, err := aes.Decrypt(invalidData)
		if err == nil {
			t.Error("Expected error for inconsistent padding content, got nil")
		}
		t.Logf("Got expected error for inconsistent content: %v", err)
	})
}

// TestAesEncrypt_EmptyData 测试空数据加密
func TestAesEncrypt_EmptyData(t *testing.T) {
	aes := NewAesEncrypt("test-key")

	_, err := aes.Encrypt([]byte{})
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// TestAesEncrypt_ShortCiphertext 测试过短的密文
func TestAesEncrypt_ShortCiphertext(t *testing.T) {
	aes := NewAesEncrypt("test-key")

	// 密文少于16字节（AES块大小）
	shortData := []byte("short")

	_, err := aes.Decrypt(shortData)
	if err == nil {
		t.Error("Expected error for short ciphertext, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// TestAesEncrypt_InvalidBlockSize 测试非块大小倍数的密文
func TestAesEncrypt_InvalidBlockSize(t *testing.T) {
	aes := NewAesEncrypt("test-key")

	// 构造长度不是16的倍数的数据（17字节：16字节IV + 1字节数据）
	invalidData := make([]byte, 17)

	_, err := aes.Decrypt(invalidData)
	if err == nil {
		t.Error("Expected error for invalid block size, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// TestAesEncrypt_LongMessage 测试长消息加解密
func TestAesEncrypt_LongMessage(t *testing.T) {
	aes := NewAesEncrypt("long-test-key")

	// 创建1KB的数据
	plaintext := bytes.Repeat([]byte("A"), 1024)

	ciphertext, err := aes.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt long message failed: %v", err)
	}

	decrypted, err := aes.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt long message failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Long message decryption mismatch")
	}
}

// TestAesEncrypt_DifferentKeys 测试不同密钥
func TestAesEncrypt_DifferentKeys(t *testing.T) {
	plaintext := []byte("secret message")

	// 使用key1加密
	aes1 := NewAesEncrypt("key-1")
	ciphertext, err := aes1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with key1 failed: %v", err)
	}

	// 尝试使用key2解密（应该失败）
	aes2 := NewAesEncrypt("key-2")
	decrypted, err := aes2.Decrypt(ciphertext)

	// 可能会解密"成功"但得到错误的数据，或者在填充验证时失败
	if err == nil && bytes.Equal(plaintext, decrypted) {
		t.Error("Different key should not decrypt correctly")
	}
}

// TestAesEncrypt_UnicodeData 测试Unicode数据
func TestAesEncrypt_UnicodeData(t *testing.T) {
	aes := NewAesEncrypt("unicode-test-key")

	plaintext := []byte("你好世界！Hello World! 🌍🔐")

	ciphertext, err := aes.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt unicode failed: %v", err)
	}

	decrypted, err := aes.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt unicode failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Unicode data mismatch")
	}
}

// TestAesEncrypt_BinaryData 测试二进制数据
func TestAesEncrypt_BinaryData(t *testing.T) {
	aes := NewAesEncrypt("binary-test-key")

	// 创建包含所有字节值的数据
	plaintext := make([]byte, 256)
	for i := 0; i < 256; i++ {
		plaintext[i] = byte(i)
	}

	ciphertext, err := aes.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt binary failed: %v", err)
	}

	decrypted, err := aes.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt binary failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Binary data mismatch")
	}
}

// TestPKCS7Padding 测试PKCS7填充
func TestPKCS7Padding(t *testing.T) {
	aes := NewAesEncrypt("test")

	testCases := []struct {
		name      string
		data      []byte
		blockSize int
		wantLen   int
	}{
		{"empty", []byte{}, 16, 16},
		{"one byte", []byte{1}, 16, 16},
		{"15 bytes", bytes.Repeat([]byte{1}, 15), 16, 16},
		{"16 bytes", bytes.Repeat([]byte{1}, 16), 16, 32},
		{"17 bytes", bytes.Repeat([]byte{1}, 17), 16, 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			padded := aes.PKCS7Padding(tc.data, tc.blockSize)
			if len(padded) != tc.wantLen {
				t.Errorf("Padding length mismatch: got %d, want %d", len(padded), tc.wantLen)
			}
			// 验证填充值正确
			paddingLen := len(padded) - len(tc.data)
			for i := len(tc.data); i < len(padded); i++ {
				if padded[i] != byte(paddingLen) {
					t.Errorf("Invalid padding byte at %d: got %d, want %d", i, padded[i], paddingLen)
				}
			}
		})
	}
}

// TestPKCS7UnPaddingWithValidation 测试带验证的去填充
func TestPKCS7UnPaddingWithValidation(t *testing.T) {
	aes := NewAesEncrypt("test")

	testCases := []struct {
		name      string
		data      []byte
		shouldErr bool
	}{
		{"valid 1 byte padding", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 1}, false},
		{"valid 2 byte padding", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 2, 2}, false},
		{"valid 16 byte padding", bytes.Repeat([]byte{16}, 16), false},
		{"invalid padding value", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 99}, true},
		{"inconsistent padding", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 3, 2}, true},
		{"zero padding", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 0}, true},
		{"empty data", []byte{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := aes.PKCS7UnPaddingWithValidation(tc.data)
			if tc.shouldErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// BenchmarkAesEncrypt 性能基准测试
func BenchmarkAesEncrypt(b *testing.B) {
	aes := NewAesEncrypt("benchmark-key")
	data := bytes.Repeat([]byte("A"), 1024) // 1KB数据

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := aes.Encrypt(data)
		if err != nil {
			b.Fatal(err) // 防止测的是报错的速度
		}
	}
}

// BenchmarkAesDecrypt 解密性能基准测试
func BenchmarkAesDecrypt(b *testing.B) {
	aes := NewAesEncrypt("benchmark-key")
	data := bytes.Repeat([]byte("A"), 1024)
	ciphertext, err := aes.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := aes.Decrypt(ciphertext)
		if err != nil {
			b.Fatal(err) // 确保测的是正常解密而不是错误处理
		}
	}
}

// ====================================
// AES-GCM 模式测试（推荐）
// ====================================

// TestAesGcmEncrypt_BasicEncryptDecrypt 测试GCM基本加解密
func TestAesGcmEncrypt_BasicEncryptDecrypt(t *testing.T) {
	gcm := NewAesGcmEncrypt("test-gcm-key")

	plaintext := []byte("Hello, GCM! This is authenticated encryption.")

	// 加密
	ciphertext, err := gcm.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("GCM Encrypt failed: %v", err)
	}

	// 解密
	decrypted, err := gcm.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("GCM Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("GCM decrypted data mismatch")
	}
}

// TestAesGcmEncrypt_RandomNonce 测试GCM的Nonce随机性
func TestAesGcmEncrypt_RandomNonce(t *testing.T) {
	gcm := NewAesGcmEncrypt("test-key")
	plaintext := []byte("same message")

	// 加密两次相同的消息
	ciphertext1, _ := gcm.Encrypt(plaintext)
	ciphertext2, _ := gcm.Encrypt(plaintext)

	// 密文应该不同（因为Nonce不同）
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("GCM: Same plaintext produced identical ciphertext - Nonce not random!")
	}

	// 但都能正确解密
	decrypted1, _ := gcm.Decrypt(ciphertext1)
	decrypted2, _ := gcm.Decrypt(ciphertext2)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("GCM decryption failed")
	}
}

// TestAesGcmEncrypt_TamperDetection 测试GCM的防篡改能力（关键特性）
func TestAesGcmEncrypt_TamperDetection(t *testing.T) {
	gcm := NewAesGcmEncrypt("test-key")
	plaintext := []byte("important data")

	ciphertext, err := gcm.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 篡改密文中的一个字节
	if len(ciphertext) > 20 {
		ciphertext[20] ^= 0xFF // 翻转一个字节
	}

	// 解密应该失败（检测到篡改）
	_, err = gcm.Decrypt(ciphertext)
	if err == nil {
		t.Error("GCM should detect tampering but didn't!")
	}
	t.Logf("Successfully detected tampering: %v", err)
}

// TestAesGcmEncrypt_EmptyData 测试GCM空数据
func TestAesGcmEncrypt_EmptyData(t *testing.T) {
	gcm := NewAesGcmEncrypt("test-key")

	_, err := gcm.Encrypt([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

// TestAesGcmEncrypt_ShortCiphertext 测试GCM过短密文
func TestAesGcmEncrypt_ShortCiphertext(t *testing.T) {
	gcm := NewAesGcmEncrypt("test-key")

	// 密文少于nonce大小（12字节）
	shortData := []byte("short")

	_, err := gcm.Decrypt(shortData)
	if err == nil {
		t.Error("Expected error for short ciphertext")
	}
}

// TestAesGcmEncrypt_LongMessage 测试GCM长消息
func TestAesGcmEncrypt_LongMessage(t *testing.T) {
	gcm := NewAesGcmEncrypt("long-test-key")

	// 创建10KB的数据
	plaintext := bytes.Repeat([]byte("A"), 10*1024)

	ciphertext, err := gcm.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("GCM Encrypt long message failed: %v", err)
	}

	decrypted, err := gcm.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("GCM Decrypt long message failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("GCM long message decryption mismatch")
	}
}

// TestAesGcmEncrypt_DifferentKeys 测试GCM不同密钥
func TestAesGcmEncrypt_DifferentKeys(t *testing.T) {
	plaintext := []byte("secret message")

	// 使用key1加密
	gcm1 := NewAesGcmEncrypt("key-1")
	ciphertext, err := gcm1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with key1 failed: %v", err)
	}

	// 尝试使用key2解密（应该失败）
	gcm2 := NewAesGcmEncrypt("key-2")
	_, err = gcm2.Decrypt(ciphertext)

	if err == nil {
		t.Error("Different key should not decrypt correctly")
	}
	t.Logf("Correctly rejected wrong key: %v", err)
}

// TestAesGcmEncrypt_UnicodeData 测试GCM Unicode数据
func TestAesGcmEncrypt_UnicodeData(t *testing.T) {
	gcm := NewAesGcmEncrypt("unicode-key")

	plaintext := []byte("你好世界！Hello World! 🌍🔐")

	ciphertext, err := gcm.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("GCM Encrypt unicode failed: %v", err)
	}

	decrypted, err := gcm.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("GCM Decrypt unicode failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("GCM Unicode data mismatch")
	}
}

// TestCompare_CBC_vs_GCM 对比CBC和GCM
func TestCompare_CBC_vs_GCM(t *testing.T) {
	plaintext := []byte("Compare CBC and GCM modes")

	// CBC加密
	cbc := NewAesEncrypt("test-key")
	cbcCiphertext, _ := cbc.Encrypt(plaintext)

	// GCM加密
	gcm := NewAesGcmEncrypt("test-key")
	gcmCiphertext, _ := gcm.Encrypt(plaintext)

	t.Logf("Plaintext length: %d bytes", len(plaintext))
	t.Logf("CBC ciphertext length: %d bytes (IV + padded data)", len(cbcCiphertext))
	t.Logf("GCM ciphertext length: %d bytes (Nonce + data + tag)", len(gcmCiphertext))

	// 密文长度公式：
	// CBC = 16(IV) + (len/16 + 1)*16  (PKCS7总是至少填充1个块)
	// GCM = 12(Nonce) + len + 16(tag) (无需填充)
	// 例：26字节明文 -> CBC=64字节, GCM=54字节
}

// BenchmarkAesGcmEncrypt GCM加密性能基准测试
func BenchmarkAesGcmEncrypt(b *testing.B) {
	gcm := NewAesGcmEncrypt("benchmark-key")
	data := bytes.Repeat([]byte("A"), 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gcm.Encrypt(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAesGcmDecrypt GCM解密性能基准测试
func BenchmarkAesGcmDecrypt(b *testing.B) {
	gcm := NewAesGcmEncrypt("benchmark-key")
	data := bytes.Repeat([]byte("A"), 1024)
	ciphertext, err := gcm.Encrypt(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gcm.Decrypt(ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompare_CBC_vs_GCM_Encrypt 对比CBC和GCM加密性能
func BenchmarkCompare_CBC_vs_GCM_Encrypt(b *testing.B) {
	data := bytes.Repeat([]byte("A"), 1024)

	b.Run("CBC", func(b *testing.B) {
		cbc := NewAesEncrypt("key")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := cbc.Encrypt(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GCM", func(b *testing.B) {
		gcm := NewAesGcmEncrypt("key")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := gcm.Encrypt(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
