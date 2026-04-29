package zencrypt

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestBatchEncrypt_Empty 测试空输入
func TestBatchEncrypt_Empty(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	results, err := cipher.BatchEncrypt(nil)
	if err != nil {
		t.Errorf("空输入应该成功，但返回错误: %v", err)
	}
	if results != nil {
		t.Errorf("空输入应返回 nil，实际: %v", results)
	}

	results, err = cipher.BatchEncrypt([][]byte{})
	if err != nil {
		t.Errorf("空切片应该成功，但返回错误: %v", err)
	}
	if results != nil {
		t.Errorf("空切片应返回 nil，实际: %v", results)
	}
}

// TestBatchEncrypt_Single 测试单个消息
func TestBatchEncrypt_Single(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")
	plaintext := []byte("Hello, World!")

	results, err := cipher.BatchEncrypt([][]byte{plaintext})
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("期望1个结果，实际: %d", len(results))
	}

	// 验证可以正确解密
	decrypted, err := cipher.Decrypt(results[0])
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("解密结果不匹配\n期望: %s\n实际: %s", plaintext, decrypted)
	}
}

// TestBatchEncrypt_Multiple 测试多个消息
func TestBatchEncrypt_Multiple(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	// 准备测试数据
	plaintexts := [][]byte{
		[]byte("message-1"),
		[]byte("message-2"),
		[]byte("message-3"),
		[]byte("message-4"),
		[]byte("message-5"),
	}

	// 批量加密
	encrypted, err := cipher.BatchEncrypt(plaintexts)
	if err != nil {
		t.Fatalf("批量加密失败: %v", err)
	}

	if len(encrypted) != len(plaintexts) {
		t.Fatalf("期望 %d 个结果，实际: %d", len(plaintexts), len(encrypted))
	}

	// 验证每个消息都能正确解密
	for i, ciphertext := range encrypted {
		decrypted, err := cipher.Decrypt(ciphertext)
		if err != nil {
			t.Errorf("解密第 %d 个消息失败: %v", i, err)
			continue
		}

		if !bytes.Equal(decrypted, plaintexts[i]) {
			t.Errorf("第 %d 个消息解密结果不匹配\n期望: %s\n实际: %s", i, plaintexts[i], decrypted)
		}
	}
}

// TestBatchDecrypt 测试批量解密
func TestBatchDecrypt(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	plaintexts := [][]byte{
		[]byte("data-1"),
		[]byte("data-2"),
		[]byte("data-3"),
	}

	// 先批量加密
	encrypted, err := cipher.BatchEncrypt(plaintexts)
	if err != nil {
		t.Fatalf("批量加密失败: %v", err)
	}

	// 再批量解密
	decrypted, err := cipher.BatchDecrypt(encrypted)
	if err != nil {
		t.Fatalf("批量解密失败: %v", err)
	}

	// 验证结果
	if len(decrypted) != len(plaintexts) {
		t.Fatalf("期望 %d 个结果，实际: %d", len(plaintexts), len(decrypted))
	}

	for i, plain := range decrypted {
		if !bytes.Equal(plain, plaintexts[i]) {
			t.Errorf("第 %d 个解密结果不匹配\n期望: %s\n实际: %s", i, plaintexts[i], plain)
		}
	}
}

// TestBatchEncryptPooled_Empty 测试池化版本空输入
func TestBatchEncryptPooled_Empty(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	results, err := cipher.BatchEncryptPooled(nil, 4)
	if err != nil {
		t.Errorf("空输入应该成功，但返回错误: %v", err)
	}
	if results != nil {
		t.Errorf("空输入应返回 nil，实际: %v", results)
	}
}

// TestBatchEncryptPooled_Multiple 测试池化版本多消息
func TestBatchEncryptPooled_Multiple(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	// 准备 100 个消息
	count := 100
	plaintexts := make([][]byte, count)
	for i := 0; i < count; i++ {
		plaintexts[i] = []byte(fmt.Sprintf("message-%d", i))
	}

	// 使用 8 个 worker 并行加密
	encrypted, err := cipher.BatchEncryptPooled(plaintexts, 8)
	if err != nil {
		t.Fatalf("池化批量加密失败: %v", err)
	}

	if len(encrypted) != count {
		t.Fatalf("期望 %d 个结果，实际: %d", count, len(encrypted))
	}

	// 验证所有消息都能正确解密
	for i, ciphertext := range encrypted {
		decrypted, err := cipher.Decrypt(ciphertext)
		if err != nil {
			t.Errorf("解密第 %d 个消息失败: %v", i, err)
			continue
		}

		if !bytes.Equal(decrypted, plaintexts[i]) {
			t.Errorf("第 %d 个消息解密结果不匹配", i)
		}
	}
}

// TestBatchEncryptPooled_DefaultPoolSize 测试默认池大小
func TestBatchEncryptPooled_DefaultPoolSize(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	plaintexts := [][]byte{
		[]byte("test-1"),
		[]byte("test-2"),
	}

	// poolSize <= 0 应使用默认值 8
	encrypted, err := cipher.BatchEncryptPooled(plaintexts, 0)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if len(encrypted) != len(plaintexts) {
		t.Fatalf("期望 %d 个结果，实际: %d", len(plaintexts), len(encrypted))
	}
}

// TestBatchEncrypt_Concurrent 测试并发安全
func TestBatchEncrypt_Concurrent(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	plaintexts := [][]byte{
		[]byte("data-1"),
		[]byte("data-2"),
		[]byte("data-3"),
	}

	// 并发调用批量加密
	var wg sync.WaitGroup
	concurrency := 10
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			encrypted, err := cipher.BatchEncrypt(plaintexts)
			if err != nil {
				t.Errorf("goroutine %d 加密失败: %v", idx, err)
				return
			}

			if len(encrypted) != len(plaintexts) {
				t.Errorf("goroutine %d 结果数量不匹配", idx)
			}
		}(i)
	}

	wg.Wait()
}

// TestBatchEncrypt_LargeData 测试大数据量
func TestBatchEncrypt_LargeData(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大数据测试")
	}

	cipher := NewAesGcmEncrypt("test-key-12345")

	// 准备 1000 个 1KB 消息
	count := 1000
	plaintexts := make([][]byte, count)
	for i := 0; i < count; i++ {
		plaintexts[i] = make([]byte, 1024)
		for j := range plaintexts[i] {
			plaintexts[i][j] = byte(i % 256)
		}
	}

	start := time.Now()
	encrypted, err := cipher.BatchEncryptPooled(plaintexts, 8)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("大数据加密失败: %v", err)
	}

	if len(encrypted) != count {
		t.Fatalf("期望 %d 个结果，实际: %d", count, len(encrypted))
	}

	t.Logf("批量加密 %d 个 1KB 包，耗时: %v", count, elapsed)
}

// BenchmarkBatchEncrypt_vs_Sequential 对比顺序加密和批量加密
func BenchmarkBatchEncrypt_vs_Sequential(b *testing.B) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	// 准备 100 个 1KB 消息
	count := 100
	plaintexts := make([][]byte, count)
	for i := 0; i < count; i++ {
		plaintexts[i] = make([]byte, 1024)
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			results := make([][]byte, count)
			for j := 0; j < count; j++ {
				encrypted, _ := cipher.Encrypt(plaintexts[j])
				results[j] = encrypted
			}
		}
	})

	b.Run("BatchEncrypt", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cipher.BatchEncrypt(plaintexts)
		}
	})

	b.Run("BatchEncryptPooled-4", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cipher.BatchEncryptPooled(plaintexts, 4)
		}
	})

	b.Run("BatchEncryptPooled-8", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cipher.BatchEncryptPooled(plaintexts, 8)
		}
	})
}

// BenchmarkBatchEncrypt_DifferentSizes 测试不同批量大小
func BenchmarkBatchEncrypt_DifferentSizes(b *testing.B) {
	cipher := NewAesGcmEncrypt("test-key-12345")

	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		plaintexts := make([][]byte, size)
		for i := 0; i < size; i++ {
			plaintexts[i] = make([]byte, 1024)
		}

		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = cipher.BatchEncryptPooled(plaintexts, 8)
			}
		})
	}
}

func TestDefaultBatchWorkerSize(t *testing.T) {
	if got := defaultBatchWorkerSize(0); got != 1 {
		t.Fatalf("count=0: expected 1, got %d", got)
	}
	if got := defaultBatchWorkerSize(1); got != 1 {
		t.Fatalf("count=1: expected 1, got %d", got)
	}

	limit := runtime.GOMAXPROCS(0) * 2
	if limit < 1 {
		limit = 1
	}

	if got := defaultBatchWorkerSize(limit * 10); got != limit {
		t.Fatalf("large count: expected %d, got %d", limit, got)
	}
	if got := defaultBatchWorkerSize(3); got != minInt(3, limit) {
		t.Fatalf("small count: expected %d, got %d", minInt(3, limit), got)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
