package zerrs

import (
	"io"
	"testing"
)

// FuzzErrsConstructors 对 New / Newf / Wrap / Wrapf 做模糊测试，要求不 panic。
func FuzzErrsConstructors(f *testing.F) {
	f.Add("x", int32(1))
	f.Add("", int32(0))

	f.Fuzz(func(t *testing.T, msg string, code int32) {
		if len(msg) > 4096 {
			msg = msg[:4096]
		}
		types := []ErrorType{
			ErrTypeInternal, ErrTypeTimeout, ErrTypeValidation, ErrTypeNetwork,
		}
		typ := types[int(code)%len(types)]

		_ = New(typ, msg)
		_ = Newf(typ, "f %d %s", code, msg)
		_ = Wrap(io.EOF, typ, msg)
		_ = Wrapf(io.EOF, typ, "w %d", code)
	})
}
