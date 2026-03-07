package zlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

// 测试辅助函数
func createOptionTestDir(t *testing.T) string {
	tempDir := filepath.Join(os.TempDir(), "zhenyi_option_test", t.Name(), time.Now().Format("150405.000"))
	os.RemoveAll(filepath.Join(os.TempDir(), "zhenyi_option_test", t.Name()))
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	return tempDir
}

// TestWithLevel 测试级别设置
func TestWithLevel(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithLevel(zapcore.DebugLevel).apply(&config)

	if config.Level != zapcore.DebugLevel {
		t.Errorf("期望 DebugLevel，实际: %v", config.Level)
	}
}

// TestWithLevelString 测试字符串级别设置
func TestWithLevelString(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithLevelString("debug").apply(&config)

	if config.Level != zapcore.DebugLevel {
		t.Errorf("期望 DebugLevel，实际: %v", config.Level)
	}
}

// TestWithConsole 测试控制台输出设置
func TestWithConsole(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithConsole(true).apply(&config)

	if !config.IsConsole {
		t.Error("IsConsole 应该为 true")
	}
}

// TestWithBufferOptions 测试缓冲相关 Options
func TestWithBufferOptions(t *testing.T) {
	t.Run("WithBuffer", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithBuffer().apply(&config)
		if config.BufferSize != 4096 {
			t.Errorf("期望 4096，实际: %d", config.BufferSize)
		}
	})

	t.Run("WithoutBuffer", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithoutBuffer().apply(&config)
		if config.BufferSize != 0 {
			t.Errorf("期望 0，实际: %d", config.BufferSize)
		}
	})

	t.Run("WithBufferSize", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithBufferSize(8192).apply(&config)
		if config.BufferSize != 8192 {
			t.Errorf("期望 8192，实际: %d", config.BufferSize)
		}
	})
}

// TestWithSamplingOptions 测试采样相关 Options
func TestWithSamplingOptions(t *testing.T) {
	t.Run("WithSampling", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithSampling(100, 50).apply(&config)

		if !config.EnableSampling {
			t.Error("EnableSampling 应该为 true")
		}
		if config.SamplingInitial != 100 {
			t.Errorf("期望 100，实际: %d", config.SamplingInitial)
		}
		if config.SamplingThereafter != 50 {
			t.Errorf("期望 50，实际: %d", config.SamplingThereafter)
		}
	})

	t.Run("WithDefaultSampling", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithDefaultSampling().apply(&config)

		if !config.EnableSampling {
			t.Error("EnableSampling 应该为 true")
		}
		if config.SamplingInitial != 1000 {
			t.Errorf("期望 1000，实际: %d", config.SamplingInitial)
		}
		if config.SamplingThereafter != 100 {
			t.Errorf("期望 100，实际: %d", config.SamplingThereafter)
		}
	})

	t.Run("WithAggressiveSampling", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithAggressiveSampling().apply(&config)

		if !config.EnableSampling {
			t.Error("EnableSampling 应该为 true")
		}
		if config.SamplingInitial != 100 {
			t.Errorf("期望 100，实际: %d", config.SamplingInitial)
		}
		if config.SamplingThereafter != 1000 {
			t.Errorf("期望 1000，实际: %d", config.SamplingThereafter)
		}
	})

	t.Run("WithoutSampling", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.EnableSampling = true
		WithoutSampling().apply(&config)

		if config.EnableSampling {
			t.Error("EnableSampling 应该为 false")
		}
	})
}

// TestWithCircuitBreakerOptions 测试熔断器相关 Options
func TestWithCircuitBreakerOptions(t *testing.T) {
	t.Run("WithCircuitBreaker", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithCircuitBreaker(1000, 5).apply(&config)

		if !config.EnableCircuitBreaker {
			t.Error("EnableCircuitBreaker 应该为 true")
		}
		if config.CircuitBreakerThreshold != 1000 {
			t.Errorf("期望 1000，实际: %d", config.CircuitBreakerThreshold)
		}
		if config.CircuitBreakerWindow != 5 {
			t.Errorf("期望 5，实际: %d", config.CircuitBreakerWindow)
		}
	})

	t.Run("WithDefaultCircuitBreaker", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithDefaultCircuitBreaker().apply(&config)

		if !config.EnableCircuitBreaker {
			t.Error("EnableCircuitBreaker 应该为 true")
		}
		if config.CircuitBreakerThreshold != 10000 {
			t.Errorf("期望 10000，实际: %d", config.CircuitBreakerThreshold)
		}
	})

	t.Run("WithStrictCircuitBreaker", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithStrictCircuitBreaker().apply(&config)

		if !config.EnableCircuitBreaker {
			t.Error("EnableCircuitBreaker 应该为 true")
		}
		if config.CircuitBreakerThreshold != 1000 {
			t.Errorf("期望 1000，实际: %d", config.CircuitBreakerThreshold)
		}
	})

	t.Run("WithoutCircuitBreaker", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.EnableCircuitBreaker = true
		WithoutCircuitBreaker().apply(&config)

		if config.EnableCircuitBreaker {
			t.Error("EnableCircuitBreaker 应该为 false")
		}
	})
}

// TestWithLogFileOptions 测试日志文件相关 Options
func TestWithLogFileOptions(t *testing.T) {
	t.Run("WithLogFile", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.Logs = nil
		WithLogFile("test", zapcore.InfoLevel).apply(&config)

		if config.Logs == nil {
			t.Fatal("Logs 不应该为 nil")
		}
		if level, ok := config.Logs["test"]; !ok || level != int(zapcore.InfoLevel) {
			t.Error("未正确设置日志文件")
		}
	})

	t.Run("WithLogFiles", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.Logs = nil
		WithLogFiles(map[string]zapcore.Level{
			"info":  zapcore.InfoLevel,
			"error": zapcore.ErrorLevel,
		}).apply(&config)

		if len(config.Logs) != 2 {
			t.Errorf("期望 2 个日志文件，实际: %d", len(config.Logs))
		}
	})

	t.Run("WithStandardLogFiles", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithStandardLogFiles().apply(&config)

		if len(config.Logs) != 2 {
			t.Errorf("期望 2 个日志文件，实际: %d", len(config.Logs))
		}
		if _, ok := config.Logs["info"]; !ok {
			t.Error("缺少 info 日志文件")
		}
		if _, ok := config.Logs["error"]; !ok {
			t.Error("缺少 error 日志文件")
		}
	})

	t.Run("WithAllLevelLogFiles", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithAllLevelLogFiles().apply(&config)

		if len(config.Logs) != 4 {
			t.Errorf("期望 4 个日志文件，实际: %d", len(config.Logs))
		}
	})

	t.Run("WithSingleLogFile", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithSingleLogFile("all").apply(&config)

		if len(config.Logs) != 1 {
			t.Errorf("期望 1 个日志文件，实际: %d", len(config.Logs))
		}
		if _, ok := config.Logs["all"]; !ok {
			t.Error("缺少 all 日志文件")
		}
	})
}

// TestWithEncoderOptions 测试编码器相关 Options
func TestWithEncoderOptions(t *testing.T) {
	t.Run("WithJSONEncoder", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithJSONEncoder().apply(&config)

		if !config.UseJSONEncoder {
			t.Error("UseJSONEncoder 应该为 true")
		}
	})

	t.Run("WithConsoleEncoder", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.UseJSONEncoder = true
		WithConsoleEncoder().apply(&config)

		if config.UseJSONEncoder {
			t.Error("UseJSONEncoder 应该为 false")
		}
	})
}

// TestPresetOptions 测试预设配置组合
func TestPresetOptions(t *testing.T) {
	t.Run("WithDevelopment", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithDevelopment().apply(&config)

		if config.Level != zapcore.DebugLevel {
			t.Error("开发环境应该是 Debug 级别")
		}
		if !config.IsConsole {
			t.Error("开发环境应该输出到控制台")
		}
		if !config.IsFileNum {
			t.Error("开发环境应该显示文件名和行号")
		}
		if config.UseJSONEncoder {
			t.Error("开发环境应该使用控制台编码器")
		}
		if config.BufferSize != 0 {
			t.Error("开发环境应该无缓冲")
		}
	})

	t.Run("WithProduction", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithProduction().apply(&config)

		if config.Level != zapcore.InfoLevel {
			t.Error("生产环境应该是 Info 级别")
		}
		if config.IsConsole {
			t.Error("生产环境不应该输出到控制台")
		}
		if !config.UseJSONEncoder {
			t.Error("生产环境应该使用 JSON 编码器")
		}
		if config.BufferSize == 0 {
			t.Error("生产环境应该启用缓冲")
		}
		if !config.EnableSampling {
			t.Error("生产环境应该启用采样")
		}
		if !config.EnableCircuitBreaker {
			t.Error("生产环境应该启用熔断器")
		}
	})

	t.Run("WithTest", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithTest().apply(&config)

		if config.Level != zapcore.DebugLevel {
			t.Error("测试环境应该是 Debug 级别")
		}
		if !config.IsConsole {
			t.Error("测试环境应该输出到控制台")
		}
		if config.IsFileNum {
			t.Error("测试环境不应该显示文件名和行号（简化输出）")
		}
		if config.BufferSize != 0 {
			t.Error("测试环境应该无缓冲")
		}
	})

	t.Run("WithHighPerformance", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithHighPerformance().apply(&config)

		if config.Level != zapcore.WarnLevel {
			t.Error("高性能环境应该是 Warn 级别")
		}
		if config.IsConsole {
			t.Error("高性能环境不应该输出到控制台")
		}
		if config.IsFileNum {
			t.Error("高性能环境不应该显示文件名和行号")
		}
		if config.BufferSize != 8192 {
			t.Errorf("高性能环境应该使用 8KB 缓冲，实际: %d", config.BufferSize)
		}
		if !config.EnableSampling {
			t.Error("高性能环境应该启用采样")
		}
		if !config.EnableCircuitBreaker {
			t.Error("高性能环境应该启用熔断器")
		}
	})
}

// TestNewLoggerWithOptions 测试使用 Options 创建 Logger
func TestNewLoggerWithOptions(t *testing.T) {
	tempDir := createOptionTestDir(t)

	baseConfig := NewDefaultLoggerConfig()
	baseConfig.PathName = tempDir
	baseConfig.Filename = "test"

	logger := NewLoggerWithOptions(baseConfig,
		WithLevel(zapcore.DebugLevel),
		WithConsole(false),
		WithSingleLogFile("all"),
	)
	defer logger.Close()

	// 验证配置已应用
	if logger.config.Level != zapcore.DebugLevel {
		t.Error("Level 未正确应用")
	}
	if logger.config.IsConsole {
		t.Error("IsConsole 未正确应用")
	}
	if len(logger.config.Logs) != 1 {
		t.Error("Logs 未正确应用")
	}

	// 测试日志功能
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Sync()
}

// TestNewDefaultLoggerWithOptions 测试使用 Options 创建默认 Logger
func TestNewDefaultLoggerWithOptions(t *testing.T) {
	tempDir := createOptionTestDir(t)

	NewDefaultLoggerWithOptions(
		WithLevel(zapcore.InfoLevel),
		WithPathName(tempDir),
		WithFilename("test"),
		WithStandardLogFiles(),
		WithConsole(false),
	)
	defer CloseDefaultLog()

	// 测试默认 Logger 功能
	GetDefaultLog().Info("test message from default logger")
	GetDefaultLog().Sync()

	// 验证日志级别
	if GetDefaultLog().GetLevel() != zapcore.InfoLevel {
		t.Error("默认 Logger 级别未正确设置")
	}
}

// TestMultipleOptionsOverride 测试多个 Options 的覆盖行为
func TestMultipleOptionsOverride(t *testing.T) {
	config := NewDefaultLoggerConfig()

	// 先设置为 Debug，再设置为 Info（应该被覆盖）
	WithLevel(zapcore.DebugLevel).apply(&config)
	WithLevel(zapcore.InfoLevel).apply(&config)

	if config.Level != zapcore.InfoLevel {
		t.Error("后应用的 Option 应该覆盖之前的")
	}
}

// TestOptionsCombination 测试 Options 组合使用
func TestOptionsCombination(t *testing.T) {
	tempDir := createOptionTestDir(t)

	baseConfig := NewDefaultLoggerConfig()
	baseConfig.PathName = tempDir
	baseConfig.Filename = "combined"

	logger := NewLoggerWithOptions(baseConfig,
		// 基础配置
		WithLevel(zapcore.InfoLevel),
		WithConsole(false),
		WithFileNum(true),
		// 性能配置
		WithBuffer(),
		WithDefaultSampling(),
		WithDefaultCircuitBreaker(),
		// 文件配置
		WithStandardLogFiles(),
		// 编码配置
		WithJSONEncoder(),
	)
	defer logger.Close()

	// 验证所有配置
	if logger.config.Level != zapcore.InfoLevel {
		t.Error("Level 未正确设置")
	}
	if logger.config.IsConsole {
		t.Error("IsConsole 未正确设置")
	}
	if !logger.config.IsFileNum {
		t.Error("IsFileNum 未正确设置")
	}
	if logger.config.BufferSize != 4096 {
		t.Error("BufferSize 未正确设置")
	}
	if !logger.config.EnableSampling {
		t.Error("EnableSampling 未正确设置")
	}
	if !logger.config.EnableCircuitBreaker {
		t.Error("EnableCircuitBreaker 未正确设置")
	}
	if !logger.config.UseJSONEncoder {
		t.Error("UseJSONEncoder 未正确设置")
	}

	// 测试日志输出
	for i := 0; i < 100; i++ {
		logger.Info("combined test message")
	}
	logger.Sync()

	t.Log("组合配置测试通过")
}

// TestWithPathName 测试路径名设置
func TestWithPathName(t *testing.T) {
	config := NewDefaultLoggerConfig()
	testPath := "/tmp/test_logs"
	WithPathName(testPath).apply(&config)

	if config.PathName != testPath {
		t.Errorf("期望 %s，实际: %s", testPath, config.PathName)
	}
}

// TestWithFilename 测试文件名前缀设置
func TestWithFilename(t *testing.T) {
	config := NewDefaultLoggerConfig()
	testFilename := "myapp"
	WithFilename(testFilename).apply(&config)

	if config.Filename != testFilename {
		t.Errorf("期望 %s，实际: %s", testFilename, config.Filename)
	}
}

// TestWithFileNum 测试文件名和行号显示
func TestWithFileNum(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithFileNum(true).apply(&config)

	if !config.IsFileNum {
		t.Error("IsFileNum 应该为 true")
	}

	WithFileNum(false).apply(&config)
	if config.IsFileNum {
		t.Error("IsFileNum 应该为 false")
	}
}

// TestWithCallerOnlyError 测试仅 Error 显示调用栈
func TestWithCallerOnlyError(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithCallerOnlyError(true).apply(&config)

	if !config.CallerOnlyError {
		t.Error("CallerOnlyError 应该为 true")
	}
}

// TestWithGoroutineID 测试 Goroutine ID 总开关
func TestWithGoroutineID(t *testing.T) {
	t.Run("WithGoroutineID_Enable", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithGoroutineID(true).apply(&config)

		if !config.EnableGoroutineID {
			t.Error("EnableGoroutineID 应该为 true")
		}
	})

	t.Run("WithGoroutineID_Disable", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithGoroutineID(false).apply(&config)

		if config.EnableGoroutineID {
			t.Error("EnableGoroutineID 应该为 false")
		}
	})

	t.Run("WithoutGoroutineID", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		config.EnableGoroutineID = true
		WithoutGoroutineID().apply(&config)

		if config.EnableGoroutineID {
			t.Error("EnableGoroutineID 应该为 false")
		}
	})
}

// TestWithGoroutineIDField 测试 Goroutine ID 字段模式
func TestWithGoroutineIDField(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithGoroutineIDField(true).apply(&config)

	if !config.UseFieldForGoroutineID {
		t.Error("UseFieldForGoroutineID 应该为 true")
	}
}

// TestConvenienceOptions 测试便捷 Options
func TestConvenienceOptions(t *testing.T) {
	t.Run("WithDebug", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithDebug().apply(&config)
		if config.Level != zapcore.DebugLevel {
			t.Error("应该设置为 DebugLevel")
		}
	})

	t.Run("WithInfo", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithInfo().apply(&config)
		if config.Level != zapcore.InfoLevel {
			t.Error("应该设置为 InfoLevel")
		}
	})

	t.Run("WithWarn", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithWarn().apply(&config)
		if config.Level != zapcore.WarnLevel {
			t.Error("应该设置为 WarnLevel")
		}
	})

	t.Run("WithError", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithError().apply(&config)
		if config.Level != zapcore.ErrorLevel {
			t.Error("应该设置为 ErrorLevel")
		}
	})

	t.Run("WithLargeBuffer", func(t *testing.T) {
		config := NewDefaultLoggerConfig()
		WithLargeBuffer().apply(&config)
		if config.BufferSize != 16384 {
			t.Errorf("期望 16384，实际: %d", config.BufferSize)
		}
	})
}

// TestWithStaging 测试预发布环境配置
func TestWithStaging(t *testing.T) {
	config := NewDefaultLoggerConfig()
	WithStaging().apply(&config)

	if config.Level != zapcore.InfoLevel {
		t.Error("预发布环境应该是 Info 级别")
	}
	if !config.IsConsole {
		t.Error("预发布环境应该输出到控制台")
	}
	if !config.IsFileNum {
		t.Error("预发布环境应该显示文件名和行号")
	}
	if !config.UseJSONEncoder {
		t.Error("预发布环境应该使用 JSON 编码器")
	}
	if config.BufferSize == 0 {
		t.Error("预发布环境应该启用缓冲")
	}
	if len(config.Logs) != 4 {
		t.Error("预发布环境应该有全级别日志文件")
	}
	if config.EnableSampling {
		t.Error("预发布环境不应该启用采样")
	}
	if !config.EnableCircuitBreaker {
		t.Error("预发布环境应该启用熔断器")
	}
}

// TestOptionsIntegration 集成测试：验证 Options 真正生效
func TestOptionsIntegration(t *testing.T) {
	tempDir := createOptionTestDir(t)

	logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
		WithLevel(zapcore.DebugLevel),
		WithPathName(tempDir),
		WithFilename("integration"),
		WithSingleLogFile("all"),
		WithConsole(false),
		WithJSONEncoder(),
	)
	defer logger.Close()

	// 测试 Debug 级别生效
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Error("error message")
	logger.Sync()

	// 验证文件存在
	time.Sleep(50 * time.Millisecond)
	matches, _ := filepath.Glob(filepath.Join(tempDir, "integration.*.all.log"))
	if len(matches) == 0 {
		t.Fatal("日志文件未创建")
	}

	// 读取文件内容
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	content := string(data)

	// 验证所有级别的日志都存在
	if !strings.Contains(content, "debug message") {
		t.Error("缺少 debug 日志")
	}
	if !strings.Contains(content, "info message") {
		t.Error("缺少 info 日志")
	}
	if !strings.Contains(content, "error message") {
		t.Error("缺少 error 日志")
	}

	t.Logf("集成测试通过，日志内容长度: %d 字节", len(content))
}

// TestDynamicLevelWithOptions 测试动态级别与 Options 配合
func TestDynamicLevelWithOptions(t *testing.T) {
	tempDir := createOptionTestDir(t)

	logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
		WithInfo(), // 初始 Info 级别
		WithPathName(tempDir),
		WithFilename("dynamic"),
		WithSingleLogFile("all"),
		WithConsole(false),
	)
	defer logger.Close()

	// 阶段1：Info 级别，Debug 不输出
	logger.Debug("debug1")
	logger.Info("info1")
	logger.Sync()

	// 动态切换到 Debug
	logger.SetLevel(zapcore.DebugLevel)

	// 阶段2：Debug 级别，Debug 输出
	logger.Debug("debug2")
	logger.Info("info2")
	logger.Sync()

	time.Sleep(50 * time.Millisecond)

	// 验证
	matches, _ := filepath.Glob(filepath.Join(tempDir, "dynamic.*.all.log"))
	if len(matches) == 0 {
		t.Fatal("日志文件未创建")
	}

	data, _ := os.ReadFile(matches[0])
	content := string(data)

	if strings.Contains(content, "debug1") {
		t.Error("debug1 不应该输出（初始是 Info 级别）")
	}
	if !strings.Contains(content, "info1") {
		t.Error("info1 应该输出")
	}
	if !strings.Contains(content, "debug2") {
		t.Error("debug2 应该输出（已切换到 Debug 级别）")
	}
	if !strings.Contains(content, "info2") {
		t.Error("info2 应该输出")
	}

	t.Log("动态级别测试通过")
}

// TestSamplingWithOptions 测试采样与 Options 配合
func TestSamplingWithOptions(t *testing.T) {
	tempDir := createOptionTestDir(t)

	logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
		WithDebug(),
		WithPathName(tempDir),
		WithFilename("sampling"),
		WithSingleLogFile("all"),
		WithConsole(false),
		WithSampling(5, 5), // 前5条必出，之后每5条出1条
	)
	defer logger.Close()

	// 写入20条日志
	for i := 0; i < 20; i++ {
		logger.Info("repeated message")
	}
	logger.Sync()

	time.Sleep(50 * time.Millisecond)

	// 验证采样生效
	matches, _ := filepath.Glob(filepath.Join(tempDir, "sampling.*.all.log"))
	if len(matches) == 0 {
		t.Fatal("日志文件未创建")
	}

	data, _ := os.ReadFile(matches[0])
	content := string(data)
	lines := strings.Count(strings.TrimSpace(content), "\n") + 1

	// 期望：前5条 + (20-5)/5 = 5 + 3 = 8 条
	if lines > 12 {
		t.Errorf("采样应该减少日志量，期望 <= 12，实际: %d", lines)
	}

	t.Logf("采样测试通过，输出了 %d/%d 条日志", lines, 20)
}

// TestCircuitBreakerWithOptions 测试熔断器与 Options 配合
func TestCircuitBreakerWithOptions(t *testing.T) {
	tempDir := createOptionTestDir(t)

	logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
		WithDebug(),
		WithPathName(tempDir),
		WithFilename("breaker"),
		WithSingleLogFile("all"),
		WithConsole(false),
		WithCircuitBreaker(10, 10), // 10秒内最多10条
	)
	defer logger.Close()

	// 写入100条日志
	for i := 0; i < 100; i++ {
		logger.Info("test message")
	}
	logger.Sync()

	time.Sleep(50 * time.Millisecond)

	// 验证熔断器生效
	matches, _ := filepath.Glob(filepath.Join(tempDir, "breaker.*.all.log"))
	if len(matches) == 0 {
		t.Fatal("日志文件未创建")
	}

	data, _ := os.ReadFile(matches[0])
	content := string(data)
	count := strings.Count(content, "test message")

	if count > 20 {
		t.Errorf("熔断器应该限制日志量，期望 <= 20，实际: %d", count)
	}

	t.Logf("熔断器测试通过，输出了 %d/%d 条日志", count, 100)
}

// TestPresetWithOverride 测试预设配置被覆盖
func TestPresetWithOverride(t *testing.T) {
	config := NewDefaultLoggerConfig()

	// 先应用生产环境预设
	WithProduction().apply(&config)

	// 然后覆盖部分配置
	WithLevel(zapcore.DebugLevel).apply(&config)
	WithConsole(true).apply(&config)

	// 验证覆盖生效
	if config.Level != zapcore.DebugLevel {
		t.Error("Level 应该被覆盖为 DebugLevel")
	}
	if !config.IsConsole {
		t.Error("IsConsole 应该被覆盖为 true")
	}

	// 验证其他配置仍然保留
	if !config.UseJSONEncoder {
		t.Error("UseJSONEncoder 应该保留生产环境配置")
	}
	if !config.EnableSampling {
		t.Error("EnableSampling 应该保留生产环境配置")
	}
}

// TestAllPresetsValid 测试所有预设配置都有效
func TestAllPresetsValid(t *testing.T) {
	presets := map[string]Option{
		"Development":     WithDevelopment(),
		"Production":      WithProduction(),
		"Test":            WithTest(),
		"HighPerformance": WithHighPerformance(),
		"Staging":         WithStaging(),
	}

	for name, preset := range presets {
		t.Run(name, func(t *testing.T) {
			tempDir := createOptionTestDir(t)

			logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
				preset,
				WithPathName(tempDir),
				WithFilename("preset"),
			)
			defer logger.Close()

			// 测试基本日志功能
			logger.Info("test message for " + name)
			logger.Error("error message for " + name)
			logger.Sync()

			// 验证日志文件创建
			time.Sleep(50 * time.Millisecond)
			matches, _ := filepath.Glob(filepath.Join(tempDir, "preset.*.*.log"))
			if len(matches) == 0 {
				t.Fatalf("%s 预设配置未创建日志文件", name)
			}

			t.Logf("%s 预设配置测试通过", name)
		})
	}
}

// TestGoroutineIDIntegration 集成测试：验证 Goroutine ID 开关真正生效
func TestGoroutineIDIntegration(t *testing.T) {
	t.Run("EnableGoroutineID", func(t *testing.T) {
		tempDir := createOptionTestDir(t)

		logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
			WithInfo(),
			WithPathName(tempDir),
			WithFilename("goid_enabled"),
			WithSingleLogFile("all"),
			WithConsole(false),
			WithGoroutineID(true), // 启用
		)
		defer logger.Close()

		logger.Info("test message with goroutine id")
		logger.Sync()
		time.Sleep(50 * time.Millisecond)

		matches, _ := filepath.Glob(filepath.Join(tempDir, "goid_enabled.*.all.log"))
		if len(matches) == 0 {
			t.Fatal("日志文件未创建")
		}

		data, _ := os.ReadFile(matches[0])
		content := string(data)

		// 应该包含 goroutine 字样
		if !strings.Contains(content, "goroutine") {
			t.Error("启用 Goroutine ID 时，日志应该包含 goroutine 信息")
		}
	})

	t.Run("DisableGoroutineID", func(t *testing.T) {
		tempDir := createOptionTestDir(t)

		logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
			WithInfo(),
			WithPathName(tempDir),
			WithFilename("goid_disabled"),
			WithSingleLogFile("all"),
			WithConsole(false),
			WithoutGoroutineID(), // 禁用
		)
		defer logger.Close()

		logger.Info("test message without goroutine id")
		logger.Sync()
		time.Sleep(50 * time.Millisecond)

		matches, _ := filepath.Glob(filepath.Join(tempDir, "goid_disabled.*.all.log"))
		if len(matches) == 0 {
			t.Fatal("日志文件未创建")
		}

		data, _ := os.ReadFile(matches[0])
		content := string(data)

		// 更精确的检查：不应该包含 [goroutine:] 或 "goroutine": 格式
		if strings.Contains(content, "[goroutine:") || strings.Contains(content, `"goroutine"`) {
			t.Errorf("禁用 Goroutine ID 时，日志不应该包含 goroutine 信息。日志内容：\n%s", content)
		}
	})
}

// TestPresetGoroutineIDSetting 测试预设配置中的 Goroutine ID 设置
func TestPresetGoroutineIDSetting(t *testing.T) {
	testCases := []struct {
		name     string
		preset   Option
		expected bool
	}{
		{"Development", WithDevelopment(), true},          // Debug 级别，应该启用
		{"Production", WithProduction(), false},           // Info 级别，应该禁用
		{"Test", WithTest(), true},                        // Debug 级别，应该启用
		{"HighPerformance", WithHighPerformance(), false}, // Warn 级别，应该禁用
		{"Staging", WithStaging(), false},                 // Info 级别，应该禁用
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := NewDefaultLoggerConfig()
			tc.preset.apply(&config)

			if config.EnableGoroutineID != tc.expected {
				t.Errorf("%s 环境：EnableGoroutineID 期望 %v，实际 %v",
					tc.name, tc.expected, config.EnableGoroutineID)
			}
		})
	}
}

// BenchmarkOptionApply 基准测试：Option 应用性能
func BenchmarkOptionApply(b *testing.B) {
	config := NewDefaultLoggerConfig()
	opt := WithLevel(zapcore.InfoLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opt.apply(&config)
	}
}

// BenchmarkMultipleOptionsApply 基准测试：多个 Options 应用性能
func BenchmarkMultipleOptionsApply(b *testing.B) {
	opts := []Option{
		WithLevel(zapcore.InfoLevel),
		WithConsole(false),
		WithBuffer(),
		WithDefaultSampling(),
		WithDefaultCircuitBreaker(),
		WithStandardLogFiles(),
		WithJSONEncoder(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := NewDefaultLoggerConfig()
		for _, opt := range opts {
			opt.apply(&config)
		}
	}
}

// BenchmarkNewLoggerWithOptions 基准测试：使用 Options 创建 Logger
func BenchmarkNewLoggerWithOptions(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zhenyi_bench_options")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := NewLoggerWithOptions(NewDefaultLoggerConfig(),
			WithInfo(),
			WithPathName(tempDir),
			WithFilename("bench"),
			WithStandardLogFiles(),
			WithConsole(false),
		)
		logger.Close()
	}
}

// BenchmarkPresetOptions 基准测试：预设配置应用性能
func BenchmarkPresetOptions(b *testing.B) {
	presets := []Option{
		WithDevelopment(),
		WithProduction(),
		WithTest(),
		WithHighPerformance(),
		WithStaging(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := NewDefaultLoggerConfig()
		preset := presets[i%len(presets)]
		preset.apply(&config)
	}
}
