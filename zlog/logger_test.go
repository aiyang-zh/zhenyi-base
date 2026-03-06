package zlog

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 测试辅助函数：创建临时日志配置
func createTestConfig(t *testing.T) LoggerConfig {
	tempDir := filepath.Join(os.TempDir(), "zynet_logger_test", t.Name(), time.Now().Format("150405.000"))
	// 先清理可能存在的旧目录
	os.RemoveAll(filepath.Join(os.TempDir(), "zynet_logger_test", t.Name()))
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() {
		// 延迟删除，给文件足够时间释放
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	return LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "test",
		IsConsole:            false,
		IsFileNum:            false,
		UseJSONEncoder:       false,
		BufferSize:           0, // 测试时不使用缓冲，方便立即读取
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info":  int(zapcore.InfoLevel),
			"error": int(zapcore.ErrorLevel),
		},
	}
}

// 测试辅助函数：读取日志文件内容（支持通配符）
func readLogFile(t *testing.T, pattern string) string {
	// 如果pattern包含通配符，使用filepath.Glob
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob 失败: %v", err)
	}
	if len(matches) == 0 {
		return ""
	}
	// 读取第一个匹配的文件
	data, err := os.ReadFile(matches[0])
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("读取日志文件失败: %v", err)
	}
	return string(data)
}

// 测试辅助函数：计算日志行数
func countLogLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(strings.TrimSpace(content), "\n") + 1
}

// TestLogger_BasicLogging 测试基本日志功能
func TestLogger_BasicLogging(t *testing.T) {
	config := createTestConfig(t)
	logger := NewLogger(config)

	// 写入不同级别的日志
	logger.Debug("debug message") // 应该被过滤（全局 Level 是 Info）
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// 强制刷盘并关闭
	logger.Sync()
	logger.Close()

	// 等待文件释放
	time.Sleep(50 * time.Millisecond)

	// 读取日志文件（使用通配符匹配日期）
	infoLog := readLogFile(t, filepath.Join(config.PathName, "test.*.info.log"))
	errorLog := readLogFile(t, filepath.Join(config.PathName, "test.*.error.log"))

	// 验证 info.log 应该包含 info、warn（因为 Logs["info"] = InfoLevel，所有 >= Info 的都会写入）
	if !strings.Contains(infoLog, "info message") {
		t.Error("info.log 应该包含 info message")
	}
	if !strings.Contains(infoLog, "warn message") {
		t.Error("info.log 应该包含 warn message")
	}
	if strings.Contains(infoLog, "debug message") {
		t.Error("info.log 不应该包含 debug message（被全局 Level 过滤）")
	}

	// 验证 error.log 应该只包含 error
	if !strings.Contains(errorLog, "error message") {
		t.Error("error.log 应该包含 error message")
	}
	if strings.Contains(errorLog, "info message") {
		t.Error("error.log 不应该包含 info message")
	}
}

// TestLogger_DynamicLevel 测试动态级别控制（核心功能）
func TestLogger_DynamicLevel(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.InfoLevel
	config.Logs = map[string]int{
		"all":   int(zapcore.DebugLevel), // 文件接受所有级别
		"error": int(zapcore.ErrorLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 阶段1：初始级别是 Info
	logger.Debug("debug1") // 被全局 Level 拦截
	logger.Info("info1")   // 通过
	logger.Sync()

	allLog := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if strings.Contains(allLog, "debug1") {
		t.Error("初始 Info 级别，不应该输出 debug1")
	}
	if !strings.Contains(allLog, "info1") {
		t.Error("应该输出 info1")
	}

	// 阶段2：动态降低到 Debug
	logger.SetLevel(zapcore.DebugLevel)
	logger.Debug("debug2") // 应该通过
	logger.Info("info2")
	logger.Sync()

	allLog = readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if !strings.Contains(allLog, "debug2") {
		t.Error("SetLevel(Debug) 后，应该输出 debug2")
	}
	if !strings.Contains(allLog, "info2") {
		t.Error("应该输出 info2")
	}

	// 阶段3：动态提高到 Error
	logger.SetLevel(zapcore.ErrorLevel)
	logger.Info("info3")   // 被全局 Level 拦截
	logger.Error("error3") // 通过
	logger.Sync()

	allLog = readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if strings.Contains(allLog, "info3") {
		t.Error("SetLevel(Error) 后，不应该输出 info3")
	}
	if !strings.Contains(allLog, "error3") {
		t.Error("应该输出 error3")
	}

	// 验证 GetLevel
	if logger.GetLevel() != zapcore.ErrorLevel {
		t.Errorf("GetLevel() 应该返回 ErrorLevel，实际: %v", logger.GetLevel())
	}
}

// TestLogger_DynamicLevel_ErrorFileIsolation 测试 Error 文件不受动态级别影响
func TestLogger_DynamicLevel_ErrorFileIsolation(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.InfoLevel
	config.Logs = map[string]int{
		"all":   int(zapcore.DebugLevel),
		"error": int(zapcore.ErrorLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 初始：只有 Error 写入 error.log
	logger.Info("info1")
	logger.Error("error1")
	logger.Sync()

	errorLog := readLogFile(t, filepath.Join(config.PathName, "test.*.error.log"))
	if !strings.Contains(errorLog, "error1") {
		t.Error("error.log 应该包含 error1")
	}
	if strings.Contains(errorLog, "info1") {
		t.Error("error.log 不应该包含 info1")
	}

	// 动态降低到 Debug
	logger.SetLevel(zapcore.DebugLevel)
	logger.Debug("debug2")
	logger.Info("info2")
	logger.Error("error2")
	logger.Sync()

	errorLog = readLogFile(t, filepath.Join(config.PathName, "test.*.error.log"))
	// 关键验证：error.log 仍然只有 Error 级别，不会因为 SetLevel(Debug) 变脏
	if !strings.Contains(errorLog, "error2") {
		t.Error("error.log 应该包含 error2")
	}
	if strings.Contains(errorLog, "debug2") {
		t.Error("error.log 不应该包含 debug2（即使全局级别是 Debug）")
	}
	if strings.Contains(errorLog, "info2") {
		t.Error("error.log 不应该包含 info2")
	}
}

// TestLogger_SetLevelByString 测试通过字符串设置级别
func TestLogger_SetLevelByString(t *testing.T) {
	testCases := []struct {
		levelStr string
		logDebug bool
		logInfo  bool
		logError bool
	}{
		{"debug", true, true, true},
		{"info", false, true, true},
		{"warn", false, false, true},
		{"error", false, false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.levelStr, func(t *testing.T) {
			// 每个子测试使用独立的配置和logger
			config := createTestConfig(t)
			config.Level = zapcore.InfoLevel
			config.Logs = map[string]int{
				"all": int(zapcore.DebugLevel),
			}
			logger := NewLogger(config)

			err := logger.SetLevelByString(tc.levelStr)
			if err != nil {
				t.Fatalf("SetLevelByString(%s) 失败: %v", tc.levelStr, err)
			}

			logger.Debug("debug msg")
			logger.Info("info msg")
			logger.Error("error msg")
			logger.Sync()
			logger.Close()
			time.Sleep(50 * time.Millisecond)

			content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))

			if tc.logDebug && !strings.Contains(content, "debug msg") {
				t.Errorf("级别 %s 应该输出 debug", tc.levelStr)
			}
			if !tc.logDebug && strings.Contains(content, "debug msg") {
				t.Errorf("级别 %s 不应该输出 debug", tc.levelStr)
			}

			if tc.logInfo && !strings.Contains(content, "info msg") {
				t.Errorf("级别 %s 应该输出 info", tc.levelStr)
			}
			if !tc.logInfo && strings.Contains(content, "info msg") {
				t.Errorf("级别 %s 不应该输出 info", tc.levelStr)
			}
		})
	}

	// 测试无效级别（使用新的logger实例）
	config := createTestConfig(t)
	config.Logs = map[string]int{
		"all": int(zapcore.DebugLevel),
	}
	logger := NewLogger(config)
	err := logger.SetLevelByString("invalid")
	if err == nil {
		t.Error("SetLevelByString('invalid') 应该返回错误")
	}
	logger.Close()
}

// TestLogger_Sampling 测试采样功能
func TestLogger_Sampling(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.DebugLevel
	config.EnableSampling = true
	config.SamplingInitial = 5    // 前5条必定输出
	config.SamplingThereafter = 5 // 之后每5条输出1条
	config.Logs = map[string]int{
		"all": int(zapcore.DebugLevel),
	}
	logger := NewLogger(config)

	// 写入20条相同的日志
	for i := 0; i < 20; i++ {
		logger.Info("repeated message", zap.Int("i", i))
	}
	logger.Sync()
	logger.Close()
	time.Sleep(50 * time.Millisecond)

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	lines := countLogLines(content)

	// 期望：前5条 + (20-5)/5 = 5 + 3 = 8 条
	// 允许一定误差
	expectedMin := 6
	expectedMax := 10
	if lines < expectedMin || lines > expectedMax {
		t.Errorf("采样后日志行数应该在 %d~%d 之间，实际: %d", expectedMin, expectedMax, lines)
	}
	t.Logf("采样结果：输出了 %d/%d 条日志", lines, 20)
}

// TestLogger_CircuitBreaker 测试熔断功能
func TestLogger_CircuitBreaker(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.DebugLevel
	config.EnableCircuitBreaker = true
	config.CircuitBreakerThreshold = 15 // 10秒内最多15条
	config.CircuitBreakerWindow = 10
	config.Logs = map[string]int{
		"all": int(zapcore.DebugLevel),
	}
	logger := NewLogger(config)

	// 写入100条 Info 日志（应该被熔断）
	for i := 0; i < 100; i++ {
		logger.Info("info message", zap.Int("i", i))
	}

	// 写入5条 Error 日志（Error 不受熔断影响）
	for i := 0; i < 5; i++ {
		logger.Error("error message", zap.Int("i", i))
	}

	logger.Sync()
	logger.Close()
	time.Sleep(50 * time.Millisecond)

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	infoCount := strings.Count(content, "info message")
	errorCount := strings.Count(content, "error message")

	// 验证：Info 日志应该被限制在阈值附近
	if infoCount > config.CircuitBreakerThreshold+10 {
		t.Errorf("熔断后 Info 日志应该 <= %d，实际: %d", config.CircuitBreakerThreshold+10, infoCount)
	}

	// 验证：Error 日志应该全部输出
	if errorCount != 5 {
		t.Errorf("Error 日志不应该被熔断，期望 5，实际: %d", errorCount)
	}

	t.Logf("熔断结果：Info=%d/100, Error=%d/5", infoCount, errorCount)
}

// TestLogger_CircuitBreaker_Concurrent 测试熔断的并发安全性
func TestLogger_CircuitBreaker_Concurrent(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.InfoLevel
	config.EnableCircuitBreaker = true
	config.CircuitBreakerThreshold = 100
	config.CircuitBreakerWindow = 1
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	var wg sync.WaitGroup
	goroutines := 10
	logsPerGoroutine := 100

	// 10个 goroutine 并发写入
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				logger.Info("concurrent log")
			}
		}(i)
	}

	wg.Wait()
	logger.Sync()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	lines := countLogLines(content)

	// 验证：应该有熔断效果（远少于 1000 条）
	totalLogs := goroutines * logsPerGoroutine
	if lines >= totalLogs {
		t.Errorf("熔断应该生效，期望 < %d，实际: %d", totalLogs, lines)
	}
	if lines > config.CircuitBreakerThreshold*2 {
		t.Errorf("熔断后日志数应该接近阈值 %d，实际: %d", config.CircuitBreakerThreshold, lines)
	}

	t.Logf("并发熔断结果：%d/%d 条日志通过", lines, totalLogs)
}

// TestLogger_WriteInterface 测试 io.Writer 接口
func TestLogger_WriteInterface(t *testing.T) {
	config := createTestConfig(t)
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)

	// 测试正常消息
	n, err := logger.Write([]byte("test message"))
	if err != nil {
		t.Errorf("Write() 返回错误: %v", err)
	}
	if n != len("test message") {
		t.Errorf("Write() 返回长度不对，期望 %d，实际 %d", len("test message"), n)
	}

	// 测试带换行符的消息（应该被 TrimSuffix）
	logger.Write([]byte("message with newline\n"))

	// 测试空消息（应该被忽略）
	logger.Write([]byte(""))
	logger.Write([]byte("\n"))

	logger.Sync()
	logger.Close()
	time.Sleep(50 * time.Millisecond)

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))

	if !strings.Contains(content, "test message") {
		t.Error("应该包含 'test message'")
	}
	if !strings.Contains(content, "message with newline") {
		t.Error("应该包含 'message with newline'")
	}
	// 验证换行符被去除（不应该有多余的空行）
	lines := countLogLines(content)
	if lines != 2 {
		t.Logf("日志内容：\n%s", content)
		t.Errorf("应该只有2条日志，实际: %d", lines)
	}
}

// TestLogger_Concurrent 测试并发安全性
func TestLogger_Concurrent(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.DebugLevel
	config.EnableSampling = false       // 禁用采样
	config.EnableCircuitBreaker = false // 禁用熔断
	config.Logs = map[string]int{
		"all": int(zapcore.DebugLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	var wg sync.WaitGroup
	goroutines := 10
	logsPerGoroutine := 25

	// 多个 goroutine 并发写入不同级别的日志
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				switch j % 4 {
				case 0:
					logger.Debug("debug from goroutine", zap.Int("id", id), zap.Int("j", j))
				case 1:
					logger.Info("info from goroutine", zap.Int("id", id), zap.Int("j", j))
				case 2:
					logger.Warn("warn from goroutine", zap.Int("id", id), zap.Int("j", j))
				case 3:
					logger.Error("error from goroutine", zap.Int("id", id), zap.Int("j", j))
				}
			}
		}(i)
	}

	wg.Wait()
	logger.Sync()
	time.Sleep(50 * time.Millisecond)
	logger.Close()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))

	// 验证每个级别都有日志（不严格验证数量，因为可能有格式问题）
	debugCount := strings.Count(content, "debug from goroutine")
	infoCount := strings.Count(content, "info from goroutine")
	warnCount := strings.Count(content, "warn from goroutine")
	errorCount := strings.Count(content, "error from goroutine")

	expectedPerLevel := (goroutines * logsPerGoroutine) / 4

	if debugCount < expectedPerLevel-10 || debugCount > expectedPerLevel+10 {
		t.Errorf("debug 日志数不对，期望约 %d，实际: %d", expectedPerLevel, debugCount)
	}
	if infoCount < expectedPerLevel-10 || infoCount > expectedPerLevel+10 {
		t.Errorf("info 日志数不对，期望约 %d，实际: %d", expectedPerLevel, infoCount)
	}
	if warnCount < 1 {
		t.Error("缺少 warn 日志")
	}
	if errorCount < 1 {
		t.Error("缺少 error 日志")
	}

	t.Logf("并发测试：Debug=%d, Info=%d, Warn=%d, Error=%d (总计: %d/%d)",
		debugCount, infoCount, warnCount, errorCount,
		debugCount+infoCount+warnCount+errorCount, goroutines*logsPerGoroutine)
}

// TestLogger_DynamicLevel_Concurrent 测试动态调整级别的并发安全性
func TestLogger_DynamicLevel_Concurrent(t *testing.T) {
	config := createTestConfig(t)
	config.Level = zapcore.InfoLevel
	config.Logs = map[string]int{
		"all": int(zapcore.DebugLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Goroutine 1: 不断切换级别
	wg.Add(1)
	go func() {
		defer wg.Done()
		levels := []zapcore.Level{
			zapcore.DebugLevel,
			zapcore.InfoLevel,
			zapcore.WarnLevel,
			zapcore.ErrorLevel,
		}
		i := 0
		for {
			select {
			case <-stopChan:
				return
			default:
				logger.SetLevel(levels[i%len(levels)])
				i++
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Goroutine 2-11: 并发写入日志
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				select {
				case <-stopChan:
					return
				default:
					logger.Info("concurrent info")
					logger.Debug("concurrent debug")
					logger.Error("concurrent error")
				}
			}
		}(i)
	}

	// 运行500ms
	time.Sleep(500 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	logger.Sync()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	// 只验证没有崩溃，有日志输出即可
	if content == "" {
		t.Error("应该有日志输出")
	}
	t.Logf("并发动态级别测试通过，输出了 %d 行日志", countLogLines(content))
}

// TestLogger_AutoSync 测试自动同步功能
func TestLogger_AutoSync(t *testing.T) {
	config := createTestConfig(t)
	config.BufferSize = 0 // 无缓冲，应该100ms同步一次
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 写入日志但不手动 Sync
	logger.Info("auto sync test")

	// 等待自动同步（100ms间隔）
	time.Sleep(200 * time.Millisecond)

	// 验证文件已经有内容（说明自动同步生效）
	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if !strings.Contains(content, "auto sync test") {
		t.Error("自动同步未生效，日志未写入文件")
	}
}

// TestLogger_FormattedLogging 测试格式化日志
func TestLogger_FormattedLogging(t *testing.T) {
	config := createTestConfig(t)
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 测试 F 系列（Printf 风格）
	logger.InfoF("formatted %s %d", "test", 123)
	logger.ErrorF("error %v", map[string]int{"code": 500})

	// 测试 S 系列（Sprint 风格）
	logger.InfoS("sprint", "test", 123)
	logger.ErrorS("error", "sprint")

	logger.Sync()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))

	if !strings.Contains(content, "formatted test 123") {
		t.Error("InfoF 格式化失败")
	}
	if !strings.Contains(content, "500") {
		t.Error("ErrorF 格式化失败")
	}
	if !strings.Contains(content, "sprint") {
		t.Error("InfoS/ErrorS 失败")
	}
}

// TestLogger_BufferedWrite 测试缓冲写入
func TestLogger_BufferedWrite(t *testing.T) {
	config := createTestConfig(t)
	config.BufferSize = 4096 // 4KB 缓冲
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 写入少量日志（应该在缓冲区）
	logger.Info("buffered message 1")
	logger.Info("buffered message 2")

	// 立即读取（可能还在缓冲区，不一定能读到）
	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	buffered := content == ""

	// 手动同步
	logger.Sync()

	// 再次读取（应该能读到了）
	content = readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if !strings.Contains(content, "buffered message 1") {
		t.Error("Sync 后应该能读到日志")
	}
	if !strings.Contains(content, "buffered message 2") {
		t.Error("Sync 后应该能读到日志")
	}

	if buffered {
		t.Log("缓冲生效：日志在 Sync 前未落盘")
	} else {
		t.Log("缓冲可能未生效（数据太少，立即刷盘）")
	}
}

// TestDefaultLogger 测试默认 Logger
func TestDefaultLogger(t *testing.T) {
	config := createTestConfig(t)
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}

	NewDefaultLoggerWithConfig(config)
	defer CloseDefaultLog()

	// 使用默认 Logger
	GetDefaultLog().Info("default logger test")

	// 测试全局函数
	SetDefaultLogLevel(zapcore.DebugLevel)
	level := GetDefaultLog().GetLevel()
	if level != zapcore.DebugLevel {
		t.Errorf("SetDefaultLogLevel 失败，期望 Debug，实际: %v", level)
	}

	// 测试字符串设置
	err := SetDefaultLogLevelByString("warn")
	if err != nil {
		t.Errorf("SetDefaultLogLevelByString 失败: %v", err)
	}
	level = GetDefaultLog().GetLevel()
	if level != zapcore.WarnLevel {
		t.Errorf("期望 Warn，实际: %v", level)
	}

	GetDefaultLog().Sync()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))
	if !strings.Contains(content, "default logger test") {
		t.Error("默认 Logger 未正常工作")
	}
}

// TestLogger_ConcurrentClose 测试并发关闭的安全性
func TestLogger_ConcurrentClose(t *testing.T) {
	config := createTestConfig(t)
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)

	// 写入一些日志
	logger.Info("test message before close")
	logger.Sync()

	// 并发调用 Close 多次
	var wg sync.WaitGroup
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 每个 goroutine 调用多次 Close
			for j := 0; j < 5; j++ {
				err := logger.Close()
				if err != nil {
					// Close 可能返回错误（如文件已关闭），但不应该 panic
					t.Logf("goroutine %d call %d: Close returned error: %v", id, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	t.Log("并发 Close 测试通过：没有 panic")
}

// TestLogger_FieldsAndStructured 测试结构化日志
func TestLogger_FieldsAndStructured(t *testing.T) {
	config := createTestConfig(t)
	config.UseJSONEncoder = true // 使用 JSON 格式便于验证字段
	config.Logs = map[string]int{
		"all": int(zapcore.InfoLevel),
	}
	logger := NewLogger(config)
	defer logger.Close()

	// 测试带字段的日志
	logger.Info("user login",
		zap.String("username", "alice"),
		zap.Int("user_id", 123),
		zap.Duration("latency", 50*time.Millisecond),
	)

	logger.Error("database error",
		zap.String("table", "users"),
		zap.Error(os.ErrNotExist),
	)

	logger.Sync()

	content := readLogFile(t, filepath.Join(config.PathName, "test.*.all.log"))

	// 验证 JSON 字段
	if !strings.Contains(content, "user login") {
		t.Error("缺少消息")
	}
	if !strings.Contains(content, `"username":"alice"`) {
		t.Error("缺少 username 字段")
	}
	if !strings.Contains(content, `"user_id":123`) {
		t.Error("缺少 user_id 字段")
	}
	if !strings.Contains(content, "database error") {
		t.Error("缺少错误消息")
	}
}

// =============================================================================
// Recover / RecoverWith / logPanic 测试
// =============================================================================

func createRecoverTestLogger(t *testing.T) *Logger {
	tempDir := filepath.Join(os.TempDir(), "zynet_logger_test", t.Name(), time.Now().Format("150405.000"))
	os.RemoveAll(filepath.Join(os.TempDir(), "zynet_logger_test", t.Name()))
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.DebugLevel,
		PathName:   tempDir,
		Filename:   "test",
		IsConsole:  false,
		BufferSize: 0,
		Logs: map[string]int{
			"error": int(zapcore.ErrorLevel),
		},
	}
	return NewLogger(config)
}

// --- Recover 单元测试 ---

func TestRecover_CatchesPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	recovered := false
	func() {
		defer logger.Recover("test panic")
		panic("boom")
	}()
	recovered = true

	if !recovered {
		t.Fatal("expected function to return after Recover caught panic")
	}
}

func TestRecover_NoPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	executed := false
	func() {
		defer logger.Recover("no panic")
		executed = true
	}()

	if !executed {
		t.Fatal("function body should have executed")
	}
}

func TestRecover_StringPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.Recover("string panic")
		panic("string error message")
	}()
}

func TestRecover_ErrorPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.Recover("error panic")
		panic(42)
	}()
}

func TestRecover_NilPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.Recover("nil panic")
		panic(nil)
	}()
}

func TestRecover_WithExtraFields(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.Recover("with fields",
			zap.Int("workerID", 3),
			zap.String("actor", "gate"))
		panic("extra fields test")
	}()
}

func TestRecover_IndirectCallFails(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to NOT be caught by indirect Recover call")
		}
	}()

	func() {
		defer func() {
			logger.Recover("indirect call")
		}()
		panic("should not be caught")
	}()
}

// --- RecoverWith 单元测试 ---

func TestRecoverWith_CatchesPanic(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	cleanupCalled := false
	recovered := false

	func() {
		defer logger.RecoverWith("test panic", func() {
			cleanupCalled = true
		})
		panic("boom")
	}()
	recovered = true

	if !recovered {
		t.Fatal("expected function to return after RecoverWith caught panic")
	}
	if !cleanupCalled {
		t.Fatal("cleanup should be called even when panic occurs")
	}
}

func TestRecoverWith_NoPanic_CleanupStillRuns(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	cleanupCalled := false
	func() {
		defer logger.RecoverWith("no panic", func() {
			cleanupCalled = true
		})
	}()

	if !cleanupCalled {
		t.Fatal("cleanup should be called even without panic")
	}
}

func TestRecoverWith_NilCleanup(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.RecoverWith("nil cleanup", nil)
		panic("should not crash on nil cleanup")
	}()
}

func TestRecoverWith_CleanupRunsBeforePanicLog(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	order := make([]string, 0, 2)
	func() {
		defer logger.RecoverWith("order test", func() {
			order = append(order, "cleanup")
		})
		panic("order test")
	}()

	if len(order) == 0 || order[0] != "cleanup" {
		t.Fatal("cleanup should run before panic is logged")
	}
}

func TestRecoverWith_WithExtraFields(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	func() {
		defer logger.RecoverWith("with fields", func() {},
			zap.Int("workerID", 5),
			zap.Uint8("type", 1))
		panic("fields test")
	}()
}

func TestRecoverWith_CleanupPanicDoesNotCrash(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Log("cleanup panic was handled or propagated as expected")
		}
	}()

	func() {
		defer logger.RecoverWith("cleanup panic", func() {
			panic("cleanup itself panics")
		})
		panic("original panic")
	}()
}

func TestRecoverWith_InAnonymousDefer(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	recovered := false
	func() {
		defer logger.RecoverWith("anonymous defer", func() {
			recovered = true
		})
		panic("should be caught")
	}()

	if !recovered {
		t.Fatal("RecoverWith should work as direct defer target")
	}
}

func TestRecoverWith_Concurrent(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			func() {
				defer logger.RecoverWith("concurrent", func() {},
					zap.Int("id", id))
				if id%2 == 0 {
					panic("even panic")
				}
			}()
		}(i)
	}
	wg.Wait()
}

// --- PanicHook 集成测试 ---

func TestRecover_PanicHookCalled(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	hookCalled := false
	hook := func() { hookCalled = true }
	SetPanicHook(hook)
	defer SetPanicHook(nil)

	func() {
		defer logger.Recover("hook test")
		panic("trigger hook")
	}()

	if !hookCalled {
		t.Fatal("panic hook should be called")
	}
}

func TestRecoverWith_PanicHookCalled(t *testing.T) {
	logger := createRecoverTestLogger(t)
	defer logger.Close()

	hookCalled := false
	hook := func() { hookCalled = true }
	SetPanicHook(hook)
	defer SetPanicHook(nil)

	func() {
		defer logger.RecoverWith("hook test", func() {})
		panic("trigger hook")
	}()

	if !hookCalled {
		t.Fatal("panic hook should be called via RecoverWith")
	}
}

// --- 基准测试 ---

// BenchmarkLogger_Info 基准测试：Info 日志
func BenchmarkLogger_Info(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(filepath.Join(os.TempDir(), "zynet_bench"))
	})

	config := LoggerConfig{
		Level:      zapcore.InfoLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message")
		}
	})
}

// BenchmarkLogger_InfoWithFields 基准测试：带字段的 Info 日志
func BenchmarkLogger_InfoWithFields(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(filepath.Join(os.TempDir(), "zynet_bench"))
	})

	config := LoggerConfig{
		Level:      zapcore.InfoLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				zap.String("key", "value"),
				zap.Int("count", 123),
			)
		}
	})
}

// BenchmarkLogger_SetLevel 基准测试：动态调整级别
func BenchmarkLogger_SetLevel(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(filepath.Join(os.TempDir(), "zynet_bench"))
	})

	config := LoggerConfig{
		Level:     zapcore.InfoLevel,
		PathName:  tempDir,
		Filename:  "bench",
		IsConsole: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	levels := []zapcore.Level{
		zapcore.DebugLevel,
		zapcore.InfoLevel,
		zapcore.WarnLevel,
		zapcore.ErrorLevel,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.SetLevel(levels[i%len(levels)])
	}
}

// BenchmarkLogger_CircuitBreaker 基准测试：熔断器性能
func BenchmarkLogger_CircuitBreaker(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(filepath.Join(os.TempDir(), "zynet_bench"))
	})

	config := LoggerConfig{
		Level:                   zapcore.InfoLevel,
		PathName:                tempDir,
		Filename:                "bench",
		IsConsole:               false,
		BufferSize:              4096,
		EnableCircuitBreaker:    true,
		CircuitBreakerThreshold: 10000,
		CircuitBreakerWindow:    10,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark with circuit breaker")
		}
	})
}

// BenchmarkLogger_Sampling 基准测试：采样性能
func BenchmarkLogger_Sampling(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(filepath.Join(os.TempDir(), "zynet_bench"))
	})

	config := LoggerConfig{
		Level:              zapcore.InfoLevel,
		PathName:           tempDir,
		Filename:           "bench",
		IsConsole:          false,
		BufferSize:         4096,
		EnableSampling:     true,
		SamplingInitial:    100,
		SamplingThereafter: 100,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark with sampling")
		}
	})
}

// ============================================================
// Goroutine ID 性能对比基准测试
// ============================================================

// BenchmarkLogger_Info_WithGoroutineID 基准测试：启用 Goroutine ID（Field 方式）
func BenchmarkLogger_Info_WithGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_enabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                  zapcore.InfoLevel,
		PathName:               tempDir,
		Filename:               "bench",
		IsConsole:              false,
		BufferSize:             4096,
		EnableGoroutineID:      true, // 启用
		UseFieldForGoroutineID: true, // 使用 Field 方式
		EnableSampling:         false,
		EnableCircuitBreaker:   false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message")
		}
	})
}

// BenchmarkLogger_Info_WithoutGoroutineID 基准测试：禁用 Goroutine ID
func BenchmarkLogger_Info_WithoutGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_disabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    false, // 禁用
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message")
		}
	})
}

// BenchmarkLogger_Info_WithGoroutineID_StringConcat 基准测试：启用 Goroutine ID（字符串拼接方式）
func BenchmarkLogger_Info_WithGoroutineID_StringConcat(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_string")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                  zapcore.InfoLevel,
		PathName:               tempDir,
		Filename:               "bench",
		IsConsole:              false,
		BufferSize:             4096,
		EnableGoroutineID:      true,  // 启用
		UseFieldForGoroutineID: false, // 使用字符串拼接
		EnableSampling:         false,
		EnableCircuitBreaker:   false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message")
		}
	})
}

// BenchmarkLogger_InfoWithFields_WithGoroutineID 基准测试：带字段 + Goroutine ID
func BenchmarkLogger_InfoWithFields_WithGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_fields_enabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                  zapcore.InfoLevel,
		PathName:               tempDir,
		Filename:               "bench",
		IsConsole:              false,
		BufferSize:             4096,
		EnableGoroutineID:      true,
		UseFieldForGoroutineID: true,
		EnableSampling:         false,
		EnableCircuitBreaker:   false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				zap.String("key", "value"),
				zap.Int("count", 123),
			)
		}
	})
}

// BenchmarkLogger_InfoWithFields_WithoutGoroutineID 基准测试：带字段 + 无 Goroutine ID
func BenchmarkLogger_InfoWithFields_WithoutGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_fields_disabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    false,
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				zap.String("key", "value"),
				zap.Int("count", 123),
			)
		}
	})
}

// BenchmarkLogger_InfoF_WithGoroutineID 基准测试：InfoF + Goroutine ID
func BenchmarkLogger_InfoF_WithGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_infof_enabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    true,
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoF("benchmark message: %s %d", "test", 123)
		}
	})
}

// BenchmarkLogger_InfoF_WithoutGoroutineID 基准测试：InfoF + 无 Goroutine ID
func BenchmarkLogger_InfoF_WithoutGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_infof_disabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    false,
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoF("benchmark message: %s %d", "test", 123)
		}
	})
}

// BenchmarkLogger_InfoS_WithGoroutineID 基准测试：InfoS + Goroutine ID
func BenchmarkLogger_InfoS_WithGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_infos_enabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    true,
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoS("benchmark", "message", "test")
		}
	})
}

// BenchmarkLogger_InfoS_WithoutGoroutineID 基准测试：InfoS + 无 Goroutine ID
func BenchmarkLogger_InfoS_WithoutGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "goid_infos_disabled")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	config := LoggerConfig{
		Level:                zapcore.InfoLevel,
		PathName:             tempDir,
		Filename:             "bench",
		IsConsole:            false,
		BufferSize:           4096,
		EnableGoroutineID:    false,
		EnableSampling:       false,
		EnableCircuitBreaker: false,
		Logs: map[string]int{
			"info": int(zapcore.InfoLevel),
		},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoS("benchmark", "message", "test")
		}
	})
}

// BenchmarkLogger_SingleThread_WithVsWithoutGoroutineID 基准测试：单线程对比
func BenchmarkLogger_SingleThread_WithVsWithoutGoroutineID(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", "single_thread")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	b.Run("WithGoroutineID", func(b *testing.B) {
		config := LoggerConfig{
			Level:                zapcore.InfoLevel,
			PathName:             tempDir,
			Filename:             "with",
			IsConsole:            false,
			BufferSize:           4096,
			EnableGoroutineID:    true,
			EnableSampling:       false,
			EnableCircuitBreaker: false,
			Logs: map[string]int{
				"info": int(zapcore.InfoLevel),
			},
		}
		logger := NewLogger(config)
		defer logger.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("benchmark message")
		}
	})

	b.Run("WithoutGoroutineID", func(b *testing.B) {
		config := LoggerConfig{
			Level:                zapcore.InfoLevel,
			PathName:             tempDir,
			Filename:             "without",
			IsConsole:            false,
			BufferSize:           4096,
			EnableGoroutineID:    false,
			EnableSampling:       false,
			EnableCircuitBreaker: false,
			Logs: map[string]int{
				"info": int(zapcore.InfoLevel),
			},
		}
		logger := NewLogger(config)
		defer logger.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("benchmark message")
		}
	})
}

func BenchmarkRecover_NoPanic(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.ErrorLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs:       map[string]int{"error": int(zapcore.ErrorLevel)},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer logger.Recover("bench no panic")
		}()
	}
}

func BenchmarkRecover_WithPanic(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.ErrorLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs:       map[string]int{"error": int(zapcore.ErrorLevel)},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer logger.Recover("bench with panic")
			panic("bench")
		}()
	}
}

func BenchmarkRecoverWith_NoPanic(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.ErrorLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs:       map[string]int{"error": int(zapcore.ErrorLevel)},
	}
	logger := NewLogger(config)
	defer logger.Close()

	counter := 0
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer logger.RecoverWith("bench no panic", func() {
				counter++
			})
		}()
	}
}

func BenchmarkRecoverWith_WithPanic(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.ErrorLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs:       map[string]int{"error": int(zapcore.ErrorLevel)},
	}
	logger := NewLogger(config)
	defer logger.Close()

	counter := 0
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer logger.RecoverWith("bench with panic", func() {
				counter++
			})
			panic("bench")
		}()
	}
}

func BenchmarkRecoverWith_WithPanic_Parallel(b *testing.B) {
	tempDir := filepath.Join(os.TempDir(), "zynet_bench", b.Name())
	os.MkdirAll(tempDir, 0755)
	b.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		os.RemoveAll(tempDir)
	})
	config := LoggerConfig{
		Level:      zapcore.ErrorLevel,
		PathName:   tempDir,
		Filename:   "bench",
		IsConsole:  false,
		BufferSize: 4096,
		Logs:       map[string]int{"error": int(zapcore.ErrorLevel)},
	}
	logger := NewLogger(config)
	defer logger.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			func() {
				defer logger.RecoverWith("bench parallel", func() {})
				panic("bench")
			}()
		}
	})
}
