//go:build !linux && !darwin

package zreactor

import (
	"context"
	"net"
)

// Serve 仅在 Linux 与 macOS（darwin）可用；其它平台调用会 panic。
func Serve(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics) error {
	panic("zreactor.Serve is only available on Linux and macOS (darwin)")
}

// ServeWithConfig 仅在 Linux 与 macOS（darwin）可用；其它平台调用会 panic。
func ServeWithConfig(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics, config *ServeConfig) error {
	panic("zreactor.ServeWithConfig is only available on Linux and macOS (darwin)")
}
