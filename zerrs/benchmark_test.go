package zerrs

import (
	"errors"
	"fmt"
	"testing"
)

// 包级变量防止编译器优化
var (
	benchErr    error
	benchString string
	benchType   ErrorType
	benchBool   bool
)

// ============================================================================
// 基础错误创建性能测试
// ============================================================================

// BenchmarkNew 测试创建简单错误的性能
func BenchmarkNew(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = New(ErrTypeValidation, "test error")
	}
	benchErr = err
}

// BenchmarkNewf 测试格式化创建错误的性能
func BenchmarkNewf(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = Newf(ErrTypeValidation, "error code: %d, user: %s", 404, "admin")
	}
	benchErr = err
}

// BenchmarkWithStack 测试创建带堆栈错误的性能（应该最慢）
func BenchmarkWithStack(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = WithStack(ErrTypeInternal, "internal error")
	}
	benchErr = err
}

// BenchmarkWithStackf 测试格式化创建带堆栈错误的性能
func BenchmarkWithStackf(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = WithStackf(ErrTypeInternal, "error: %s", "test")
	}
	benchErr = err
}

// BenchmarkWithStackSkip 测试 WithStackSkip（extraSkip=0）的性能
func BenchmarkWithStackSkip(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = WithStackSkip(0, ErrTypeInternal, "skip0")
	}
	benchErr = err
}

// BenchmarkWithStackSkip1 测试 WithStackSkip（extraSkip=1）经 helper 调用的性能
func BenchmarkWithStackSkip1(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = benchHelper()
	}
	benchErr = err
}

func benchHelper() error {
	return WithStackSkip(1, ErrTypeInternal, "from helper")
}

// BenchmarkSentinel 测试直接返回 Sentinel Error 的性能（零分配）
func BenchmarkSentinel(b *testing.B) {
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = ErrInvalidParameter
	}
	benchErr = err
}

// ============================================================================
// 错误包装性能测试
// ============================================================================

// BenchmarkWrap 测试包装错误的性能
func BenchmarkWrap(b *testing.B) {
	baseErr := errors.New("base error")
	var err error
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err = Wrap(baseErr, ErrTypeNetwork, "network error")
	}
	benchErr = err
}

// BenchmarkWrapf 测试格式化包装错误的性能
func BenchmarkWrapf(b *testing.B) {
	baseErr := errors.New("base error")
	var err error
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err = Wrapf(baseErr, ErrTypeNetwork, "network error: %s", "timeout")
	}
	benchErr = err
}

// BenchmarkWrapWithStack 测试带堆栈包装的性能
func BenchmarkWrapWithStack(b *testing.B) {
	baseErr := errors.New("base error")
	var err error
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err = WrapWithStack(baseErr, ErrTypeRPC, "rpc failed")
	}
	benchErr = err
}

// BenchmarkStdlibWrap 对比：标准库 fmt.Errorf 包装性能
func BenchmarkStdlibWrap(b *testing.B) {
	baseErr := errors.New("base error")
	var err error
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err = fmt.Errorf("wrapped: %w", baseErr)
	}
	benchErr = err
}

// ============================================================================
// 错误使用场景性能测试
// ============================================================================

// BenchmarkError 测试 Error() 方法调用性能
func BenchmarkError(b *testing.B) {
	err := New(ErrTypeDatabase, "database error")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = err.Error()
	}
	benchString = s
}

// BenchmarkErrorWrapped 测试包装错误的 Error() 方法性能
func BenchmarkErrorWrapped(b *testing.B) {
	baseErr := errors.New("base error")
	err := Wrap(baseErr, ErrTypeNetwork, "network failure")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = err.Error()
	}
	benchString = s
}

// BenchmarkStackTrace 测试堆栈格式化性能（应该非常慢）
func BenchmarkStackTrace(b *testing.B) {
	err := WithStack(ErrTypeInternal, "internal error")
	te := GetTypedError(err)
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = te.StackTrace()
	}
	benchString = s
}

// ============================================================================
// fmt.Formatter 性能测试
// ============================================================================

// BenchmarkFormatV 测试 %v 格式化（无堆栈，等同 Error()）
func BenchmarkFormatV(b *testing.B) {
	err := WithStack(ErrTypeInternal, "boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%v", err)
	}
	benchString = s
}

// BenchmarkFormatPlusV 测试 %+v 格式化（含堆栈展开，与 pkg/errors 一致）
func BenchmarkFormatPlusV(b *testing.B) {
	err := WithStack(ErrTypeInternal, "boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%+v", err)
	}
	benchString = s
}

// BenchmarkFormatS 测试 %s 格式化
func BenchmarkFormatS(b *testing.B) {
	err := WithStack(ErrTypeInternal, "boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%s", err)
	}
	benchString = s
}

// BenchmarkFormatQ 测试 %q 格式化
func BenchmarkFormatQ(b *testing.B) {
	err := WithStack(ErrTypeInternal, "boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%q", err)
	}
	benchString = s
}

// BenchmarkFormatPlusV_NoStack 测试 %+v 但无堆栈（退化为 %v）
func BenchmarkFormatPlusV_NoStack(b *testing.B) {
	err := New(ErrTypeInternal, "boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%+v", err)
	}
	benchString = s
}

// BenchmarkStdlibFormatPlusV 对比：标准库 %+v 格式化
func BenchmarkStdlibFormatPlusV(b *testing.B) {
	err := errors.New("boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%+v", err)
	}
	benchString = s
}

// BenchmarkStdlibFormatV 对比：标准库 %v 格式化
func BenchmarkStdlibFormatV(b *testing.B) {
	err := errors.New("boom")
	var s string
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s = fmt.Sprintf("%v", err)
	}
	benchString = s
}

// ============================================================================
// 类型检查性能测试
// ============================================================================

// BenchmarkIsType 测试单层错误类型检查
func BenchmarkIsType(b *testing.B) {
	err := New(ErrTypeTimeout, "timeout")
	var result bool
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result = IsType(err, ErrTypeTimeout)
	}
	benchBool = result
}

// BenchmarkIsTypeMultiLayer 测试多层包装的类型检查
func BenchmarkIsTypeMultiLayer(b *testing.B) {
	baseErr := errors.New("base")
	err1 := Wrap(baseErr, ErrTypeDatabase, "db")
	err2 := Wrap(err1, ErrTypeNetwork, "network")
	err3 := Wrap(err2, ErrTypeInternal, "internal")

	var result bool
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result = IsType(err3, ErrTypeDatabase)
	}
	benchBool = result
}

// BenchmarkGetType 测试获取错误类型
func BenchmarkGetType(b *testing.B) {
	err := New(ErrTypeActor, "actor error")
	var typ ErrorType
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		typ = GetType(err)
	}
	benchType = typ
}

// BenchmarkIsTimeout 测试便捷类型检查函数
func BenchmarkIsTimeout(b *testing.B) {
	err := New(ErrTypeTimeout, "timeout")
	var result bool
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result = IsTimeout(err)
	}
	benchBool = result
}

// ============================================================================
// 标准库兼容性性能测试
// ============================================================================

// BenchmarkStdlibIs 测试 errors.Is 性能
func BenchmarkStdlibIs(b *testing.B) {
	err := ErrTimeout
	target := ErrTimeout
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errors.Is(err, target)
	}
}

// BenchmarkStdlibIsWrapped 测试包装后的 errors.Is 性能
func BenchmarkStdlibIsWrapped(b *testing.B) {
	err := fmt.Errorf("wrapped: %w", ErrTimeout)
	target := ErrTimeout
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errors.Is(err, target)
	}
}

// BenchmarkStdlibAs 测试 errors.As 性能
func BenchmarkStdlibAs(b *testing.B) {
	err := New(ErrTypeDatabase, "db error")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var te *TypedError
		_ = errors.As(err, &te)
	}
}

// ============================================================================
// 便捷函数性能测试
// ============================================================================

// BenchmarkInvalidParameterf 测试便捷函数性能
func BenchmarkInvalidParameterf(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = InvalidParameterf("field %s is required", "username")
	}
}

// BenchmarkTimeoutf测试超时便捷函数性能
func BenchmarkTimeoutf(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Timeoutf("operation timeout after %ds", 30)
	}
}

// BenchmarkInternalf 测试带堆栈的内部错误便捷函数性能
func BenchmarkInternalf(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Internalf("internal error: %s", "test")
	}
}

// ============================================================================
// 实际场景模拟
// ============================================================================

// BenchmarkScenarioValidation 模拟请求验证场景（热路径）
func BenchmarkScenarioValidation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := validateRequest(i)
		if err != nil {
			_ = err.Error()
		}
	}
}

func validateRequest(userID int) error {
	if userID == 0 {
		return ErrInvalidParameter
	}
	if userID < 0 {
		return InvalidParameterf("user_id must be positive")
	}
	return nil
}

// BenchmarkScenarioDatabaseError 模拟数据库错误场景
func BenchmarkScenarioDatabaseError(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := queryDatabase(i)
		if err != nil {
			_ = err.Error()
		}
	}
}

func queryDatabase(id int) error {
	baseErr := errors.New("connection timeout")
	return Wrap(baseErr, ErrTypeDatabase, "query user failed")
}

// BenchmarkScenarioPanicRecovery 模拟 panic 恢复场景（带堆栈）
func BenchmarkScenarioPanicRecovery(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := safeExecute(func() {
			// 正常执行，不 panic
		})
		if err != nil {
			_ = err.Error()
		}
	}
}

func safeExecute(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = WithStackf(ErrTypeInternal, "panic: %v", r)
		}
	}()
	fn()
	return nil
}

// ============================================================================
// 对比测试：与其他实现对比
// ============================================================================

// BenchmarkStdlibError 对比：标准库 errors.New
func BenchmarkStdlibError(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errors.New("test error")
	}
}

// BenchmarkStdlibErrorf 对比：标准库 fmt.Errorf
func BenchmarkStdlibErrorf(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fmt.Errorf("error code: %d", 404)
	}
}
