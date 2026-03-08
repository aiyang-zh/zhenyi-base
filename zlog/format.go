package zlog

import (
	"runtime"

	"go.uber.org/zap"
)

// 包级别的便捷函数，基于默认 logger 封装常用日志入口。

// Info 使用默认 logger 输出 Info 级别日志。
func Info(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Info(msg, fields...)
	}
}

// Debug 使用默认 logger 输出 Debug 级别日志。
func Debug(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Debug(msg, fields...)
	}
}

// Warn 使用默认 logger 输出 Warn 级别日志。
func Warn(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Warn(msg, fields...)
	}
}

// Error 使用默认 logger 输出 Error 级别日志。
// 内部会使用带 caller 的 Logger 并结合 fieldPool 减少分配。
func Error(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Error(msg, fields...)
	}
}

// Fatal 使用默认 logger 输出 Fatal 级别日志。
func Fatal(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Fatal(msg, fields...)
	}
}

// Panic 使用默认 logger 输出 Panic 级别日志。
func Panic(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Panic(msg, fields...)
	}
}

// InfoF 使用默认 logger 输出格式化 Info 日志。
func InfoF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.InfoF(template, args...)
	}
}

// DebugF 使用默认 logger 输出格式化 Debug 日志。
func DebugF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.DebugF(template, args...)
	}
}

// WarnF 使用默认 logger 输出格式化 Warn 日志。
func WarnF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.WarnF(template, args...)
	}
}

// ErrorF 使用默认 logger 输出格式化 Error 日志。
func ErrorF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.ErrorF(template, args...)
	}
}

// FatalF 使用默认 logger 输出格式化 Fatal 日志。
func FatalF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.FatalF(template, args...)
	}
}

// PanicF 使用默认 logger 输出格式化 Panic 日志。
func PanicF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.PanicF(template, args...)
	}
}

// Recover 包级别的 Recover 函数（使用默认 logger）。
// defaultLog 未初始化时 fallback 到 stderr 输出，保证 panic 可见。
// 必须直接作为 defer 调用的目标。
func Recover(msg string, extra ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Recover(msg, extra...)
		return
	}
	if err := recover(); err != nil {
		stackBuf := make([]byte, 2048)
		n := runtime.Stack(stackBuf, false)
		println("zlog.Recover:", msg, "err=", err)
		println(string(stackBuf[:n]))
	}
}
