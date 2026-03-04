package zerrs

import (
	"errors"
	"fmt"
	"strings"
)

func ExampleNew() {
	err := New(ErrTypeTimeout, "operation timeout")
	fmt.Println(err)
	// Output:
	// [TIMEOUT] operation timeout
}

func ExampleNewf() {
	err := Newf(ErrTypeNetwork, "port %d refused", 8080)
	fmt.Println(err)
	// Output:
	// [NETWORK] port 8080 refused
}

func ExampleWithStack() {
	err := WithStack(ErrTypeInternal, "unexpected nil pointer")

	// %v 只输出消息，不含堆栈
	fmt.Println(err)

	// %+v 输出消息 + 完整堆栈（此处只验证堆栈存在）
	full := fmt.Sprintf("%+v", err)
	fmt.Println(strings.Contains(full, "ExampleWithStack"))
	// Output:
	// [INTERNAL] unexpected nil pointer
	// true
}

func ExampleWrap() {
	ioErr := errors.New("connection reset by peer")
	err := Wrap(ioErr, ErrTypeNetwork, "read failed")
	fmt.Println(err)

	// 通过 Unwrap 可以拿到原始错误
	fmt.Println(errors.Unwrap(err))
	// Output:
	// [NETWORK] read failed: connection reset by peer
	// connection reset by peer
}

func ExampleWrapWithStack() {
	dbErr := errors.New("duplicate key")
	err := WrapWithStack(dbErr, ErrTypeDatabase, "insert user")
	fmt.Println(err)

	te := GetTypedError(err)
	fmt.Println(len(te.Stack) > 0)
	// Output:
	// [DATABASE] insert user: duplicate key
	// true
}

// ExampleWithStackSkip 展示在辅助函数中使用 WithStackSkip 跳过自身帧。
func ExampleWithStackSkip() {
	err := helper()
	full := fmt.Sprintf("%+v", err)

	// 堆栈中不包含 helper 函数本身
	fmt.Println(strings.Contains(full, "ExampleWithStackSkip"))
	// Output:
	// true
}

func helper() error {
	return WithStackSkip(1, ErrTypeInternal, "from helper")
}

func ExampleIsType() {
	inner := New(ErrTypeTimeout, "query timeout")
	outer := Wrap(inner, ErrTypeDatabase, "load user")

	fmt.Println(IsType(outer, ErrTypeDatabase))
	fmt.Println(IsType(outer, ErrTypeTimeout))
	fmt.Println(IsType(outer, ErrTypeNetwork))
	// Output:
	// true
	// true
	// false
}

func ExampleGetType() {
	err := Wrap(
		New(ErrTypeTimeout, "inner"),
		ErrTypeDatabase, "outer",
	)
	fmt.Println(GetType(err))
	// Output:
	// DATABASE
}

func ExampleIsTimeout() {
	err := Wrap(ErrReadTimeout, ErrTypeNetwork, "recv")
	fmt.Println(IsTimeout(err))
	// Output:
	// true
}

func ExampleTypedError_Format() {
	err := New(ErrTypeNetwork, "reset")

	fmt.Printf("%%v:  %v\n", err)
	fmt.Printf("%%s:  %s\n", err)
	fmt.Printf("%%q:  %q\n", err)
	// Output:
	// %v:  [NETWORK] reset
	// %s:  [NETWORK] reset
	// %q:  "[NETWORK] reset"
}

func ExampleTypedError_Is() {
	err1 := New(ErrTypeTimeout, "read timeout")
	err2 := New(ErrTypeTimeout, "write timeout")
	err3 := New(ErrTypeNetwork, "refused")

	// 同类型匹配
	fmt.Println(errors.Is(err1, err2))

	// 不同类型不匹配
	fmt.Println(errors.Is(err1, err3))

	// 用哨兵值匹配
	sentinel := &TypedError{Type: ErrTypeTimeout}
	fmt.Println(errors.Is(err1, sentinel))
	// Output:
	// true
	// false
	// true
}

func ExampleInternalf() {
	err := Internalf("nil pointer in %s", "UserService.Get")
	fmt.Println(err)

	te := GetTypedError(err)
	fmt.Println(len(te.Stack) > 0)
	// Output:
	// [INTERNAL] nil pointer in UserService.Get
	// true
}
