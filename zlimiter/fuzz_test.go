package zlimiter

import "testing"

// FuzzLimiterAllow 对令牌桶 Allow 做模糊测试，要求不 panic（limit/burst 必须为正）。
func FuzzLimiterAllow(f *testing.F) {
	f.Add(10, 20)
	f.Add(1, 1)

	f.Fuzz(func(t *testing.T, limit, burst int) {
		if limit <= 0 {
			limit = 1
		}
		if burst <= 0 {
			burst = 1
		}
		if limit > 1_000_000 {
			limit = 1_000_000
		}
		if burst > 1_000_000 {
			burst = 1_000_000
		}
		l := NewLimiter(limit, burst)
		for i := 0; i < 32; i++ {
			_ = l.Allow()
		}
	})
}
