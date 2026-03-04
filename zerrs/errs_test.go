package zerrs

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// New / NewF
// ============================================================================

func TestNew(t *testing.T) {
	err := New(ErrTypeNetwork, "conn reset")
	te := err.(*TypedError)
	if te.Type != ErrTypeNetwork {
		t.Fatalf("type = %q, want %q", te.Type, ErrTypeNetwork)
	}
	if te.Message != "conn reset" {
		t.Fatalf("message = %q, want %q", te.Message, "conn reset")
	}
	if te.Err != nil {
		t.Fatal("Err should be nil")
	}
	if len(te.Stack) != 0 {
		t.Fatal("Stack should be empty for New")
	}
}

func TestNewf(t *testing.T) {
	err := Newf(ErrTypeTimeout, "waited %dms", 500)
	want := "[TIMEOUT] waited 500ms"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

// ============================================================================
// WithStack / WithStackf / WithStackSkip
// ============================================================================

func TestWithStack_HasStack(t *testing.T) {
	err := WithStack(ErrTypeInternal, "boom")
	te := err.(*TypedError)
	if len(te.Stack) == 0 {
		t.Fatal("WithStack should capture stack")
	}
}

func TestWithStackf(t *testing.T) {
	err := WithStackf(ErrTypeDatabase, "table %s missing", "users")
	te := err.(*TypedError)
	if te.Message != "table users missing" {
		t.Fatalf("message = %q", te.Message)
	}
	if len(te.Stack) == 0 {
		t.Fatal("WithStackf should capture stack")
	}
}

func helperCreateError() error {
	return WithStackSkip(1, ErrTypeInternal, "from helper")
}

func TestWithStackSkip_SkipsHelper(t *testing.T) {
	err := helperCreateError()
	te := err.(*TypedError)
	out := fmt.Sprintf("%+v", te)
	if strings.Contains(out, "helperCreateError") {
		t.Fatalf("stack should NOT contain helperCreateError, got:\n%s", out)
	}
	if !strings.Contains(out, "TestWithStackSkip_SkipsHelper") {
		t.Fatalf("stack should contain TestWithStackSkip_SkipsHelper, got:\n%s", out)
	}
}

func TestWithStackSkip_Zero_SameAsWithStack(t *testing.T) {
	err := WithStackSkip(0, ErrTypeInternal, "skip0")
	te := err.(*TypedError)
	out := fmt.Sprintf("%+v", te)
	if !strings.Contains(out, "TestWithStackSkip_Zero_SameAsWithStack") {
		t.Fatalf("extraSkip=0 should capture from caller, got:\n%s", out)
	}
}

func TestWithStackSkipf(t *testing.T) {
	err := WithStackSkipf(0, ErrTypeRPC, "code=%d", 42)
	te := err.(*TypedError)
	if te.Message != "code=42" {
		t.Fatalf("message = %q", te.Message)
	}
	if len(te.Stack) == 0 {
		t.Fatal("should have stack")
	}
}

// ============================================================================
// Wrap / Wrapf/ WrapWithStack / WrapWithStackf
// ============================================================================

func TestWrap(t *testing.T) {
	inner := errors.New("raw io error")
	err := Wrap(inner, ErrTypeNetwork, "read failed")
	te := err.(*TypedError)
	if te.Err != inner {
		t.Fatal("Err should be inner")
	}
	want := "[NETWORK] read failed: raw io error"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestWrap_NilReturnsNil(t *testing.T) {
	if Wrap(nil, ErrTypeNetwork, "x") != nil {
		t.Fatal("Wrap(nil, ...) should return nil")
	}
}

func TestWrapf_NilReturnsNil(t *testing.T) {
	if Wrapf(nil, ErrTypeNetwork, "x %d", 1) != nil {
		t.Fatal("Wrapf(nil, ...) should return nil")
	}
}

func TestWrapWithStack(t *testing.T) {
	inner := errors.New("disk full")
	err := WrapWithStack(inner, ErrTypeDatabase, "write failed")
	te := err.(*TypedError)
	if len(te.Stack) == 0 {
		t.Fatal("WrapWithStack should capture stack")
	}
	if te.Err != inner {
		t.Fatal("Err should be inner")
	}
}

func TestWrapWithStack_NilReturnsNil(t *testing.T) {
	if WrapWithStack(nil, ErrTypeDatabase, "x") != nil {
		t.Fatal("WrapWithStack(nil, ...) should return nil")
	}
}

func TestWrapWithStackf(t *testing.T) {
	inner := errors.New("ENOENT")
	err := WrapWithStackf(inner, ErrTypeNotFound, "file %s", "data.bin")
	te := err.(*TypedError)
	if te.Message != "file data.bin" {
		t.Fatalf("message = %q", te.Message)
	}
	if len(te.Stack) == 0 {
		t.Fatal("should have stack")
	}
}

func TestWrapWithStackf_NilReturnsNil(t *testing.T) {
	if WrapWithStackf(nil, ErrTypeNotFound, "x %d", 1) != nil {
		t.Fatal("WrapWithStackf(nil, ...) should return nil")
	}
}

// ============================================================================
// Unwrap / Is / As / errors chain
// ============================================================================

func TestUnwrap(t *testing.T) {
	inner := errors.New("base")
	err := Wrap(inner, ErrTypeInternal, "layer")
	if errors.Unwrap(err) != inner {
		t.Fatal("Unwrap should return inner")
	}
}

func TestUnwrap_NoInner(t *testing.T) {
	err := New(ErrTypeInternal, "standalone")
	if errors.Unwrap(err) != nil {
		t.Fatal("Unwrap should be nil when no inner error")
	}
}

func TestIs_SameType(t *testing.T) {
	err1 := New(ErrTypeTimeout, "op1")
	err2 := New(ErrTypeTimeout, "op2")
	if !errors.Is(err1, err2) {
		t.Fatal("same type should match via errors.Is")
	}
}

func TestIs_DifferentType(t *testing.T) {
	err1 := New(ErrTypeTimeout, "op1")
	err2 := New(ErrTypeNetwork, "op2")
	if errors.Is(err1, err2) {
		t.Fatal("different types should not match")
	}
}

func TestIs_NonTypedError(t *testing.T) {
	err := New(ErrTypeTimeout, "op")
	if errors.Is(err, errors.New("other")) {
		t.Fatal("TypedError.Is should return false for non-TypedError target")
	}
}

func TestAs(t *testing.T) {
	inner := New(ErrTypeNetwork, "inner")
	wrapped := fmt.Errorf("outer: %w", inner)
	var te *TypedError
	if !errors.As(wrapped, &te) {
		t.Fatal("errors.As should find TypedError in chain")
	}
	if te.Type != ErrTypeNetwork {
		t.Fatalf("type = %q, want NETWORK", te.Type)
	}
}

func TestIs_WrappedChain(t *testing.T) {
	base := New(ErrTypeTimeout, "inner timeout")
	wrapped := Wrap(base, ErrTypeInternal, "handler failed")
	sentinel := &TypedError{Type: ErrTypeInternal}
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("outer type should match")
	}
	sentinel2 := &TypedError{Type: ErrTypeTimeout}
	if !errors.Is(wrapped, sentinel2) {
		t.Fatal("inner type should be reachable via errors.Is chain")
	}
}

// ============================================================================
// IsType / GetType / GetTypedError
// ============================================================================

func TestIsType(t *testing.T) {
	err := Wrap(New(ErrTypeTimeout, "inner"), ErrTypeInternal, "outer")
	if !IsType(err, ErrTypeTimeout) {
		t.Fatal("should find inner timeout type")
	}
	if !IsType(err, ErrTypeInternal) {
		t.Fatal("should find outer internal type")
	}
	if IsType(err, ErrTypeNetwork) {
		t.Fatal("should not find network type")
	}
}

func TestIsType_NilError(t *testing.T) {
	if IsType(nil, ErrTypeTimeout) {
		t.Fatal("nil error should return false")
	}
}

func TestGetType(t *testing.T) {
	err := Wrap(New(ErrTypeTimeout, "base"), ErrTypeInternal, "top")
	if got := GetType(err); got != ErrTypeInternal {
		t.Fatalf("GetType = %q, want INTERNAL (outermost)", got)
	}
}

func TestGetType_NilError(t *testing.T) {
	if got := GetType(nil); got != "" {
		t.Fatalf("GetType(nil) = %q, want empty", got)
	}
}

func TestGetType_NonTypedError(t *testing.T) {
	if got := GetType(errors.New("plain")); got != "" {
		t.Fatalf("GetType(plain error) = %q, want empty", got)
	}
}

func TestGetTypedError(t *testing.T) {
	err := Wrap(New(ErrTypeTimeout, "base"), ErrTypeInternal, "top")
	te := GetTypedError(err)
	if te == nil {
		t.Fatal("should not be nil")
	}
	if te.Type != ErrTypeInternal {
		t.Fatalf("type = %q, want INTERNAL", te.Type)
	}
}

func TestGetTypedError_Nil(t *testing.T) {
	if GetTypedError(nil) != nil {
		t.Fatal("should be nil")
	}
}

// ============================================================================
// Convenience type-check functions
// ============================================================================

func TestConvenienceChecks(t *testing.T) {
	tests := []struct {
		name string
		err  error
		fn   func(error) bool
		want bool
	}{
		{"IsTimeout/true", New(ErrTypeTimeout, "t"), IsTimeout, true},
		{"IsTimeout/false", New(ErrTypeNetwork, "n"), IsTimeout, false},
		{"IsNetwork/true", New(ErrTypeNetwork, "n"), IsNetwork, true},
		{"IsDatabase/true", New(ErrTypeDatabase, "d"), IsDatabase, true},
		{"IsValidation/true", New(ErrTypeValidation, "v"), IsValidation, true},
		{"IsNotFound/true", New(ErrTypeNotFound, "n"), IsNotFound, true},
		{"IsAlreadyExists/true", New(ErrTypeAlreadyExists, "a"), IsAlreadyExists, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.err); got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Sentinel errors (common.go)
// ============================================================================

func TestSentinelErrors_IsType(t *testing.T) {
	if !IsTimeout(ErrTimeout) {
		t.Fatal("ErrTimeout should be timeout")
	}
	if !IsTimeout(ErrReadTimeout) {
		t.Fatal("ErrReadTimeout should be timeout")
	}
	if !IsNotFound(ErrNotFound) {
		t.Fatal("ErrNotFound should be not_found")
	}
	if !IsType(ErrActorNotFound, ErrTypeActor) {
		t.Fatal("ErrActorNotFound should be ACTOR type")
	}
}

func TestSentinelErrors_ErrorsIs(t *testing.T) {
	if !errors.Is(ErrTimeout, ErrConnectionTimeout) {
		t.Fatal("ErrTimeout and ErrConnectionTimeout share TIMEOUT type, should match")
	}
	if errors.Is(ErrTimeout, ErrNotFound) {
		t.Fatal("different types should not match")
	}
}

// ============================================================================
// Convenience creation functions (common.go)
// ============================================================================

func TestConvenienceCreators(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantTyp ErrorType
		substr  string
	}{
		{"InvalidParameterf", InvalidParameterf("id=%d", 0), ErrTypeValidation, "invalid parameter: id=0"},
		{"Timeoutf", Timeoutf("after %ds", 5), ErrTypeTimeout, "after 5s"},
		{"NotFoundf", NotFoundf("user %s", "bob"), ErrTypeNotFound, "user bob"},
		{"AlreadyExistsf", AlreadyExistsf("key %s", "x"), ErrTypeAlreadyExists, "key x"},
		{"Networkf", Networkf("port %d", 80), ErrTypeNetwork, "port 80"},
		{"Databasef", Databasef("table %s", "t"), ErrTypeDatabase, "table t"},
		{"Actorf", Actorf("actor %s", "a1"), ErrTypeActor, "actor a1"},
		{"RPCf", RPCf("method %s", "Ping"), ErrTypeRPC, "method Ping"},
		{"Configf", Configf("key %s", "k"), ErrTypeConfig, "key k"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := tt.err.(*TypedError)
			if te.Type != tt.wantTyp {
				t.Fatalf("type = %q, want %q", te.Type, tt.wantTyp)
			}
			if !strings.Contains(tt.err.Error(), tt.substr) {
				t.Fatalf("Error() = %q, want substring %q", tt.err.Error(), tt.substr)
			}
		})
	}
}

func TestInternalf_HasStack(t *testing.T) {
	err := Internalf("panic: %s", "nil ptr")
	te := err.(*TypedError)
	if len(te.Stack) == 0 {
		t.Fatal("Internalf should capture stack")
	}
}

// ============================================================================
// fmt.Formatter (%v, %+v, %s, %q)
// ============================================================================

func TestFormat_V(t *testing.T) {
	err := New(ErrTypeNetwork, "reset")
	got := fmt.Sprintf("%v", err)
	want := "[NETWORK] reset"
	if got != want {
		t.Fatalf("%%v = %q, want %q", got, want)
	}
}

func TestFormat_S(t *testing.T) {
	err := New(ErrTypeNetwork, "reset")
	got := fmt.Sprintf("%s", err)
	want := "[NETWORK] reset"
	if got != want {
		t.Fatalf("%%s = %q, want %q", got, want)
	}
}

func TestFormat_Q(t *testing.T) {
	err := New(ErrTypeNetwork, "reset")
	got := fmt.Sprintf("%q", err)
	want := `"[NETWORK] reset"`
	if got != want {
		t.Fatalf("%%q = %q, want %q", got, want)
	}
}

func TestFormat_PlusV_NoStack(t *testing.T) {
	err := New(ErrTypeNetwork, "reset")
	got := fmt.Sprintf("%+v", err)
	want := "[NETWORK] reset"
	if got != want {
		t.Fatalf("%%+v without stack = %q, want %q", got, want)
	}
}

func TestFormat_PlusV_WithStack(t *testing.T) {
	err := WithStack(ErrTypeInternal, "boom")
	got := fmt.Sprintf("%+v", err)

	if !strings.HasPrefix(got, "[INTERNAL] boom") {
		t.Fatalf("should start with error message, got:\n%s", got)
	}
	if !strings.Contains(got, "TestFormat_PlusV_WithStack") {
		t.Fatalf("%%+v should contain caller function name, got:\n%s", got)
	}
	if !strings.Contains(got, "errs_test.go:") {
		t.Fatalf("%%+v should contain file:line, got:\n%s", got)
	}
}

func TestFormat_PlusV_WithWrap(t *testing.T) {
	inner := errors.New("io: broken pipe")
	err := WrapWithStack(inner, ErrTypeNetwork, "send failed")
	got := fmt.Sprintf("%+v", err)
	if !strings.Contains(got, "io: broken pipe") {
		t.Fatalf("should contain inner error message, got:\n%s", got)
	}
	if !strings.Contains(got, "TestFormat_PlusV_WithWrap") {
		t.Fatalf("should contain caller, got:\n%s", got)
	}
}

// ============================================================================
// StackTrace()
// ============================================================================

func TestStackTrace_Empty(t *testing.T) {
	err := New(ErrTypeInternal, "no stack")
	te := err.(*TypedError)
	if s := te.StackTrace(); s != "" {
		t.Fatalf("StackTrace should be empty, got %q", s)
	}
}

func TestStackTrace_NonEmpty(t *testing.T) {
	err := WithStack(ErrTypeInternal, "has stack")
	te := err.(*TypedError)
	s := te.StackTrace()
	if !strings.Contains(s, "Stack trace:") {
		t.Fatalf("should contain header, got:\n%s", s)
	}
	if !strings.Contains(s, "TestStackTrace_NonEmpty") {
		t.Fatalf("should contain caller, got:\n%s", s)
	}
}

// ============================================================================
// Edge cases
// ============================================================================

func TestError_WithInnerError(t *testing.T) {
	inner := errors.New("EOF")
	err := Wrap(inner, ErrTypeNetwork, "read")
	want := "[NETWORK] read: EOF"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestError_WithoutInnerError(t *testing.T) {
	err := New(ErrTypeNetwork, "reset")
	want := "[NETWORK] reset"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestIs_PackageLevelFunctions(t *testing.T) {
	err1 := New(ErrTypeTimeout, "t1")
	err2 := New(ErrTypeTimeout, "t2")
	if !Is(err1, err2) {
		t.Fatal("package Is should delegate to errors.Is")
	}
}

func TestAs_PackageLevelFunction(t *testing.T) {
	err := New(ErrTypeTimeout, "t")
	var te *TypedError
	if !As(err, &te) {
		t.Fatal("package As should delegate to errors.As")
	}
}
