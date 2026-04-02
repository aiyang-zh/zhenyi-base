//go:build !linux && !darwin

package ztcp

import "context"

// ServerReactor 仅在 Linux 与 macOS 可用；其它 GOOS 调用会 panic。
func (ser *Server) ServerReactor(ctx context.Context) {
	panic("ztcp.ServerReactor is only available on Linux (epoll) and macOS (kqueue)")
}
