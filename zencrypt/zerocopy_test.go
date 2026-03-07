package zencrypt

import (
	"bytes"
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"sync"
	"testing"
)

// TestZeroCopyEncrypt_Basic 测试基本加密解密
func TestZeroCopyEncrypt_Basic(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := []byte("Hello, ZeroCopy World!")

	// 加密
	encBuf, err := cipher.EncryptZeroCopy(plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	defer encBuf.Release()

	if len(encBuf.B) == 0 {
		t.Fatal("加密结果为空")
	}

	// 解密
	decBuf, err := cipher.DecryptZeroCopy(encBuf.B)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	defer decBuf.Release()

	// 验证
	if !bytes.Equal(decBuf.B, plaintext) {
		t.Errorf("解密结果不匹配\n期望: %s\n实际: %s", plaintext, decBuf.B)
	}
}

// TestZeroCopyEncrypt_Empty 测试空数据
func TestZeroCopyEncrypt_Empty(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	_, err := cipher.EncryptZeroCopy(nil)
	if err == nil {
		t.Error("空数据应该返回错误")
	}

	_, err = cipher.EncryptZeroCopy([]byte{})
	if err == nil {
		t.Error("空切片应该返回错误")
	}
}

// TestZeroCopyDecrypt_Invalid 测试无效密文
func TestZeroCopyDecrypt_Invalid(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	// 密文太短
	_, err := cipher.DecryptZeroCopy([]byte("short"))
	if err == nil {
		t.Error("短密文应该返回错误")
	}

	// 无效密文
	invalidCipher := make([]byte, 50)
	_, err = cipher.DecryptZeroCopy(invalidCipher)
	if err == nil {
		t.Error("无效密文应该返回错误")
	}
}

// TestZeroCopyEncrypt_DifferentSizes 测试不同大小数据
func TestZeroCopyEncrypt_DifferentSizes(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	sizes := []int{1, 10, 100, 1024, 4096, 8192}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Size-%d", size), func(t *testing.T) {
			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			encBuf, err := cipher.EncryptZeroCopy(plaintext)
			if err != nil {
				t.Fatalf("加密失败: %v", err)
			}
			defer encBuf.Release()

			decBuf, err := cipher.DecryptZeroCopy(encBuf.B)
			if err != nil {
				t.Fatalf("解密失败: %v", err)
			}
			defer decBuf.Release()

			if !bytes.Equal(decBuf.B, plaintext) {
				t.Errorf("大小 %d 的数据解密失败", size)
			}
		})
	}
}

// TestZeroCopyEncrypt_PoolUsage 测试对象池使用情况
func TestZeroCopyEncrypt_PoolUsage(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := make([]byte, 1024)

	// 记录初始统计
	initialStats := zpool.GetStats()

	// 执行加密解密（正确释放）
	iterations := 100
	for i := 0; i < iterations; i++ {
		encBuf, err := cipher.EncryptZeroCopy(plaintext)
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}

		decBuf, err := cipher.DecryptZeroCopy(encBuf.B)
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}

		decBuf.Release()
		encBuf.Release()
	}

	// 检查统计
	finalStats := zpool.GetStats()

	// 计算所有 bucket 的总和
	var getTimes, putTimes uint64
	for i := range finalStats.BucketStats {
		getTimes += finalStats.BucketStats[i].GetRequests - initialStats.BucketStats[i].GetRequests
		putTimes += finalStats.BucketStats[i].PutReturns - initialStats.BucketStats[i].PutReturns
	}

	t.Logf("对象池使用统计:")
	t.Logf("  Get 次数: %d", getTimes)
	t.Logf("  Put 次数: %d", putTimes)
	if getTimes > 0 {
		t.Logf("  复用率: %.2f%%", float64(putTimes)/float64(getTimes)*100)

		// 验证复用率（应该接近 100%）
		if putTimes < getTimes*90/100 {
			t.Errorf("对象池复用率过低: %.2f%%", float64(putTimes)/float64(getTimes)*100)
		}
	}
}

// TestZeroCopyEncrypt_Concurrent 测试并发安全
func TestZeroCopyEncrypt_Concurrent(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := []byte("concurrent test data")

	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			encBuf, err := cipher.EncryptZeroCopy(plaintext)
			if err != nil {
				t.Errorf("goroutine %d 加密失败: %v", idx, err)
				return
			}
			defer encBuf.Release()

			decBuf, err := cipher.DecryptZeroCopy(encBuf.B)
			if err != nil {
				t.Errorf("goroutine %d 解密失败: %v", idx, err)
				return
			}
			defer decBuf.Release()

			if !bytes.Equal(decBuf.B, plaintext) {
				t.Errorf("goroutine %d 数据不匹配", idx)
			}
		}(i)
	}

	wg.Wait()
}

// TestEncryptInPlace_Basic 测试原地加密
func TestEncryptInPlace_Basic(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := []byte("Hello, InPlace!")

	// 预分配足够大的缓冲（nonce(12) + plaintext + tag(16)）
	dst := make([]byte, 0, 12+len(plaintext)+16+100) // 多留一些空间

	encrypted, err := cipher.EncryptInPlace(dst, plaintext)
	if err != nil {
		t.Fatalf("原地加密失败: %v", err)
	}

	// 验证可以解密
	decBuf, err := cipher.DecryptZeroCopy(encrypted)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	defer decBuf.Release()

	if !bytes.Equal(decBuf.B, plaintext) {
		t.Errorf("解密结果不匹配\n期望: %s\n实际: %s", plaintext, decBuf.B)
	}
}

// TestEncryptInPlace_Empty 测试原地加密空数据
func TestEncryptInPlace_Empty(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	dst := make([]byte, 0, 100)

	_, err := cipher.EncryptInPlace(dst, nil)
	if err == nil {
		t.Error("空数据应该返回错误")
	}
}

// TestEncryptInPlace_SmallBuffer 测试缓冲区过小
func TestEncryptInPlace_SmallBuffer(t *testing.T) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := []byte("test data")

	// 故意分配过小的缓冲
	dst := make([]byte, 0, 5)

	_, err := cipher.EncryptInPlace(dst, plaintext)
	if err == nil {
		t.Error("缓冲区过小应该返回错误")
	}

	expectedErr := "dst buffer too small"
	if err.Error() != expectedErr {
		t.Errorf("期望错误: %s, 实际: %v", expectedErr, err)
	}
}

// TestZeroCopyEncrypt_vs_Standard 对比零拷贝和标准版性能
func TestZeroCopyEncrypt_vs_Standard(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能对比测试")
	}

	stdCipher := NewAesGcmEncrypt("test-key-12345")
	zeroCipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	plaintext := make([]byte, 1024)

	// 预热
	for i := 0; i < 10; i++ {
		stdCipher.Encrypt(plaintext)
		enc, _ := zeroCipher.EncryptZeroCopy(plaintext)
		enc.Release()
	}

	// 测试标准版
	iterations := 10000
	start := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < iterations; i++ {
			_, _ = stdCipher.Encrypt(plaintext)
		}
	})

	// 测试零拷贝版
	zero := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < iterations; i++ {
			enc, _ := zeroCipher.EncryptZeroCopy(plaintext)
			enc.Release()
		}
	})

	t.Logf("标准版: %d ns/op", start.NsPerOp())
	t.Logf("零拷贝: %d ns/op", zero.NsPerOp())
	if zero.NsPerOp() < start.NsPerOp() {
		improvement := float64(start.NsPerOp()-zero.NsPerOp()) / float64(start.NsPerOp()) * 100
		t.Logf("性能提升: %.2f%%", improvement)
	}
}

// BenchmarkZeroCopyEncrypt 零拷贝加密基准测试
func BenchmarkZeroCopyEncrypt(b *testing.B) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		plaintext := make([]byte, size)

		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				enc, _ := cipher.EncryptZeroCopy(plaintext)
				enc.Release()
			}
		})
	}
}

// BenchmarkZeroCopyDecrypt 零拷贝解密基准测试
func BenchmarkZeroCopyDecrypt(b *testing.B) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")

	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		plaintext := make([]byte, size)
		encBuf, _ := cipher.EncryptZeroCopy(plaintext)

		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				dec, _ := cipher.DecryptZeroCopy(encBuf.B)
				dec.Release()
			}
		})

		encBuf.Release()
	}
}

// BenchmarkEncryptInPlace 原地加密基准测试
func BenchmarkEncryptInPlace(b *testing.B) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := make([]byte, 1024)
	dst := make([]byte, 0, 2048)

	b.SetBytes(1024)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		_, _ = cipher.EncryptInPlace(dst, plaintext)
	}
}

// BenchmarkZeroCopy_vs_Standard 对比基准测试
func BenchmarkZeroCopy_vs_Standard(b *testing.B) {
	stdCipher := NewAesGcmEncrypt("test-key-12345")
	zeroCipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := make([]byte, 1024)

	b.Run("Standard-Encrypt", func(b *testing.B) {
		b.SetBytes(1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = stdCipher.Encrypt(plaintext)
		}
	})

	b.Run("ZeroCopy-Encrypt", func(b *testing.B) {
		b.SetBytes(1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enc, _ := zeroCipher.EncryptZeroCopy(plaintext)
			enc.Release()
		}
	})

	encrypted, _ := stdCipher.Encrypt(plaintext)
	zeroEncBuf, _ := zeroCipher.EncryptZeroCopy(plaintext)
	defer zeroEncBuf.Release()

	b.Run("Standard-Decrypt", func(b *testing.B) {
		b.SetBytes(1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = stdCipher.Decrypt(encrypted)
		}
	})

	b.Run("ZeroCopy-Decrypt", func(b *testing.B) {
		b.SetBytes(1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dec, _ := zeroCipher.DecryptZeroCopy(zeroEncBuf.B)
			dec.Release()
		}
	})
}

// BenchmarkZeroCopy_Parallel 并发基准测试
func BenchmarkZeroCopy_Parallel(b *testing.B) {
	cipher := NewZeroCopyAesGcmEncrypt("test-key-12345")
	plaintext := make([]byte, 1024)

	b.SetBytes(1024)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			encBuf, _ := cipher.EncryptZeroCopy(plaintext)
			decBuf, _ := cipher.DecryptZeroCopy(encBuf.B)
			decBuf.Release()
			encBuf.Release()
		}
	})
}
