//go:build !linux && !darwin

package zreactor

import (
	"context"
	"net"
	"testing"
)

func TestServe_PanicsOnNonLinux(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no tcp listen:", err)
	}
	defer ln.Close()
	ctx := context.Background()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = Serve(ctx, ln, nil, nil)
	}()
	if !panicked {
		t.Error("Serve should panic on non-Linux")
	}
}
