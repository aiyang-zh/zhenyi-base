package zlimiter

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLimiter_Basic 测试基本限流功能
func TestLimiter_Basic(t *testing.T) {
	// 创建限流器：每秒 10 个请求，最大突发 10
	limiter := NewLimiter(10, 10)

	if limiter == nil {
		t.Fatal("Expected limiter to be created")
	}

	if limiter.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", limiter.Limit)
	}

	if limiter.MaxLimit != 10 {
		t.Errorf("Expected maxLimit 10, got %d", limiter.MaxLimit)
	}
}

// TestLimiter_Allow 测试允许请求
func TestLimiter_Allow(t *testing.T) {
	// 创建限流器：每秒 5 个请求，最大突发 5
	limiter := NewLimiter(5, 5)

	// 前 5 个请求应该被允许（突发容量）
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should be allowed (burst)", i+1)
		}
	}

	// 第 6 个请求应该被拒绝
	if limiter.Allow() {
		t.Error("Request 6 should be denied (burst exhausted)")
	}
}

// TestLimiter_Refill 测试令牌桶补充
func TestLimiter_Refill(t *testing.T) {
	// 创建限流器：每秒 10 个请求，最大突发 10
	limiter := NewLimiter(10, 10)

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 下一个请求应该被拒绝
	if limiter.Allow() {
		t.Error("Request 11 should be denied")
	}

	// 等待 200ms（应该补充 2 个令牌）
	time.Sleep(200 * time.Millisecond)

	// 应该能通过 2 个请求
	allowedCount := 0
	for i := 0; i < 5; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	if allowedCount < 1 || allowedCount > 3 {
		// 由于时间精度问题，允许 1-3 个请求通过
		t.Errorf("Expected 1-3 requests to be allowed after 200ms, got %d", allowedCount)
	}
}

// TestLimiter_HighRate 测试高速率限流
func TestLimiter_HighRate(t *testing.T) {
	// 创建限流器：每秒 100 个请求，最大突发 100
	limiter := NewLimiter(100, 100)

	// 快速发送 150 个请求
	allowedCount := 0
	for i := 0; i < 150; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	// 应该允许约 100 个请求（突发容量）
	if allowedCount < 95 || allowedCount > 105 {
		t.Errorf("Expected ~100 requests to be allowed, got %d", allowedCount)
	}

	deniedCount := 150 - allowedCount
	t.Logf("Allowed: %d, Denied: %d", allowedCount, deniedCount)
}

// TestLimiter_Concurrent 测试并发请求
func TestLimiter_Concurrent(t *testing.T) {
	// 创建限流器：每秒 50 个请求，最大突发 50
	limiter := NewLimiter(50, 50)

	const goroutines = 10
	const requestsPerGoroutine = 10
	const totalRequests = goroutines * requestsPerGoroutine

	var allowedCount atomic.Int32
	var deniedCount atomic.Int32

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.Allow() {
					allowedCount.Add(1)
				} else {
					deniedCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	allowed := allowedCount.Load()
	denied := deniedCount.Load()

	if int(allowed+denied) != totalRequests {
		t.Errorf("Expected %d total requests, got %d", totalRequests, allowed+denied)
	}

	// 应该允许约 50 个请求（突发容量）
	if allowed < 45 || allowed > 55 {
		t.Errorf("Expected ~50 requests to be allowed, got %d", allowed)
	}

	t.Logf("Concurrent test - Allowed: %d, Denied: %d", allowed, denied)
}

// TestLimiter_ZeroLimit 测试零限制（实际上会阻止所有请求）
func TestLimiter_ZeroLimit(t *testing.T) {
	// 创建限流器：每秒 0 个请求（阻止所有）
	limiter := NewLimiter(0, 0)

	// 所有请求都应该被拒绝
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			t.Errorf("Request %d should be denied with zero limit", i+1)
		}
	}
}

// TestLimiter_LowBurst 测试低突发容量
func TestLimiter_LowBurst(t *testing.T) {
	// 创建限流器：每秒 10 个请求，但突发容量只有 2
	limiter := NewLimiter(10, 2)

	// 前 2 个请求应该被允许（突发容量）
	allowedCount := 0
	for i := 0; i < 5; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	if allowedCount < 2 || allowedCount > 3 {
		t.Errorf("Expected 2-3 requests to be allowed with burst=2, got %d", allowedCount)
	}
}

// TestLimiter_SustainedLoad 测试持续负载
func TestLimiter_SustainedLoad(t *testing.T) {
	// 创建限流器：每秒 20 个请求，最大突发 20
	limiter := NewLimiter(20, 20)

	// 在 500ms 内发送请求
	var allowedCount atomic.Int32
	var deniedCount atomic.Int32

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			if limiter.Allow() {
				allowedCount.Add(1)
			} else {
				deniedCount.Add(1)
			}
			time.Sleep(10 * time.Millisecond) // 每 10ms 一个请求
		}
		close(done)
	}()

	<-done

	allowed := allowedCount.Load()
	denied := deniedCount.Load()

	t.Logf("Sustained load - Allowed: %d, Denied: %d", allowed, denied)

	// 在 1 秒内发送 100 个请求，每 10ms 一个
	// 限流器：20 QPS，突发20
	// 实际：最初突发20个立即通过，然后每秒补充20个
	// 1秒内应该允许约 30-50 个请求（20突发 + 20补充 + 时间误差）
	if allowed < 25 || allowed > 50 {
		t.Errorf("Expected ~30-45 requests to be allowed, got %d", allowed)
	}
}

// TestLimiter_BurstRecovery 测试突发恢复
func TestLimiter_BurstRecovery(t *testing.T) {
	// 创建限流器：每秒 10 个请求，最大突发 10
	limiter := NewLimiter(10, 10)

	// 第一波：消耗所有令牌
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}

	// 等待 1 秒让令牌恢复
	time.Sleep(1 * time.Second)

	// 第二波：应该又能允许约 10 个请求
	allowedCount := 0
	for i := 0; i < 15; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	if allowedCount < 8 || allowedCount > 12 {
		t.Errorf("Expected ~10 requests to be allowed after recovery, got %d", allowedCount)
	}
}

// BenchmarkLimiter_Allow 基准测试 Allow 方法
func BenchmarkLimiter_Allow(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000) // 高限制，避免限流影响性能测试

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

// BenchmarkLimiter_AllowConcurrent 基准测试并发 Allow
func BenchmarkLimiter_AllowConcurrent(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000) // 高限制

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkLimiter_AllowWithDeny 基准测试带拒绝的 Allow
func BenchmarkLimiter_AllowWithDeny(b *testing.B) {
	limiter := NewLimiter(10, 10) // 低限制，会有拒绝

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

// BenchmarkLimiter_NewLimiter 基准测试创建限流器
func BenchmarkLimiter_NewLimiter(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		NewLimiter(100, 100)
	}
}
