package zencrypt

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"testing"
)

// TestPoolStats 验证对象池工作正常
func TestPoolStats(t *testing.T) {
	cipher := NewAesEncrypt("test-secret-key-32bytes-long")

	// 执行加密操作
	plaintext := make([]byte, 1024)
	for i := 0; i < 1000; i++ {
		encrypted, err := cipher.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		_, err = cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 获取池统计
	stats := zpool.GetStats()

	t.Logf("\n=== 对象池统计信息 ===")
	t.Logf("直接分配（超大）: %d 次", stats.DirectAllocs)

	for i, bucket := range stats.BucketStats {
		if bucket.GetRequests > 0 {
			t.Logf("桶 %d (大小=%d字节):", i, bucket.BucketSize)
			t.Logf("  获取次数: %d", bucket.GetRequests)
			t.Logf("  归还次数: %d", bucket.PutReturns)
			t.Logf("  复用率: %.2f%%", bucket.ReuseRate)
		}
	}

	// 验证对象池被使用
	hasUsage := false
	for _, bucket := range stats.BucketStats {
		if bucket.GetRequests > 0 {
			hasUsage = true

			// 验证复用率合理（至少50%被归还）
			if bucket.ReuseRate < 50.0 {
				t.Logf("警告: 桶 %d 复用率较低: %.2f%%", bucket.BucketSize, bucket.ReuseRate)
			}
		}
	}

	if !hasUsage {
		t.Error("对象池未被使用！")
	} else {
		t.Log("✅ 对象池工作正常")
	}
}

// TestAESGCM_PoolStats 测试 AES-GCM 的池使用情况
func TestAESGCM_PoolStats(t *testing.T) {
	cipher := NewAesGcmEncrypt("test-secret-key-32bytes-long")

	// 执行加密操作
	plaintext := make([]byte, 1024)
	for i := 0; i < 1000; i++ {
		encrypted, err := cipher.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		_, err = cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 获取池统计
	stats := zpool.GetStats()

	t.Logf("\n=== AES-GCM 对象池统计 ===")
	for i, bucket := range stats.BucketStats {
		if bucket.GetRequests > 0 {
			t.Logf("桶 %d (大小=%d): Get=%d, Put=%d, 复用率=%.2f%%",
				i, bucket.BucketSize, bucket.GetRequests, bucket.PutReturns, bucket.ReuseRate)
		}
	}
}

// Example_poolUsage 展示池的使用示例
func Example_poolUsage() {
	cipher := NewAesEncrypt("test-key")
	plaintext := []byte("Hello, World!")

	// 加密
	encrypted, _ := cipher.Encrypt(plaintext)

	// 解密
	decrypted, _ := cipher.Decrypt(encrypted)

	fmt.Println(string(decrypted))
	// Output: Hello, World!
}
