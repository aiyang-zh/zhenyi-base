package zencrypt

import (
	"runtime"
	"testing"
)

// TestCheckAESNI 测试 AES-NI 检测功能
func TestCheckAESNI(t *testing.T) {
	result := CheckAESNI()
	arch := runtime.GOARCH

	t.Logf("当前架构: %s", arch)
	t.Logf("AES-NI 支持: %v", result)

	// 验证逻辑正确性
	switch arch {
	case "amd64", "arm64":
		if !result {
			t.Errorf("期望 amd64/arm64 架构支持硬件加速，但返回 false")
		}
	default:
		if result {
			t.Errorf("期望 %s 架构不支持硬件加速，但返回 true", arch)
		}
	}
}

// TestGetCryptoInfo 测试加密信息获取
func TestGetCryptoInfo(t *testing.T) {
	info := GetCryptoInfo()
	arch := runtime.GOARCH

	t.Logf("当前架构: %s", arch)
	t.Logf("加密实现: %s", info)

	// 验证返回信息非空
	if info == "" {
		t.Error("GetCryptoInfo 返回空字符串")
	}

	// 验证信息包含预期关键字
	switch arch {
	case "amd64":
		if info != "AES-NI hardware acceleration (auto-detected)" {
			t.Errorf("amd64 架构期望 AES-NI 信息，实际: %s", info)
		}
	case "arm64":
		if info != "ARM Crypto Extensions (auto-detected)" {
			t.Errorf("arm64 架构期望 ARM 信息，实际: %s", info)
		}
	default:
		if info != "Software implementation (no hardware acceleration)" {
			t.Errorf("其他架构期望软件实现信息，实际: %s", info)
		}
	}
}

// BenchmarkCheckAESNI 基准测试（验证无性能开销）
func BenchmarkCheckAESNI(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CheckAESNI()
	}
}

// BenchmarkGetCryptoInfo 基准测试
func BenchmarkGetCryptoInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetCryptoInfo()
	}
}
