package zlog

import "go.uber.org/zap/zapcore"

// Option 日志配置选项接口
type Option interface {
	apply(config *LoggerConfig)
}

// optionFunc 函数式 Option 实现
type optionFunc func(*LoggerConfig)

func (f optionFunc) apply(config *LoggerConfig) {
	f(config)
}

// ============================================================
// 基础配置 Options
// ============================================================

// WithLevel 设置全局日志级别
func WithLevel(level zapcore.Level) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.Level = level
	})
}

// WithAsync 启用异步写入
func WithAsync() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableAsync = true
	})
}

// WithLevelString 通过字符串设置日志级别
func WithLevelString(level string) Option {
	return optionFunc(func(config *LoggerConfig) {
		var lvl zapcore.Level
		_ = lvl.UnmarshalText([]byte(level))
		config.Level = lvl
	})
}

// WithPathName 设置日志文件路径
func WithPathName(pathName string) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.PathName = pathName
	})
}

// WithFilename 设置日志文件名前缀
func WithFilename(filename string) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.Filename = filename
	})
}

// WithConsole 设置是否输出到控制台
func WithConsole(enabled bool) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.IsConsole = enabled
	})
}

// WithFileNum 设置是否显示文件名和行号
func WithFileNum(enabled bool) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.IsFileNum = enabled
	})
}

// WithCallerOnlyError 设置是否仅在 Error 级别显示调用栈
func WithCallerOnlyError(enabled bool) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.CallerOnlyError = enabled
	})
}

// WithGoroutineID 设置是否启用 Goroutine ID
func WithGoroutineID(enabled bool) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableGoroutineID = enabled
	})
}

// WithoutGoroutineID 禁用 Goroutine ID（适合 Info 级别）
func WithoutGoroutineID() Option {
	return WithGoroutineID(false)
}

// WithGoroutineIDField 设置是否使用 Field 方式记录 Goroutine ID
func WithGoroutineIDField(enabled bool) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.UseFieldForGoroutineID = enabled
	})
}

// ============================================================
// 编码和格式 Options
// ============================================================

// WithJSONEncoder 设置使用 JSON 编码器
func WithJSONEncoder() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.UseJSONEncoder = true
	})
}

// WithConsoleEncoder 设置使用控制台编码器
func WithConsoleEncoder() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.UseJSONEncoder = false
	})
}

// ============================================================
// 性能优化 Options
// ============================================================

// WithBufferSize 设置缓冲区大小（字节）
func WithBufferSize(size int) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.BufferSize = size
	})
}

// WithBuffer 启用默认缓冲（4KB）
func WithBuffer() Option {
	return WithBufferSize(4096)
}

// WithoutBuffer 禁用缓冲，立即刷盘
func WithoutBuffer() Option {
	return WithBufferSize(0)
}

// ============================================================
// 采样 Options
// ============================================================

// WithSampling 启用采样
func WithSampling(initial, thereafter int) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableSampling = true
		config.SamplingInitial = initial
		config.SamplingThereafter = thereafter
	})
}

// WithDefaultSampling 启用默认采样配置（前1000条必出，之后每100条出1条）
func WithDefaultSampling() Option {
	return WithSampling(1000, 100)
}

// WithAggressiveSampling 启用激进采样（高流量场景，前100条必出，之后每1000条出1条）
func WithAggressiveSampling() Option {
	return WithSampling(100, 1000)
}

// WithoutSampling 禁用采样
func WithoutSampling() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableSampling = false
	})
}

// ============================================================
// 熔断器 Options
// ============================================================

// WithCircuitBreaker 启用熔断器
func WithCircuitBreaker(threshold, windowSeconds int) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableCircuitBreaker = true
		config.CircuitBreakerThreshold = threshold
		config.CircuitBreakerWindow = windowSeconds
	})
}

// WithDefaultCircuitBreaker 启用默认熔断器（10秒内最多10000条）
func WithDefaultCircuitBreaker() Option {
	return WithCircuitBreaker(10000, 10)
}

// WithStrictCircuitBreaker 启用严格熔断器（10秒内最多1000条）
func WithStrictCircuitBreaker() Option {
	return WithCircuitBreaker(1000, 10)
}

// WithoutCircuitBreaker 禁用熔断器
func WithoutCircuitBreaker() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.EnableCircuitBreaker = false
	})
}

// ============================================================
// 日志文件配置 Options
// ============================================================

// WithLogFile 添加单个日志文件
func WithLogFile(name string, level zapcore.Level) Option {
	return optionFunc(func(config *LoggerConfig) {
		if config.Logs == nil {
			config.Logs = make(map[string]int)
		}
		config.Logs[name] = int(level)
	})
}

// WithLogFiles 批量添加日志文件
func WithLogFiles(logs map[string]zapcore.Level) Option {
	return optionFunc(func(config *LoggerConfig) {
		if config.Logs == nil {
			config.Logs = make(map[string]int)
		}
		for name, level := range logs {
			config.Logs[name] = int(level)
		}
	})
}

// WithStandardLogFiles 使用标准日志文件配置（info, error）
func WithStandardLogFiles() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.Logs = map[string]int{
			"info":  int(zapcore.InfoLevel),
			"error": int(zapcore.ErrorLevel),
		}
	})
}

// WithAllLevelLogFiles 使用全级别日志文件配置（debug, info, warn, error）
func WithAllLevelLogFiles() Option {
	return optionFunc(func(config *LoggerConfig) {
		config.Logs = map[string]int{
			"debug": int(zapcore.DebugLevel),
			"info":  int(zapcore.InfoLevel),
			"warn":  int(zapcore.WarnLevel),
			"error": int(zapcore.ErrorLevel),
		}
	})
}

// WithSingleLogFile 使用单一日志文件（所有级别写入同一文件）
func WithSingleLogFile(name string) Option {
	return optionFunc(func(config *LoggerConfig) {
		config.Logs = map[string]int{
			name: int(zapcore.DebugLevel),
		}
	})
}

// ============================================================
// 预设配置组合 Options
// ============================================================

// WithDevelopment 开发环境预设配置
// - Debug 级别
// - 输出到控制台
// - 显示文件名和行号
// - 启用 Goroutine ID（方便调试）
// - 使用控制台编码器
// - 无缓冲（立即刷盘）
// - 全级别日志文件
func WithDevelopment() Option {
	return optionFunc(func(config *LoggerConfig) {
		WithLevel(zapcore.DebugLevel).apply(config)
		WithConsole(true).apply(config)
		WithFileNum(true).apply(config)
		WithGoroutineID(true).apply(config)
		WithConsoleEncoder().apply(config)
		WithoutBuffer().apply(config)
		WithAllLevelLogFiles().apply(config)
	})
}

// WithProduction 生产环境预设配置
// - Info 级别
// - 不输出到控制台
// - 仅 Error 显示调用栈
// - 禁用 Goroutine ID（提升性能）
// - 使用 JSON 编码器
// - 启用缓冲
// - 标准日志文件（info, error）
// - 启用默认采样
// - 启用默认熔断器
func WithProduction() Option {
	return optionFunc(func(config *LoggerConfig) {
		WithLevel(zapcore.InfoLevel).apply(config)
		WithConsole(false).apply(config)
		WithCallerOnlyError(true).apply(config)
		WithoutGoroutineID().apply(config)
		WithJSONEncoder().apply(config)
		WithBuffer().apply(config)
		WithStandardLogFiles().apply(config)
		WithDefaultSampling().apply(config)
		WithDefaultCircuitBreaker().apply(config)
	})
}

// WithTest 测试环境预设配置
// - Debug 级别
// - 输出到控制台
// - 不显示文件名和行号（简化输出）
// - 启用 Goroutine ID（方便测试调试）
// - 使用控制台编码器
// - 无缓冲
// - 单一日志文件
func WithTest() Option {
	return optionFunc(func(config *LoggerConfig) {
		WithLevel(zapcore.DebugLevel).apply(config)
		WithConsole(true).apply(config)
		WithFileNum(false).apply(config)
		WithGoroutineID(true).apply(config)
		WithConsoleEncoder().apply(config)
		WithoutBuffer().apply(config)
		WithSingleLogFile("test").apply(config)
	})
}

// WithHighPerformance 高性能预设配置
// - Warn 级别（减少日志量）
// - 不输出到控制台
// - 不显示文件名和行号
// - 禁用 Goroutine ID（极致性能）
// - 使用 JSON 编码器
// - 启用大缓冲（8KB）
// - 标准日志文件
// - 启用激进采样
// - 启用严格熔断器
func WithHighPerformance() Option {
	return optionFunc(func(config *LoggerConfig) {
		WithLevel(zapcore.WarnLevel).apply(config)
		WithConsole(false).apply(config)
		WithFileNum(false).apply(config)
		WithoutGoroutineID().apply(config)
		WithJSONEncoder().apply(config)
		WithBufferSize(8192).apply(config)
		WithStandardLogFiles().apply(config)
		WithAggressiveSampling().apply(config)
		WithStrictCircuitBreaker().apply(config)
	})
}

// WithStaging 预发布环境预设配置
// - Info 级别
// - 输出到控制台（便于查看）
// - 显示文件名和行号
// - 禁用 Goroutine ID（与生产环境一致）
// - 使用 JSON 编码器
// - 启用缓冲
// - 全级别日志文件
// - 不启用采样（保留完整日志）
// - 启用宽松熔断器（30秒内最多50000条）
func WithStaging() Option {
	return optionFunc(func(config *LoggerConfig) {
		WithLevel(zapcore.InfoLevel).apply(config)
		WithConsole(true).apply(config)
		WithFileNum(true).apply(config)
		WithoutGoroutineID().apply(config)
		WithJSONEncoder().apply(config)
		WithBuffer().apply(config)
		WithAllLevelLogFiles().apply(config)
		WithoutSampling().apply(config)
		WithCircuitBreaker(50000, 30).apply(config)
	})
}

// ============================================================
// 便捷 Options
// ============================================================

// WithDebug 快捷启用 Debug 模式
func WithDebug() Option {
	return WithLevel(zapcore.DebugLevel)
}

// WithInfo 快捷设置 Info 级别
func WithInfo() Option {
	return WithLevel(zapcore.InfoLevel)
}

// WithWarn 快捷设置 Warn 级别
func WithWarn() Option {
	return WithLevel(zapcore.WarnLevel)
}

// WithError 快捷设置 Error 级别
func WithError() Option {
	return WithLevel(zapcore.ErrorLevel)
}

// WithLargeBuffer 使用大缓冲（16KB），适合高流量场景
func WithLargeBuffer() Option {
	return WithBufferSize(16384)
}
