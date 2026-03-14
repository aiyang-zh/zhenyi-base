//go:build !linux

package ztcp

import "context"

// ServerReactor 仅在 Linux 可用；非 Linux 调用会 panic。
func (ser *Server) ServerReactor(ctx context.Context) {
	panic("ztcp.ServerReactor is only available on Linux (zreactor/epoll)")
}
