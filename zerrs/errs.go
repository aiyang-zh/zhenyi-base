// Package zerrs 提供带类型分类和可选堆栈的结构化错误处理。
//
// 与标准库 errors 包完全兼容（支持 errors.Is / errors.As / errors.Unwrap），
// 同时扩展了两项核心能力：
//
//   - ErrorType —— 对错误进行语义分类（TIMEOUT / NETWORK / DATABASE …），
//     使上层可以用 [IsType] 做统一的分支判断，而非散落的 errors.Is。
//   - 可选堆栈 —— [WithStack] 系列函数自动捕获调用栈，
//     通过 fmt.Formatter（%+v）输出与 pkg/errors 一致的堆栈格式。
//
// # 创建错误
//
// 按是否需要堆栈，选择不同的构造函数：
//
//	err := errs.New(errs.ErrTypeTimeout, "operation timeout")      // 无堆栈，热路径友好
//	err := errs.WithStack(errs.ErrTypeInternal, "unexpected nil")  // 带堆栈，用于非预期错误
//	err := errs.Newf(errs.ErrTypeNetwork, "port %d refused", 80)   // 格式化变体
//
// 包装已有错误：
//
//	err := errs.Wrap(ioErr, errs.ErrTypeNetwork, "read failed")
//	err := errs.WrapWithStack(dbErr, errs.ErrTypeDatabase, "query user")
//
// # 堆栈控制
//
// [WithStackSkip] 允许在辅助函数内部创建错误时跳过额外帧，
// 使堆栈起点指向真正的调用方：
//
//	func myHelper() error {
//	    return errs.WithStackSkip(1, errs.ErrTypeInternal, "oops")
//	}
//
// # 格式化输出
//
// TypedError 实现 [fmt.Formatter]，与 pkg/errors 的 %+v 习惯一致：
//
//	fmt.Printf("%v\n",  err)  // [INTERNAL] unexpected nil
//	fmt.Printf("%+v\n", err)  // [INTERNAL] unexpected nil
//	                           // main.doWork
//	                           //     /app/main.go:42
//	                           // main.main
//	                           //     /app/main.go:15
//
// # 类型判断
//
//	if errs.IsTimeout(err) { … }                     // 便捷函数
//	if errs.IsType(err, errs.ErrTypeDatabase) { … }  // 通用判断
//	typ := errs.GetType(err)                          // 获取最外层类型
//
// 也可通过标准库 errors.Is 按类型匹配（[TypedError.Is] 仅比较 Type 字段）：
//
//	errors.Is(err, &errs.TypedError{Type: errs.ErrTypeTimeout})
//
// # Sentinel Errors
//
// common.go 中预定义了常用 sentinel 错误（如 [ErrTimeout]、[ErrNotFound]），
// 它们均为 TypedError 实例，可直接用于 errors.Is 比较，也支持 IsType 类型判断。
//
// # 性能
//
// 无堆栈的 New / Wrap 系列每次约 20-25 ns / 1 alloc（80 B），适合热路径。
// 带堆栈的 WithStack 系列约 220 ns / 2 allocs（336 B），仅在需要诊断信息时使用。
// IsType / GetType 均为零分配，< 2 ns。
package zerrs

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ErrorType 定义错误的语义类别。
// 同一 ErrorType 的不同 TypedError 实例在 errors.Is 比较时视为匹配。
type ErrorType string

const (
	ErrTypeUnknown       ErrorType = "UNKNOWN"
	ErrTypeNetwork       ErrorType = "NETWORK"
	ErrTypeDatabase      ErrorType = "DATABASE"
	ErrTypeValidation    ErrorType = "VALIDATION"
	ErrTypeTimeout       ErrorType = "TIMEOUT"
	ErrTypeNotFound      ErrorType = "NOT_FOUND"
	ErrTypePermission    ErrorType = "PERMISSION"
	ErrTypeInternal      ErrorType = "INTERNAL"
	ErrTypeActor         ErrorType = "ACTOR"
	ErrTypeConfig        ErrorType = "CONFIG"
	ErrTypeRPC           ErrorType = "RPC"
	ErrTypeConnection    ErrorType = "CONNECTION"
	ErrTypeAlreadyExists ErrorType = "ALREADY_EXISTS"
)

// TypedError 是带语义类型和可选调用堆栈的错误。
//
// 实现了以下接口：
//   - error                  — Error() 返回 "[TYPE] message" 或 "[TYPE] message: cause"
//   - [fmt.Formatter]        — %+v 输出完整堆栈，与 pkg/errors 一致
//   - Unwrap() error         — 支持 errors.Is / errors.As 链式查找
//   - Is(target error) bool  — 按 Type 字段匹配，同类型即视为相等
type TypedError struct {
	Type    ErrorType // 错误语义类别
	Message string    // 错误描述
	Err     error     // 被包装的底层错误（可选）
	Stack   []uintptr // 调用堆栈的 PC 值（可选，由 WithStack 系列函数填充）
}

// Error 返回 "[TYPE] message" 格式的错误描述。若包装了底层错误则追加 ": cause"。
func (e *TypedError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Format 实现 fmt.Formatter，与 pkg/errors 习惯一致：
//
//	%s, %v  → 等同于 Error()
//	%+v     → Error() + 完整堆栈（每帧 function\n\tfile:line）
//	%q      → 带引号的 Error()
func (e *TypedError) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			_, _ = fmt.Fprint(f, e.Error())
			e.writeStack(f)
			return
		}
		_, _ = fmt.Fprint(f, e.Error())
	case 's':
		_, _ = fmt.Fprint(f, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(f, "%q", e.Error())
	}
}

func (e *TypedError) writeStack(w fmt.State) {
	if len(e.Stack) == 0 {
		return
	}
	frames := runtime.CallersFrames(e.Stack)
	for {
		frame, more := frames.Next()
		_, _ = fmt.Fprintf(w, "\n%s\n\t%s:%d", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
}

// Unwrap 返回被包装的底层错误，支持 errors.Is / errors.As 链式查找。
func (e *TypedError) Unwrap() error {
	return e.Err
}

// Is 实现基于 ErrorType 的相等判断，供 errors.Is 使用。
//
// 只比较 Type 字段，不比较 Message 或 Err。这意味着相同类型的不同
// TypedError 实例在 errors.Is 看来是匹配的：
//
//	err1 := errs.New(errs.ErrTypeTimeout, "read timeout")
//	err2 := errs.New(errs.ErrTypeTimeout, "write timeout")
//	errors.Is(err1, err2) // true — 同为 TIMEOUT
//
// 常见用法是构造只含 Type 的哨兵值做匹配：
//
//	if errors.Is(err, &errs.TypedError{Type: errs.ErrTypeTimeout}) { … }
//
// 也可以直接使用更简洁的 [IsType]：
//
//	if errs.IsType(err, errs.ErrTypeTimeout) { … }
func (e *TypedError) Is(target error) bool {
	t, ok := target.(*TypedError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// StackTrace 返回人类可读的堆栈字符串，适合写入日志文件。
// 如果未捕获堆栈（Stack 为空），返回空字符串。
//
// 与 [TypedError.Format] 的 %+v 输出格式略有不同：
// StackTrace 带有 "Stack trace:" 前缀和缩进，更适合独立阅读；
// %+v 按 pkg/errors 的 "function\n\tfile:line" 格式紧凑输出。
func (e *TypedError) StackTrace() string {
	if len(e.Stack) == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(e.Stack)
	sb.WriteString("\nStack trace:\n")

	for {
		frame, more := frames.Next()
		_, _ = fmt.Fprintf(&sb, "  %s\n    %s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	return sb.String()
}

// New 创建一个 TypedError（不捕获堆栈）。
// 适合已知类型的业务错误，在热路径上开销最小（~20 ns / 1 alloc）。
func New(typ ErrorType, message string) error {
	return &TypedError{
		Type:    typ,
		Message: message,
	}
}

// Newf 是 [New] 的格式化版本。
func Newf(typ ErrorType, format string, args ...interface{}) error {
	return &TypedError{
		Type:    typ,
		Message: fmt.Sprintf(format, args...),
	}
}

// callers 捕获调用堆栈，skip 是相对于 callers 调用者的额外跳过层数。
// skip=0 表示从 callers 的调用者开始捕获。
func callers(skip int) []uintptr {
	pcs := make([]uintptr, 32)
	// +2: runtime.Callers 自身 + callers 自身
	n := runtime.Callers(skip+2, pcs)
	return pcs[:n]
}

// WithStack 创建一个带调用堆栈的 TypedError。
// 堆栈从 WithStack 的调用者开始捕获。使用 %+v 可打印完整堆栈。
// 约 220 ns / 2 allocs，建议仅在需要诊断信息的路径使用。
func WithStack(typ ErrorType, message string) error {
	return &TypedError{
		Type:    typ,
		Message: message,
		Stack:   callers(1),
	}
}

// WithStackf 是 [WithStack] 的格式化版本。
func WithStackf(typ ErrorType, format string, args ...interface{}) error {
	return &TypedError{
		Type:    typ,
		Message: fmt.Sprintf(format, args...),
		Stack:   callers(1),
	}
}

// WithStackSkip 创建带堆栈的 TypedError，extraSkip 控制额外跳过的帧数。
//
// extraSkip=0 等同于 [WithStack]（堆栈从调用者开始）。
// 适用于在辅助函数内部创建错误、需要跳过辅助函数本身的场景：
//
//	func newDBError(msg string) error {
//	    // extraSkip=1 跳过 newDBError，堆栈从 newDBError 的调用者开始
//	    return errs.WithStackSkip(1, errs.ErrTypeDatabase, msg)
//	}
func WithStackSkip(extraSkip int, typ ErrorType, message string) error {
	return &TypedError{
		Type:    typ,
		Message: message,
		Stack:   callers(1 + extraSkip),
	}
}

// WithStackSkipf 是 [WithStackSkip] 的格式化版本。
func WithStackSkipf(extraSkip int, typ ErrorType, format string, args ...interface{}) error {
	return &TypedError{
		Type:    typ,
		Message: fmt.Sprintf(format, args...),
		Stack:   callers(1 + extraSkip),
	}
}

// Wrap 包装一个已有错误，附加类型和消息（不捕获堆栈）。
// 如果 err 为 nil 则返回 nil，遵循 Go 的 nil error 传递惯例。
func Wrap(err error, typ ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return &TypedError{
		Type:    typ,
		Message: message,
		Err:     err,
	}
}

// WrapFWrapf 是 [Wrap] 的格式化版本。err 为 nil 时返回 nil。
func Wrapf(err error, typ ErrorType, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &TypedError{
		Type:    typ,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}

// WrapWithStack 包装已有错误，同时附加类型、消息和调用堆栈。
// err 为 nil 时返回 nil。
func WrapWithStack(err error, typ ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return &TypedError{
		Type:    typ,
		Message: message,
		Err:     err,
		Stack:   callers(1),
	}
}

// WrapWithStackf 是 [WrapWithStack] 的格式化版本。err 为 nil 时返回 nil。
func WrapWithStackf(err error, typ ErrorType, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &TypedError{
		Type:    typ,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
		Stack:   callers(1),
	}
}

// IsType 沿错误链（Unwrap）查找是否存在指定 ErrorType 的 TypedError。
// 零分配、< 2 ns（单层），适合在热路径上做分支判断。
func IsType(err error, targetType ErrorType) bool {
	for err != nil {
		if te, ok := err.(*TypedError); ok {
			if te.Type == targetType {
				return true
			}
		}
		// 使用标准库的 Unwrap 逐层剥离
		err = errors.Unwrap(err)
	}
	return false
}

// GetType 返回错误链中最外层 TypedError 的类型。
// 如果链中没有 TypedError，返回空字符串 ""。
func GetType(err error) ErrorType {
	for err != nil {
		if te, ok := err.(*TypedError); ok {
			return te.Type
		}
		err = errors.Unwrap(err)
	}
	return ""
}

// GetTypedError 从错误链中提取最外层的 *TypedError。
// 如果链中没有 TypedError，返回 nil。
func GetTypedError(err error) *TypedError {
	for err != nil {
		if te, ok := err.(*TypedError); ok {
			return te
		}
		err = errors.Unwrap(err)
	}
	return nil
}

// IsTimeout 检查错误链中是否存在 TIMEOUT 类型的错误。
func IsTimeout(err error) bool {
	return IsType(err, ErrTypeTimeout)
}

// IsNetwork 检查错误链中是否存在 NETWORK 类型的错误。
func IsNetwork(err error) bool {
	return IsType(err, ErrTypeNetwork)
}

// IsDatabase 检查错误链中是否存在 DATABASE 类型的错误。
func IsDatabase(err error) bool {
	return IsType(err, ErrTypeDatabase)
}

// IsValidation 检查错误链中是否存在 VALIDATION 类型的错误。
func IsValidation(err error) bool {
	return IsType(err, ErrTypeValidation)
}

// IsNotFound 检查错误链中是否存在 NOT_FOUND 类型的错误。
func IsNotFound(err error) bool {
	return IsType(err, ErrTypeNotFound)
}

// IsAlreadyExists 检查错误链中是否存在 ALREADY_EXISTS 类型的错误。
func IsAlreadyExists(err error) bool {
	return IsType(err, ErrTypeAlreadyExists)
}

// Is 是 errors.Is 的便捷转发，避免调用方额外 import "errors"。
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As 是 errors.As 的便捷转发，避免调用方额外 import "errors"。
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}
