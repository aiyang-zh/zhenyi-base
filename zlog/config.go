package zlog

import "go.uber.org/zap/zapcore"

// LoggerConfig 描述 Logger 的完整配置项。
// 一般通过 NewDefaultLoggerConfig + Option 组合方式构造。
type LoggerConfig struct {
	Level                   zapcore.Level  `json:"level" toml:"level"`                                     // 全局最低级别（可动态调整），低于此级别的日志会被直接丢弃
	PathName                string         `json:"pathName" toml:"pathName" `                              // 日志文件路径
	Filename                string         `json:"filename" toml:"filename"`                               // 日志文件名前缀
	IsConsole               bool           `json:"isConsole" toml:"isConsole"`                             // 是否输出到控制台
	IsFileNum               bool           `json:"isFileNum" toml:"isFileNum"`                             // 是否显示文件名和行号
	Logs                    map[string]int `json:"logs" toml:"logs"`                                       // 文件配置：key=文件名后缀，value=该文件接收的最低级别
	BufferSize              int            `json:"bufferSize" toml:"bufferSize"`                           // 缓冲大小，0表示不使用缓冲
	UseJSONEncoder          bool           `json:"useJSONEncoder" toml:"useJSONEncoder"`                   // 是否使用JSON编码器
	CallerOnlyError         bool           `json:"callerOnlyError" toml:"callerOnlyError"`                 // 是否只在Error级别及以上获取调用栈
	EnableGoroutineID       bool           `json:"enableGoroutineID" toml:"enableGoroutineID"`             // 是否启用 Goroutine ID（Info 级别下建议关闭以提升性能）
	UseFieldForGoroutineID  bool           `json:"useFieldForGoroutineID" toml:"useFieldForGoroutineID"`   // 是否使用zap Field添加goroutine ID（性能更好，但会改变日志格式）
	EnableSampling          bool           `json:"enableSampling" toml:"enableSampling"`                   // 是否启用采样
	SamplingInitial         int            `json:"samplingInitial" toml:"samplingInitial"`                 // 采样初始值：每秒最多记录多少条日志
	SamplingThereafter      int            `json:"samplingThereafter" toml:"samplingThereafter"`           // 采样后续值：超过initial后，每N条记录1条
	EnableCircuitBreaker    bool           `json:"enableCircuitBreaker" toml:"enableCircuitBreaker"`       // 是否启用熔断（Error级别日志不受限制）
	CircuitBreakerThreshold int            `json:"circuitBreakerThreshold" toml:"circuitBreakerThreshold"` // 熔断阈值：时间窗口内最大日志数
	CircuitBreakerWindow    int            `json:"circuitBreakerWindow" toml:"circuitBreakerWindow"`       // 熔断时间窗口（秒）
	EnableAsync             bool           `json:"enableAsync" toml:"enableAsync"`                         // 是否启用异步写入（极大提升性能，但在崩坏时可能丢失少量日志）
}

// NewDefaultLoggerConfig 返回一份适合大多数服务场景的默认配置。
// 可在此基础上通过 Option 进行微调。
func NewDefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:                   zapcore.DebugLevel,
		PathName:                "./logs",
		Filename:                "app",
		IsConsole:               false,
		IsFileNum:               false,
		BufferSize:              1024,  // 默认1024条缓冲
		UseJSONEncoder:          false, // 默认使用Console编码器
		CallerOnlyError:         true,  // 默认只在Error级别及以上获取调用栈
		EnableGoroutineID:       true,  // 默认启用（Debug级别建议开启，Info级别建议关闭）
		UseFieldForGoroutineID:  true,  // 默认使用字符串拼接（性能更好）
		EnableSampling:          true,  // 默认启用采样
		SamplingInitial:         1000,  // 每秒最多1000条
		SamplingThereafter:      100,   // 超过后每100条记录1条
		EnableCircuitBreaker:    false, // 默认不启用熔断（可选开启）
		CircuitBreakerThreshold: 10000, // 10秒内最多10000条日志
		CircuitBreakerWindow:    10,    // 10秒窗口
		EnableAsync:             true,  // 建议生产环境高并发下开启
		Logs: map[string]int{
			"info":  int(zapcore.InfoLevel),
			"debug": int(zapcore.DebugLevel),
			"warn":  int(zapcore.WarnLevel),
			"error": int(zapcore.ErrorLevel),
		},
	}
}

// WithOptions 在 LoggerConfig 上应用一组 Option。
// 便于在构造时链式组合配置。
func (l *LoggerConfig) WithOptions(opts ...Option) {
	for _, opt := range opts {
		opt.apply(l)
	}
}
