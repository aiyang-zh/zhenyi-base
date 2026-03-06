package zlog

import (
	"bufio"
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	zaprotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/petermattis/goid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var defaultLog *Logger

var panicHook atomic.Pointer[func()]

// SetPanicHook 注册 panic 回调（用于 metrics 等外部模块，避免循环依赖）
func SetPanicHook(fn func()) {
	panicHook.Store(&fn)
}

func GetDefaultLog() *Logger {
	return defaultLog
}
func CloneDefaultLog(name string) *Logger {
	return defaultLog.CloneLogger(name)
}

type Logger struct {
	log            *zap.Logger
	logSugar       *zap.SugaredLogger
	logWithCaller  *zap.Logger        // 专门用于 Error/Fatal 的 logger，带有 Caller
	logSugarCaller *zap.SugaredLogger // 对应的 Sugar
	encoder        zapcore.Encoder
	config         LoggerConfig
	writeSyncers   []zapcore.WriteSyncer // 保存所有 writeSyncer，用于关闭时同步
	syncTicker     *time.Ticker          // 定期同步的定时器
	stopChan       chan struct{}         // 停止信号
	atomicLevel    zap.AtomicLevel       // 动态级别控制
	closeOnce      sync.Once             // 确保 Close 操作只执行一次（并发安全）
}

func NewLogger(logConfig LoggerConfig) *Logger {
	l := &Logger{
		config:   logConfig,
		stopChan: make(chan struct{}),
	}
	l.getLogger()

	// 启动定期同步协程
	if logConfig.BufferSize > 0 {
		// 如果开启了缓冲，每秒刷新一次
		l.startAutoSync(5 * time.Second)
	} else {
		// 即使没有应用层缓冲，也定期刷新操作系统缓存
		l.startAutoSync(1 * time.Second)
	}

	return l
}

func NewDefaultLogger() {
	defaultLog = NewLogger(NewDefaultLoggerConfig())
}

// NewLoggerWithOptions 使用基础配置和可选的 Options 创建 Logger
func NewLoggerWithOptions(baseConfig LoggerConfig, opts ...Option) *Logger {
	// 应用所有 Options
	for _, opt := range opts {
		opt.apply(&baseConfig)
	}
	return NewLogger(baseConfig)
}

func NewDefaultLoggerWithConfig(logConfig LoggerConfig) {
	defaultLog = NewLogger(logConfig)
}

// NewDefaultLoggerWithOptions 使用默认配置和可选的 Options 创建默认 Logger
func NewDefaultLoggerWithOptions(opts ...Option) {
	config := NewDefaultLoggerConfig()
	for _, opt := range opts {
		opt.apply(&config)
	}
	NewDefaultLoggerWithConfig(config)
}

func (l *Logger) CloneLogger(name string) *Logger {
	prefix := ""
	if name != "" {
		prefix = fmt.Sprintf("[%s] ", name)
	}
	// 克隆 logger，共享底层的 writeSyncers 和 config
	// 这样 Sync() 和 Close() 可以正常工作
	cloned := &Logger{
		log:            nil,
		logSugar:       nil,
		logWithCaller:  nil,
		logSugarCaller: nil,
		encoder:        l.encoder,      // 共享 encoder
		config:         l.config,       // 共享 config
		writeSyncers:   l.writeSyncers, // 共享 writeSyncers，确保 Sync() 能同步所有日志
		stopChan:       make(chan struct{}),
	}

	if l.config.IsFileNum {
		cloned.log = l.log.Named(prefix).WithOptions(zap.AddCallerSkip(-1))
		cloned.logSugar = l.logSugar.Named(prefix).WithOptions(zap.AddCallerSkip(-1))
		cloned.logSugarCaller = l.logSugarCaller.Named(prefix).WithOptions(zap.AddCallerSkip(-1))
		cloned.logWithCaller = l.logWithCaller.Named(prefix).WithOptions(zap.AddCallerSkip(-1))
	} else {
		cloned.log = l.log.Named(prefix)
		cloned.logSugar = l.logSugar.Named(prefix)
		cloned.logSugarCaller = l.logSugarCaller.Named(prefix)
		cloned.logWithCaller = l.logWithCaller.Named(prefix)
	}

	return cloned
}

// startAutoSync 启动定期同步协程
func (l *Logger) startAutoSync(interval time.Duration) {
	l.syncTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-l.syncTicker.C:
				_ = l.Sync() // 定期刷新缓冲，忽略错误避免日志协程崩溃
			case <-l.stopChan:
				return
			}
		}
	}()
}

// GetLogger 创建日志
func (l *Logger) getLogger() {
	// 初始化 writeSyncers slice
	l.writeSyncers = make([]zapcore.WriteSyncer, 0)

	// 初始化动态级别控制
	l.atomicLevel = zap.NewAtomicLevelAt(l.config.Level)

	encoderConfig := l.encoderConfig()
	// 根据配置选择编码器
	if l.config.UseJSONEncoder {
		l.encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		l.encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	cores := make([]zapcore.Core, 0)
	for k, v := range l.config.Logs {
		core := l.addLogCore(k, zapcore.Level(v))
		cores = append(cores, core)
	}
	if l.config.IsConsole {
		cores = append(cores, l.addConsoleCore())
	}

	// 使用全局 Level 过滤器包装所有 Core
	// 支持动态调整日志级别（级别控制已在 addLogCore 中实现）
	zCore := zapcore.NewTee(cores...)
	// ✅ 不需要 zap.IncreaseLevel，因为底层 Core 已经直接使用 atomicLevel
	//    这样可以确保 SetLevel(Debug) 能真正降低门槛，而不是只能提高
	_log := zap.New(zCore)
	if l.config.Level == zapcore.DebugLevel {
		_log = _log.WithOptions(zap.Development())
	}
	// 优化：根据配置决定是否获取调用栈，以及获取的级别
	if l.config.IsFileNum {
		if l.config.CallerOnlyError {
			// 只在Error级别及以上获取调用栈，使用自定义Core包装
			_log = _log.WithOptions(
				zap.WrapCore(func(core zapcore.Core) zapcore.Core {
					return &callerOnlyErrorCore{Core: core}
				}),
				zap.AddCallerSkip(2),
				zap.AddCaller(),
			)
		} else {
			// 所有级别都获取调用栈
			_log = _log.WithOptions(zap.AddCallerSkip(2), zap.AddCaller())
		}
	}

	// 关键：包装顺序决定执行顺序（先调用的 WrapCore 在外层）
	// 流程：日志 -> 采样（外层，先过滤） -> 熔断（内层，再限流） -> 写入

	// 第一步：先 WrapCore 熔断器（这会成为内层）
	if l.config.EnableCircuitBreaker {
		_log = _log.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return newCircuitBreakerCore(
				core,
				l.config.CircuitBreakerThreshold,
				time.Duration(l.config.CircuitBreakerWindow)*time.Second,
			)
		}))
	}

	// 第二步：再 WrapCore 采样器（这会成为外层，先执行）
	if l.config.EnableSampling {
		_log = _log.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(
				core,
				time.Second,
				l.config.SamplingInitial,    // 每秒最多记录多少条
				l.config.SamplingThereafter, // 超过后每N条记录1条
			)
		}))
	}

	l.logSugar = _log.Sugar()
	l.log = _log
	l.logWithCaller = _log.WithOptions(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zap.ErrorLevel),
	)
	l.logSugarCaller = l.logWithCaller.Sugar()
}

// 编码配置
func (l *Logger) encoderConfig() zapcore.EncoderConfig {
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:       "msg",
		LevelKey:         "level",
		TimeKey:          "time",
		NameKey:          "logger",
		CallerKey:        "caller",
		StacktraceKey:    "stacktrace",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		EncodeCaller:     zapcore.FullCallerEncoder,
		ConsoleSeparator: " ",
	}
	return encoderConfig
}
func (l *Logger) Write(p []byte) (n int, err error) {
	// 去掉末尾的换行符（标准库 logger 往往自带 \n）
	str := strings.TrimSuffix(string(p), "\n")
	if str != "" {
		l.Info(str)
	}
	return len(p), nil
}
func (l *Logger) getLogName(logName string) string {
	return filepath.Join(l.config.PathName, l.config.Filename+".%Y%m%d."+logName+".log")
}

// 创建日志
func (l *Logger) addLogCore(logName string, configLevel zapcore.Level) zapcore.Core {
	fileWriter, _ := zaprotatelogs.New(
		l.getLogName(logName),
		zaprotatelogs.WithMaxAge(7*24*time.Hour),
		zaprotatelogs.WithRotationTime(24*time.Hour),
	)

	// 🔥 核心修复：动态级别的 LevelEnabler 🔥
	levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		// 1. 如果这个文件是专门存 Error 的（配置里 >= Error 级别），
		//    那它永远只存 Error 及以上，不受 SetLevel(Debug) 影响而变脏
		if configLevel >= zapcore.ErrorLevel {
			return lvl >= configLevel
		}

		// 2. 对于普通日志文件（Info/Debug/Warn），使用全局 atomicLevel 控制
		//    这样 SetLevel(Debug) 时，这里就能放行 Debug 日志
		//    SetLevel(Error) 时，这里也会拦截 Info/Debug 日志
		return l.atomicLevel.Enabled(lvl)
	})
	var writeSyncer zapcore.WriteSyncer
	baseSyncer := zapcore.AddSync(fileWriter)
	if l.config.BufferSize > 0 {
		// 使用缓冲写入，提高性能
		writeSyncer = newBufferedWriteSyncer(baseSyncer, l.config.BufferSize)
	} else {
		// 同步写入
		writeSyncer = baseSyncer
	}
	if l.config.EnableAsync && configLevel < zapcore.ErrorLevel {
		// channel 大小可以设大一点，比如 4096
		writeSyncer = newAsyncWriteSyncer(writeSyncer, 4096)
	}

	// 保存 writeSyncer 引用，用于关闭时同步
	l.writeSyncers = append(l.writeSyncers, writeSyncer)
	return zapcore.NewCore(l.encoder, writeSyncer, levelEnabler)
}

func (l *Logger) addConsoleCore() zapcore.Core {
	// 控制台输出也使用 atomicLevel 动态控制
	levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return l.atomicLevel.Enabled(lvl)
	})
	consoleSyncer := zapcore.AddSync(os.Stdout)
	// 控制台输出通常不需要在关闭时同步，但为了完整性也保存
	// l.writeSyncers = append(l.writeSyncers, consoleSyncer)
	return zapcore.NewCore(l.encoder, consoleSyncer, levelEnabler)
}

func (l *Logger) InfoF(template string, args ...interface{}) {
	if l.config.EnableGoroutineID {
		l.logSugar.Infof(l.buildMessage(template), args...)
	} else {
		l.logSugar.Infof(template, args...)
	}
}
func (l *Logger) DebugF(template string, args ...interface{}) {
	if l.config.EnableGoroutineID {
		l.logSugar.Debugf(l.buildMessage(template), args...)
	} else {
		l.logSugar.Debugf(template, args...)
	}
}
func (l *Logger) WarnF(template string, args ...interface{}) {
	if l.config.EnableGoroutineID {
		l.logSugar.Warnf(l.buildMessage(template), args...)
	} else {
		l.logSugar.Warnf(template, args...)
	}
}
func (l *Logger) ErrorF(template string, args ...interface{}) {
	if l.config.EnableGoroutineID {
		l.logSugarCaller.Errorf(l.buildMessage(template), args...)
	} else {
		l.logSugarCaller.Errorf(template, args...)
	}
}
func (l *Logger) FatalF(template string, args ...interface{}) {
	// Fatal 前强制刷盘
	defer l.Sync()
	if l.config.EnableGoroutineID {
		l.logSugarCaller.Fatalf(l.buildMessage(template), args...)
	} else {
		l.logSugarCaller.Fatalf(template, args...)
	}
}
func (l *Logger) PanicF(template string, args ...interface{}) {
	// Panic 前强制刷盘
	defer l.Sync()
	if l.config.EnableGoroutineID {
		l.logSugarCaller.Panicf(l.buildMessage(template), args...)
	} else {
		l.logSugarCaller.Panicf(template, args...)
	}
}

// 优化：使用 sync.Pool 复用 slice，减少内存分配
func (l *Logger) InfoS(args ...interface{}) {
	if !l.config.EnableGoroutineID {
		l.logSugar.Info(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0] // 重置长度但保留容量
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugar.Info(newArgs...)
	// 如果slice容量没有增长太多，可以放回pool复用
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}
func (l *Logger) DebugS(args ...interface{}) {
	if !l.config.EnableGoroutineID {
		l.logSugar.Debug(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0]
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugar.Debug(newArgs...)
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}
func (l *Logger) WarnS(args ...interface{}) {
	if !l.config.EnableGoroutineID {
		l.logSugar.Warn(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0]
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugar.Warn(newArgs...)
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}
func (l *Logger) ErrorS(args ...interface{}) {
	if !l.config.EnableGoroutineID {
		l.logSugarCaller.Error(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0]
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugarCaller.Error(newArgs...)
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}
func (l *Logger) FatalS(args ...interface{}) {
	// Fatal 前强制刷盘
	defer l.Sync()

	if !l.config.EnableGoroutineID {
		l.logSugarCaller.Fatal(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0]
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugarCaller.Fatal(newArgs...)
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}

func (l *Logger) PanicS(args ...interface{}) {
	// Panic 前强制刷盘
	defer l.Sync()

	if !l.config.EnableGoroutineID {
		l.logSugarCaller.Panic(args...)
		return
	}

	newArgs := slicePool.Get()
	newArgs = newArgs[:0]
	newArgs = append(newArgs, l.goId())
	newArgs = append(newArgs, args...)
	l.logSugarCaller.Panic(newArgs...)
	if cap(newArgs) <= 32 {
		slicePool.Put(newArgs)
	}
}

// 优化：使用Builder一次性构建完整消息，避免字符串拼接产生的额外分配
func (l *Logger) buildMessage(msg string) string {
	builder := goIdBuilderPool.Get()
	defer func() {
		builder.Reset()
		goIdBuilderPool.Put(builder)
	}()

	// 预分配足够空间：goroutine ID (~20字节) + 消息长度
	builder.Grow(20 + len(msg))
	builder.WriteString("[goroutine:")
	builder.WriteString(strconv.FormatInt(int64(goid.Get()), 10))
	builder.WriteString("]")
	builder.WriteString(msg)
	return builder.String()
}

// fieldPool 用于复用 []zap.Field slice，减少内存分配
var fieldPool = zpool.NewPool(func() []zap.Field {
	return make([]zap.Field, 0, 4) // 预分配容量4
})

func (l *Logger) Info(msg string, fields ...zap.Field) {
	if !l.config.EnableGoroutineID {
		// 不启用 Goroutine ID，直接输出
		l.log.Info(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.log.Info(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		// 使用字符串拼接，保持原有格式
		l.log.Info(l.buildMessage(msg), fields...)
	}
}
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	if !l.config.EnableGoroutineID {
		l.log.Debug(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.log.Debug(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		l.log.Debug(l.buildMessage(msg), fields...)
	}
}
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	if !l.config.EnableGoroutineID {
		l.log.Warn(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.log.Warn(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		l.log.Warn(l.buildMessage(msg), fields...)
	}
}
func (l *Logger) Error(msg string, fields ...zap.Field) {
	// 如果配置了CallerOnlyError，这里会自动获取调用栈（通过zap配置）
	if !l.config.EnableGoroutineID {
		l.logWithCaller.Error(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.logWithCaller.Error(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		l.logWithCaller.Error(l.buildMessage(msg), fields...)
	}
}
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	// Fatal 前强制刷盘，防止进程退出时日志丢失
	defer l.Sync()

	if !l.config.EnableGoroutineID {
		l.logWithCaller.Fatal(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.logWithCaller.Fatal(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		l.logWithCaller.Fatal(l.buildMessage(msg), fields...)
	}
}

func (l *Logger) Panic(msg string, fields ...zap.Field) {
	// Panic 前强制刷盘，防止崩溃时最后一条日志丢失
	defer l.Sync()

	if !l.config.EnableGoroutineID {
		l.logWithCaller.Panic(msg, fields...)
		return
	}

	if l.config.UseFieldForGoroutineID {
		var newFields []zap.Field
		if len(fields) == 0 {
			newFields = make([]zap.Field, 1, 1)
			newFields[0] = zap.Int("goroutine", int(goid.Get()))
		} else {
			newFields = fieldPool.Get()
			newFields = newFields[:0]
			newFields = append(newFields, zap.Int("goroutine", int(goid.Get())))
			newFields = append(newFields, fields...)
		}
		l.logWithCaller.Panic(msg, newFields...)
		if len(fields) > 0 && cap(newFields) <= 16 {
			fieldPool.Put(newFields)
		}
	} else {
		l.logWithCaller.Panic(l.buildMessage(msg), fields...)
	}
}

// recoverFieldsPool 用于 Recover() 中的 Field 数组池
var recoverFieldsPool = zpool.NewPool(func() []zap.Field {
	return make([]zap.Field, 0, 4) // 基础 3 个 + 额外
})

// Recover panic 恢复方法（使用对象池优化，零分配）
// 注意：必须作为 defer 的直接调用才能生效，即 defer logger.Recover(...)
// 不可在 defer func() { logger.Recover(...) }() 中使用，否则 recover() 返回 nil
func (l *Logger) Recover(msg string, extra ...zap.Field) {
	if err := recover(); err != nil {
		l.logPanic(err, msg, extra...)
	}
}

// RecoverWith 带 cleanup 回调的 panic 恢复方法
// cleanup 无论是否发生 panic 都会执行，适用于需要在同一个 defer 中做清理工作的场景
// 用法：defer logger.RecoverWith("msg", func() { /* metrics/cleanup */ }, extraFields...)
func (l *Logger) RecoverWith(msg string, cleanup func(), extra ...zap.Field) {
	err := recover()

	if cleanup != nil {
		cleanup()
	}

	if err != nil {
		l.logPanic(err, msg, extra...)
	}
}

// logPanic 内部方法：记录 panic 日志（从 Recover/RecoverWith 共享）
func (l *Logger) logPanic(err interface{}, msg string, extra ...zap.Field) {
	stackBuf := zpool.GetBytesBuffer(2048)
	defer stackBuf.Release()

	n := runtime.Stack(stackBuf.B, false)
	shortStack := string(stackBuf.B[:n])

	logFields := recoverFieldsPool.Get()
	logFields = logFields[:0]
	defer func() {
		if cap(logFields) <= 8 {
			recoverFieldsPool.Put(logFields)
		}
	}()

	logFields = append(logFields,
		zap.String("panic_loc", msg),
		zap.Any("error", err),
		zap.String("stack", shortStack),
	)

	if len(extra) > 0 {
		logFields = append(logFields, extra...)
	}

	l.Error("🔥 PANIC RECOVERED", logFields...)

	if onPanic := panicHook.Load(); onPanic != nil && *onPanic != nil {
		(*onPanic)()
	}
}

// 优化：使用更高效的字符串构建方式
var goIdBuilderPool = zpool.NewPool(func() *strings.Builder {
	return &strings.Builder{}
})

// slicePool 用于复用 []interface{} slice，减少内存分配
var slicePool = zpool.NewPool(func() []interface{} {
	return make([]interface{}, 0, 8) // 预分配容量8
})

func (l *Logger) goId() string {
	builder := goIdBuilderPool.Get()
	defer func() {
		builder.Reset()
		goIdBuilderPool.Put(builder)
	}()

	builder.Grow(20) // 预分配空间，减少扩容
	builder.WriteString("[goroutine:")
	builder.WriteString(strconv.FormatInt(int64(goid.Get()), 10))
	builder.WriteString("]")
	return builder.String()
}

// callerOnlyErrorCore 只在Error级别及以上获取调用栈的Core包装器
type callerOnlyErrorCore struct {
	zapcore.Core
}

func (c *callerOnlyErrorCore) With(fields []zap.Field) zapcore.Core {
	return &callerOnlyErrorCore{Core: c.Core.With(fields)}
}

func (c *callerOnlyErrorCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	// 对于非Error级别，清除调用栈信息
	if ent.Level < zapcore.ErrorLevel {
		ent.Caller = zapcore.EntryCaller{}
	}
	return c.Core.Write(ent, fields)
}

// bufferedWriteSyncer 实现带缓冲的 WriteSyncer，提高写入性能
type bufferedWriteSyncer struct {
	writer *bufio.Writer
	mu     sync.Mutex
	ws     zapcore.WriteSyncer
}

// newBufferedWriteSyncer 创建新的缓冲 WriteSyncer
func newBufferedWriteSyncer(ws zapcore.WriteSyncer, bufferSize int) zapcore.WriteSyncer {
	// zapcore.WriteSyncer 实现了 io.Writer 接口，可以直接使用
	return &bufferedWriteSyncer{
		writer: bufio.NewWriterSize(ws, bufferSize),
		ws:     ws,
	}
}

// Write 实现 io.Writer 接口
func (b *bufferedWriteSyncer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.writer.Write(p)
}

// Sync 实现 zapcore.WriteSyncer 接口，刷新缓冲区
func (b *bufferedWriteSyncer) Sync() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.writer.Flush(); err != nil {
		return err
	}
	return b.ws.Sync()
}

// Close 关闭写入器（如果实现了 io.Closer）
func (b *bufferedWriteSyncer) Close() error {
	if err := b.Sync(); err != nil {
		return err
	}
	if closer, ok := b.ws.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Sync 刷新所有缓冲的日志到磁盘
// 在服务停止前调用此方法，确保所有日志都已写入磁盘
func (l *Logger) Sync() error {
	if l.log != nil {
		// 先调用 zap 的 Sync，这会刷新所有 core
		if err := l.log.Sync(); err != nil {
			return err
		}
	}
	// 额外确保所有 writeSyncer 都已同步
	for _, ws := range l.writeSyncers {
		if err := ws.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭 logger，刷新所有缓冲的日志
// 在服务停止时调用此方法，确保所有日志都已写入磁盘
func (l *Logger) Close() error {
	// 使用 sync.Once 确保关闭操作只执行一次
	// 无论是串行多次调用还是并发调用，都绝对安全
	var closeErr error
	l.closeOnce.Do(func() {
		// 停止定期同步协程
		if l.syncTicker != nil {
			l.syncTicker.Stop()
		}
		// 关闭停止信号 channel（sync.Once 保证只执行一次，不会 panic）
		close(l.stopChan)

		// 最后一次同步所有日志
		if err := l.Sync(); err != nil {
			closeErr = err
			return
		}

		// 关闭所有 writeSyncer（如果实现了 io.Closer）
		for _, ws := range l.writeSyncers {
			if closer, ok := ws.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					closeErr = err
					return
				}
			}
		}
	})

	return closeErr
}

// SyncDefaultLog 同步默认 logger 的所有缓冲日志
func SyncDefaultLog() error {
	if defaultLog != nil {
		return defaultLog.Sync()
	}
	return nil
}

// CloseDefaultLog 关闭默认 logger，刷新所有缓冲的日志
func CloseDefaultLog() error {
	if defaultLog != nil {
		return defaultLog.Close()
	}
	return nil
}

// circuitBreakerState 熔断器共享状态（所有 Core 实例共享）
type circuitBreakerState struct {
	counter      int64 // 当前时间窗口的日志计数
	droppedCount int64 // 丢弃的日志数
	windowStart  int64 // 时间窗口开始时间（UnixNano）
	threshold    int64 // 熔断阈值
	windowSize   int64 // 时间窗口大小（纳秒）
}

// circuitBreakerCore 实现熔断机制的 Core 包装器（无锁实现）
type circuitBreakerCore struct {
	zapcore.Core
	state *circuitBreakerState // 指向共享状态
}

// newCircuitBreakerCore 创建熔断 Core（无锁、共享状态）
func newCircuitBreakerCore(core zapcore.Core, threshold int, window time.Duration) zapcore.Core {
	return &circuitBreakerCore{
		Core: core,
		state: &circuitBreakerState{
			windowStart: time.Now().UnixNano(),
			threshold:   int64(threshold),
			windowSize:  int64(window),
		},
	}
}

// With 创建子 Logger 时共享状态（修复深拷贝陷阱）
func (c *circuitBreakerCore) With(fields []zap.Field) zapcore.Core {
	return &circuitBreakerCore{
		Core:  c.Core.With(fields),
		state: c.state, // 共享状态指针，不拷贝值
	}
}

// Check 无锁检查是否应该记录日志
func (c *circuitBreakerCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// Error 及以上级别的日志永不熔断
	if ent.Level >= zapcore.ErrorLevel {
		return c.Core.Check(ent, ce)
	}

	now := time.Now().UnixNano()
	windowStart := atomic.LoadInt64(&c.state.windowStart)

	// 检查是否需要重置时间窗口（无锁 CAS）
	if now-windowStart >= c.state.windowSize {
		// 尝试 CAS 更新窗口，只有一个 goroutine 会成功
		if atomic.CompareAndSwapInt64(&c.state.windowStart, windowStart, now) {
			// 重置成功，清空计数器
			atomic.StoreInt64(&c.state.counter, 0)
			atomic.StoreInt64(&c.state.droppedCount, 0)
		}
	}

	// 原子递增计数器
	count := atomic.AddInt64(&c.state.counter, 1)

	// 超过阈值，触发熔断
	if count > c.state.threshold {
		atomic.AddInt64(&c.state.droppedCount, 1)
		// 直接返回 ce，不调用 AddCore，表示本层不记录
		return ce
	}

	// 通过检查，委托给内层 Core
	return c.Core.Check(ent, ce)
}

func (c *circuitBreakerCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	return c.Core.Write(ent, fields)
}

// GetDroppedCount 获取当前窗口丢弃的日志数
func (c *circuitBreakerCore) GetDroppedCount() int64 {
	return atomic.LoadInt64(&c.state.droppedCount)
}

// GetCurrentCount 获取当前窗口的日志数
func (c *circuitBreakerCore) GetCurrentCount() int64 {
	return atomic.LoadInt64(&c.state.counter)
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level zapcore.Level) {
	l.atomicLevel.SetLevel(level)
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() zapcore.Level {
	return l.atomicLevel.Level()
}

// SetLevelByString 通过字符串设置日志级别
func (l *Logger) SetLevelByString(level string) error {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return err
	}
	l.atomicLevel.SetLevel(lvl)
	return nil
}

// SetDefaultLogLevel 设置默认 logger 的级别
func SetDefaultLogLevel(level zapcore.Level) {
	if defaultLog != nil {
		defaultLog.SetLevel(level)
	}
}

// SetDefaultLogLevelByString 通过字符串设置默认 logger 的级别
func SetDefaultLogLevelByString(level string) error {
	if defaultLog != nil {
		return defaultLog.SetLevelByString(level)
	}
	return nil
}
