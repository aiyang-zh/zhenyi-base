package zsafe

import (
	"testing"
)

// ============================================================
// 单元测试
// ============================================================

func TestRecover_CatchesPanic(t *testing.T) {
	// 验证 Recover 能捕获 panic，不会向上传播
	didPanic := true
	func() {
		defer Recover("test_catch")
		panic("test panic value")
	}()
	didPanic = false

	if didPanic {
		t.Error("Recover should have caught the panic")
	}
}

func TestRecover_NoPanic(t *testing.T) {
	// 正常执行不触发 panic 时，Recover 应安静通过
	func() {
		defer Recover("test_no_panic")
		// 不触发 panic
	}()
	// 能走到这里说明没有意外行为
}

func TestRecover_StringPanic(t *testing.T) {
	func() {
		defer Recover("string_panic")
		panic("string error")
	}()
}

func TestRecover_ErrorPanic(t *testing.T) {
	func() {
		defer Recover("error_panic")
		panic(42) // 非 string 类型的 panic
	}()
}

func TestRecover_NilPanic(t *testing.T) {
	// panic(nil) 在 Go 1.21+ 会被 recover 捕获为 *runtime.PanicNilError
	func() {
		defer Recover("nil_panic")
		panic(nil)
	}()
}

func TestRecover_NestedPanic(t *testing.T) {
	// 嵌套调用中的 panic 应被最近的 Recover 捕获
	func() {
		defer Recover("outer")
		func() {
			defer Recover("inner")
			panic("inner panic")
		}()
		// 内层 panic 被 inner Recover 捕获后，外层正常执行
	}()
}

func TestRecover_SubsequentCodeRuns(t *testing.T) {
	// 验证 panic 被捕获后，defer Recover 之后的代码不执行（defer 语义）
	// 但调用方可以继续执行
	executed := false
	func() {
		defer Recover("test_subsequent")
		panic("stop here")
		// 这行不会执行（编译器不会报 unreachable，但运行时跳过）
	}()
	executed = true

	if !executed {
		t.Error("code after recovered function should execute")
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkRecover_NoPanic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer Recover("bench_no_panic")
		}()
	}
}

func BenchmarkRecover_WithPanic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		func() {
			defer Recover("bench_panic")
			panic("bench")
		}()
	}
}
