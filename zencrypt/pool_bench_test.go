package zencrypt

import (
	"testing"
)

// 测试数据大小
var testSizes = []int{
	64,    // 小包
	512,   // 中小包
	1024,  // 1KB（典型游戏包）
	4096,  // 4KB（大包）
	16384, // 16KB（超大包）
}

// ============================================
// AES-CBC 基准测试
// ============================================

func BenchmarkAESCBC_Encrypt(b *testing.B) {
	cipher := NewAesEncrypt("test-secret-key-32bytes-long")

	for _, size := range testSizes {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		b.Run(formatSize(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := cipher.Encrypt(plaintext)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAESCBC_Decrypt(b *testing.B) {
	cipher := NewAesEncrypt("test-secret-key-32bytes-long")

	for _, size := range testSizes {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		ciphertext, err := cipher.Encrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(formatSize(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := cipher.Decrypt(ciphertext)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ============================================
// AES-GCM 基准测试
// ============================================

func BenchmarkAESGCM_Encrypt(b *testing.B) {
	cipher := NewAesGcmEncrypt("test-secret-key-32bytes-long")

	for _, size := range testSizes {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		b.Run(formatSize(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := cipher.Encrypt(plaintext)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAESGCM_Decrypt(b *testing.B) {
	cipher := NewAesGcmEncrypt("test-secret-key-32bytes-long")

	for _, size := range testSizes {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		ciphertext, err := cipher.Encrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(formatSize(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := cipher.Decrypt(ciphertext)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ============================================
// 辅助函数
// ============================================

func formatSize(size int) string {
	if size < 1024 {
		return string(rune(size)) + "B"
	}
	return string(rune(size/1024)) + "KB"
}

// ============================================
// 并发基准测试（模拟真实场景）
// ============================================

func BenchmarkAESCBC_Encrypt_Parallel(b *testing.B) {
	cipher := NewAesEncrypt("test-secret-key-32bytes-long")
	plaintext := make([]byte, 1024) // 1KB典型包

	b.SetBytes(1024)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cipher.Encrypt(plaintext)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkAESGCM_Encrypt_Parallel(b *testing.B) {
	cipher := NewAesGcmEncrypt("test-secret-key-32bytes-long")
	plaintext := make([]byte, 1024)

	b.SetBytes(1024)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cipher.Encrypt(plaintext)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
