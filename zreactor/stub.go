//go:build !linux

package zreactor

import (
	"context"
	"net"
)

// Serve 仅在 Linux 可用；非 Linux 调用会 panic。
func Serve(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics) error {
	panic("zreactor.Serve is only available on Linux")
}

// ServeWithConfig 仅在 Linux 可用；非 Linux 调用会 panic。
func ServeWithConfig(ctx context.Context, listener net.Listener, accept AcceptFunc, metrics *Metrics, config *ServeConfig) error {
	panic("zreactor.ServeWithConfig is only available on Linux")
}
