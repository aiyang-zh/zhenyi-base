package zlog

import "go.uber.org/zap"

// 包级别的便捷函数，使用默认 logger

// Info 使用默认 logger 输出 Info 级别日志
func Info(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Info(msg, fields...)
	}
}

// Debug 使用默认 logger 输出 Debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Debug(msg, fields...)
	}
}

// Warn 使用默认 logger 输出 Warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Warn(msg, fields...)
	}
}

// Error 使用默认 logger 输出 Error 级别日志
// ✅ 优化：内部使用 fieldPool 减少分配
func Error(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Error(msg, fields...)
	}
}

// Fatal 使用默认 logger 输出 Fatal 级别日志
func Fatal(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Fatal(msg, fields...)
	}
}

// Panic 使用默认 logger 输出 Panic 级别日志
func Panic(msg string, fields ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Panic(msg, fields...)
	}
}

// InfoF 使用默认 logger 输出格式化 Info 日志
func InfoF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.InfoF(template, args...)
	}
}

// DebugF 使用默认 logger 输出格式化 Debug 日志
func DebugF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.DebugF(template, args...)
	}
}

// WarnF 使用默认 logger 输出格式化 Warn 日志
func WarnF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.WarnF(template, args...)
	}
}

// ErrorF 使用默认 logger 输出格式化 Error 日志
func ErrorF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.ErrorF(template, args...)
	}
}

// FatalF 使用默认 logger 输出格式化 Fatal 日志
func FatalF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.FatalF(template, args...)
	}
}

// PanicF 使用默认 logger 输出格式化 Panic 日志
func PanicF(template string, args ...interface{}) {
	if defaultLog != nil {
		defaultLog.PanicF(template, args...)
	}
}

// Recover 包级别的 Recover 函数（使用默认 logger）
// 内部使用对象池优化，零分配
func Recover(msg string, extra ...zap.Field) {
	if defaultLog != nil {
		defaultLog.Recover(msg, extra...)
	}
}
