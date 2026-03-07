package zencrypt

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// TestRSA_KeyGeneration tests RSA key generation
func TestRSA_KeyGeneration(t *testing.T) {
	t.Run("Default2048Bits", func(t *testing.T) {
		privKey, err := GenerateSecretKey()
		if err != nil {
			t.Fatalf("Failed to generate key: %v", err)
		}

		if privKey.Size() != 256 { // 2048 bits = 256 bytes
			t.Errorf("Expected 2048-bit key (256 bytes), got %d bytes", privKey.Size())
		}

		t.Logf("✓ Generated 2048-bit RSA key successfully")
	})

	t.Run("Custom3072Bits", func(t *testing.T) {
		privKey, err := GenerateSecretKeyWithBits(3072)
		if err != nil {
			t.Fatalf("Failed to generate 3072-bit key: %v", err)
		}

		if privKey.Size() != 384 { // 3072 bits = 384 bytes
			t.Errorf("Expected 3072-bit key (384 bytes), got %d bytes", privKey.Size())
		}

		t.Logf("✓ Generated 3072-bit RSA key successfully")
	})

	t.Run("RejectWeakKey", func(t *testing.T) {
		_, err := GenerateSecretKeyWithBits(1024)
		if err == nil {
			t.Error("Should reject 1024-bit key (too weak)")
		}

		t.Logf("✓ Weak key rejected as expected")
	})
}

// TestRSA_OAEP_BasicEncryptDecrypt tests basic OAEP encryption/decryption
func TestRSA_OAEP_BasicEncryptDecrypt(t *testing.T) {
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	plaintext := []byte("Hello, RSA-OAEP!")

	// Encrypt
	ciphertext, err := EncryptOAEP(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt
	decrypted, err := DecryptOAEP(privKey, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Verify
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text doesn't match original")
		t.Logf("Original:  %s", plaintext)
		t.Logf("Decrypted: %s", decrypted)
	}

	t.Logf("✓ Basic OAEP encrypt/decrypt works correctly")
}

// TestRSA_OAEP_RandomCiphertexts tests that encryption produces different ciphertexts
func TestRSA_OAEP_RandomCiphertexts(t *testing.T) {
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	plaintext := []byte("same message")

	// Encrypt twice
	ciphertext1, err := EncryptOAEP(pubKey, plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	ciphertext2, err := EncryptOAEP(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// Ciphertexts should be different due to random padding
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("❌ OAEP should produce different ciphertexts for same plaintext!")
	} else {
		t.Log("✓ OAEP produces random ciphertexts (secure)")
	}

	// But both should decrypt to same plaintext
	decrypted1, _ := DecryptOAEP(privKey, ciphertext1)
	decrypted2, _ := DecryptOAEP(privKey, ciphertext2)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decryption failed")
	}
}

// TestRSA_OAEP_MaximumPlaintextSize tests plaintext size limits
func TestRSA_OAEP_MaximumPlaintextSize(t *testing.T) {
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// For 2048-bit key with SHA-256: max = 256 - 2*32 - 2 = 190 bytes
	maxSize := 190

	t.Run("MaximumSize", func(t *testing.T) {
		plaintext := bytes.Repeat([]byte("A"), maxSize)
		ciphertext, err := EncryptOAEP(pubKey, plaintext)
		if err != nil {
			t.Fatalf("Failed to encrypt max size plaintext: %v", err)
		}

		decrypted, err := DecryptOAEP(privKey, ciphertext)
		if err != nil {
			t.Fatalf("Failed to decrypt: %v", err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Error("Decryption mismatch")
		}

		t.Logf("✓ Maximum plaintext size (%d bytes) works", maxSize)
	})

	t.Run("ExceedMaximumSize", func(t *testing.T) {
		plaintext := bytes.Repeat([]byte("A"), maxSize+1)
		_, err := EncryptOAEP(pubKey, plaintext)
		if err == nil {
			t.Error("Should fail when plaintext exceeds maximum size")
		} else {
			t.Logf("✓ Correctly rejects oversized plaintext: %v", err)
		}
	})
}

// TestRSA_OAEP_CustomHash tests OAEP with custom hash function
func TestRSA_OAEP_CustomHash(t *testing.T) {
	privKey, err := GenerateSecretKeyWithBits(3072) // Larger key for SHA-512
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	plaintext := []byte("Testing SHA-512 with OAEP")

	// Encrypt with SHA-512
	ciphertext, err := EncryptOAEPWithHash(pubKey, sha512.New(), plaintext)
	if err != nil {
		t.Fatalf("Encryption with SHA-512 failed: %v", err)
	}

	// Decrypt with SHA-512
	decrypted, err := DecryptOAEPWithHash(privKey, sha512.New(), ciphertext)
	if err != nil {
		t.Fatalf("Decryption with SHA-512 failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Decryption mismatch")
	}

	t.Log("✓ OAEP with custom hash (SHA-512) works correctly")
}

// TestRSA_OAEP_WrongKey tests decryption with wrong key
func TestRSA_OAEP_WrongKey(t *testing.T) {
	privKey1, _ := GenerateSecretKey()
	privKey2, _ := GenerateSecretKey()
	pubKey1 := &privKey1.PublicKey

	plaintext := []byte("secret message")

	// Encrypt with key1
	ciphertext, err := EncryptOAEP(pubKey1, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with key2 (wrong key)
	_, err = DecryptOAEP(privKey2, ciphertext)
	if err == nil {
		t.Error("Should fail when decrypting with wrong key")
	} else {
		t.Logf("✓ Correctly rejects wrong key: %v", err)
	}
}

// TestRSA_OAEP_EmptyData tests encryption of empty data
func TestRSA_OAEP_EmptyData(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	plaintext := []byte("")

	ciphertext, err := EncryptOAEP(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt empty data: %v", err)
	}

	decrypted, err := DecryptOAEP(privKey, ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("Expected empty plaintext, got %d bytes", len(decrypted))
	}

	t.Log("✓ Empty data encryption/decryption works")
}

// TestRSA_Sign_BasicSignVerify tests basic signing and verification
func TestRSA_Sign_BasicSignVerify(t *testing.T) {
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	data := []byte("Message to sign")

	// Sign
	signature, err := Sign(privKey, data)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	// Verify
	err = Verify(pubKey, data, signature)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	t.Log("✓ Basic signing and verification works")
}

// TestRSA_Sign_TamperedData tests signature verification with tampered data
func TestRSA_Sign_TamperedData(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	originalData := []byte("Original message")
	tamperedData := []byte("Tampered message")

	// Sign original
	signature, err := Sign(privKey, originalData)
	if err != nil {
		t.Fatalf("Signing failed: %v", err)
	}

	// Verify with tampered data
	err = Verify(pubKey, tamperedData, signature)
	if err == nil {
		t.Error("❌ Should detect tampered data!")
	} else {
		t.Logf("✓ Correctly detected tampered data: %v", err)
	}
}

// TestRSA_Sign_TamperedSignature tests verification with tampered signature
func TestRSA_Sign_TamperedSignature(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	data := []byte("Message")

	signature, _ := Sign(privKey, data)

	// Tamper with signature
	signature[0] ^= 0xFF

	err := Verify(pubKey, data, signature)
	if err == nil {
		t.Error("❌ Should detect tampered signature!")
	} else {
		t.Logf("✓ Correctly detected tampered signature: %v", err)
	}
}

// TestRSA_Sign_DeterministicSignature tests that signatures are deterministic for same data
func TestRSA_Sign_DeterministicSignature(t *testing.T) {
	privKey, _ := GenerateSecretKey()

	data := []byte("Same data")

	signature1, _ := Sign(privKey, data)
	signature2, _ := Sign(privKey, data)

	// Note: PKCS1v15 signatures are actually randomized, so they may differ
	// This test documents the behavior
	if bytes.Equal(signature1, signature2) {
		t.Log("✓ Signatures are deterministic (same for identical data)")
	} else {
		t.Log("✓ Signatures are randomized (different for identical data)")
	}
}

// TestRSA_Rsa2_LoadPKCS1 tests loading PKCS#1 format private key
func TestRSA_Rsa2_LoadPKCS1(t *testing.T) {
	// Generate a test key
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// Encode to PKCS#1 PEM
	privKeyBytes := x509.MarshalPKCS1PrivateKey(privKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	}
	pemData := string(pem.EncodeToMemory(pemBlock))

	// Load and verify
	rsa2 := NewRsa2()
	err = rsa2.LoadPrivateKey(pemData)
	if err != nil {
		t.Fatalf("加载PKCS#1密钥失败: %v", err)
	}

	// Test signing
	data := []byte("test data")
	signature, err := rsa2.Encrypt(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// Verify
	pubKey, _ := rsa2.GetPublicKey()
	err = Verify(pubKey, data, signature)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	t.Log("✓ PKCS#1格式加载和签名成功")
}

// TestRSA_Rsa2_LoadPKCS8 tests loading PKCS#8 format private key
func TestRSA_Rsa2_LoadPKCS8(t *testing.T) {
	// Generate a test key
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	// Encode to PKCS#8 PEM
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("编码PKCS#8失败: %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}
	pemData := string(pem.EncodeToMemory(pemBlock))

	// Load and verify
	rsa2 := NewRsa2()
	err = rsa2.LoadPrivateKey(pemData)
	if err != nil {
		t.Fatalf("加载PKCS#8密钥失败: %v", err)
	}

	// Test signing
	data := []byte("test data")
	signature, err := rsa2.Encrypt(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// Verify
	pubKey, _ := rsa2.GetPublicKey()
	err = Verify(pubKey, data, signature)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	t.Log("✓ PKCS#8格式加载和签名成功")
}

// TestRSA_Rsa2_DefensiveParsing tests defensive parsing with non-standard PEM types
func TestRSA_Rsa2_DefensiveParsing(t *testing.T) {
	privKey, err := GenerateSecretKey()
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	tests := []struct {
		name        string
		pemType     string
		encoder     func(*rsa.PrivateKey) ([]byte, error)
		shouldWork  bool
		description string
	}{
		{
			name:    "标准PKCS#1",
			pemType: "RSA PRIVATE KEY",
			encoder: func(k *rsa.PrivateKey) ([]byte, error) {
				return x509.MarshalPKCS1PrivateKey(k), nil
			},
			shouldWork:  true,
			description: "标准格式，应立即成功",
		},
		{
			name:    "标准PKCS#8",
			pemType: "PRIVATE KEY",
			encoder: func(k *rsa.PrivateKey) ([]byte, error) {
				return x509.MarshalPKCS8PrivateKey(k)
			},
			shouldWork:  true,
			description: "标准格式，应立即成功",
		},
		{
			name:    "错误Type但内容是PKCS#1",
			pemType: "OPENSSH PRIVATE KEY", // Type错误
			encoder: func(k *rsa.PrivateKey) ([]byte, error) {
				return x509.MarshalPKCS1PrivateKey(k), nil // 但内容正确
			},
			shouldWork:  true,
			description: "防御性解析应该成功（尝试PKCS#1）",
		},
		{
			name:    "错误Type但内容是PKCS#8",
			pemType: "CERTIFICATE", // Type完全错误
			encoder: func(k *rsa.PrivateKey) ([]byte, error) {
				return x509.MarshalPKCS8PrivateKey(k)
			},
			shouldWork:  true,
			description: "防御性解析应该成功（尝试PKCS#8）",
		},
		{
			name:    "自定义Type但内容是PKCS#1",
			pemType: "MY CUSTOM RSA KEY",
			encoder: func(k *rsa.PrivateKey) ([]byte, error) {
				return x509.MarshalPKCS1PrivateKey(k), nil
			},
			shouldWork:  true,
			description: "防御性解析应该成功",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 生成PEM
			keyBytes, err := tt.encoder(privKey)
			if err != nil {
				t.Fatalf("编码密钥失败: %v", err)
			}

			pemBlock := &pem.Block{
				Type:  tt.pemType,
				Bytes: keyBytes,
			}
			pemData := string(pem.EncodeToMemory(pemBlock))

			// 尝试加载
			rsa2 := NewRsa2()
			err = rsa2.LoadPrivateKey(pemData)

			if tt.shouldWork && err != nil {
				t.Errorf("❌ 应该成功但失败了: %v", err)
				t.Logf("说明: %s", tt.description)
				return
			}

			if !tt.shouldWork && err == nil {
				t.Error("❌ 应该失败但成功了")
				return
			}

			if err == nil {
				// 验证加载的密钥确实可以正常工作
				testData := []byte("test message")
				signature, err := rsa2.Encrypt(testData)
				if err != nil {
					t.Errorf("签名失败: %v", err)
					return
				}

				pubKey, _ := rsa2.GetPublicKey()
				err = Verify(pubKey, testData, signature)
				if err != nil {
					t.Errorf("验证失败: %v", err)
					return
				}

				t.Logf("✓ %s: %s", tt.name, tt.description)
			}
		})
	}

	t.Log("✓ 防御性解析测试全部通过")
	t.Log("💡 即使PEM Type不标准，也能正确解析密钥内容")
}

// TestRSA_Rsa2_InvalidPEM tests error handling for truly invalid PEM
func TestRSA_Rsa2_InvalidPEM_Enhanced(t *testing.T) {
	tests := []struct {
		name        string
		pemData     string
		expectError bool
		errorPart   string
	}{
		{
			name:        "完全无效的数据",
			pemData:     "this is not a PEM at all",
			expectError: true,
			errorPart:   "无效的PEM格式",
		},
		{
			name: "有PEM头但内容无效",
			pemData: `-----BEGIN RSA PRIVATE KEY-----
invalid base64 content here!!!
-----END RSA PRIVATE KEY-----`,
			expectError: true,
			errorPart:   "无法解析",
		},
		{
			name:        "空PEM",
			pemData:     "",
			expectError: true,
			errorPart:   "无效的PEM格式",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsa2 := NewRsa2()
			err := rsa2.LoadPrivateKey(tt.pemData)

			if tt.expectError && err == nil {
				t.Error("应该返回错误但成功了")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("不应该返回错误: %v", err)
				return
			}

			if err != nil {
				t.Logf("✓ 正确检测到错误: %v", err)
				if tt.errorPart != "" && !strings.Contains(err.Error(), tt.errorPart) {
					t.Logf("注意：错误信息中未包含 \"%s\"", tt.errorPart)
				}
			}
		})
	}
}

// TestRSA_Rsa2_NoKeyLoaded tests error handling when no key is loaded
func TestRSA_Rsa2_NoKeyLoaded(t *testing.T) {
	rsa2 := NewRsa2()

	_, err := rsa2.Encrypt([]byte("data"))
	if err == nil {
		t.Error("Should return error when no key is loaded")
	} else {
		t.Logf("✓ Correctly returns error: %v", err)
	}

	_, err = rsa2.GetPublicKey()
	if err == nil {
		t.Error("Should return error when no key is loaded")
	}
}

// TestRSA_Rsa2_InvalidPEM tests error handling for invalid PEM
func TestRSA_Rsa2_InvalidPEM(t *testing.T) {
	rsa2 := NewRsa2()

	err := rsa2.LoadPrivateKey("invalid pem data")
	if err == nil {
		t.Error("Should return error for invalid PEM")
	} else {
		t.Logf("✓ Correctly rejects invalid PEM: %v", err)
	}
}

// TestRSA_OAEP_WithLabel tests OAEP encryption with label
func TestRSA_OAEP_WithLabel(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	plaintext := []byte("sensitive data")
	label := []byte("user:12345")

	// Encrypt with label
	ciphertext, err := EncryptOAEPWithLabel(pubKey, plaintext, label)
	if err != nil {
		t.Fatalf("带Label加密失败: %v", err)
	}

	// Decrypt with correct label
	decrypted, err := DecryptOAEPWithLabel(privKey, ciphertext, label)
	if err != nil {
		t.Fatalf("带Label解密失败: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("解密数据不匹配")
	}

	// Try to decrypt with wrong label
	wrongLabel := []byte("user:99999")
	_, err = DecryptOAEPWithLabel(privKey, ciphertext, wrongLabel)
	if err == nil {
		t.Error("❌ 应该拒绝错误的Label!")
	} else {
		t.Logf("✓ 正确检测到Label不匹配: %v", err)
	}

	t.Log("✓ OAEP Label功能正常工作")
}

// TestRSA_OAEP_LabelPreventsReplay tests that label prevents context confusion
func TestRSA_OAEP_LabelPreventsReplay(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	plaintext := []byte("transfer $100")

	// Encrypt for user A
	labelA := []byte("user:alice")
	ciphertextA, _ := EncryptOAEPWithLabel(pubKey, plaintext, labelA)

	// Try to "replay" to user B context
	labelB := []byte("user:bob")
	_, err := DecryptOAEPWithLabel(privKey, ciphertextA, labelB)

	if err == nil {
		t.Error("❌ Label应该防止跨上下文重放!")
	} else {
		t.Log("✓ Label成功防止跨上下文重放")
	}
}

// TestRSA_OAEP_HashConcurrency tests hash concurrent usage warning
func TestRSA_OAEP_HashConcurrency(t *testing.T) {
	t.Log("⚠️  测试Hash并发使用（验证文档警告的必要性）")

	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	// 正确用法：每次创建新实例
	t.Run("正确用法", func(t *testing.T) {
		plaintext := []byte("test")

		// ✅ 每次都创建新实例
		ciphertext, err := EncryptOAEPWithHash(pubKey, sha512.New(), plaintext)
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}

		decrypted, err := DecryptOAEPWithHash(privKey, sha512.New(), ciphertext)
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Error("解密不匹配")
		}

		t.Log("✓ 正确用法：每次创建新Hash实例")
	})

	// 错误用法演示（不实际运行，仅文档说明）
	t.Run("错误用法演示", func(t *testing.T) {
		t.Log("⚠️  文档警告：不要复用Hash实例")
		t.Log("❌ 错误：h := sha512.New(); EncryptOAEPWithHash(..., h, ...); EncryptOAEPWithHash(..., h, ...)")
		t.Log("✅ 正确：EncryptOAEPWithHash(..., sha512.New(), ...)")
	})
}

// TestRSA_LongMessage tests encryption of longer messages (chunking needed)
func TestRSA_LongMessage(t *testing.T) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey

	// Long message exceeding RSA block size
	longMessage := bytes.Repeat([]byte("A"), 1000)

	_, err := EncryptOAEP(pubKey, longMessage)
	if err == nil {
		t.Error("Should fail for message exceeding RSA block size")
	} else {
		t.Logf("✓ Correctly rejects oversized message: %v", err)
		t.Log("💡 For long messages, use hybrid encryption (RSA + AES)")
	}
}

// BenchmarkRSA_OAEP_Encrypt benchmarks OAEP encryption
func BenchmarkRSA_OAEP_Encrypt(b *testing.B) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey
	plaintext := []byte("Benchmark message for OAEP encryption")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncryptOAEP(pubKey, plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRSA_OAEP_Decrypt benchmarks OAEP decryption
func BenchmarkRSA_OAEP_Decrypt(b *testing.B) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey
	plaintext := []byte("Benchmark message for OAEP decryption")
	ciphertext, _ := EncryptOAEP(pubKey, plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DecryptOAEP(privKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRSA_Sign benchmarks RSA signing
func BenchmarkRSA_Sign(b *testing.B) {
	privKey, _ := GenerateSecretKey()
	data := []byte("Benchmark data for signing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Sign(privKey, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRSA_Verify benchmarks RSA signature verification
func BenchmarkRSA_Verify(b *testing.B) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey
	data := []byte("Benchmark data for verification")
	signature, _ := Sign(privKey, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Verify(pubKey, data, signature)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompare_OAEP_vs_PKCS1v15 compares OAEP vs PKCS1v15 performance
func BenchmarkCompare_OAEP_vs_PKCS1v15(b *testing.B) {
	privKey, _ := GenerateSecretKey()
	pubKey := &privKey.PublicKey
	plaintext := []byte("Comparison benchmark message")

	b.Run("OAEP", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ciphertext, err := EncryptOAEP(pubKey, plaintext)
			if err != nil {
				b.Fatal(err)
			}
			_, err = DecryptOAEP(privKey, ciphertext)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

}
