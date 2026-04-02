//go:build darwin

package ztcp

import (
	"context"
	"net"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zreactor"
	"go.uber.org/zap"
)

// ServerReactor 使用 reactor（kqueue）驱动 TCP 读，减少 goroutine 数；仅 darwin。
// 与 Server() 二选一：本方法阻塞直到 ctx 取消；不支持 TLS（listener 需为 *net.TCPListener）。
func (ser *Server) ServerReactor(ctx context.Context) {
	listener, err := net.Listen("tcp", ser.GetAddr())
	if err != nil {
		zlog.Error("Failed to start TCP reactor server",
			zap.String("addr", ser.GetAddr()),
			zap.Error(err))
		return
	}
	if ser.GetTLSConfig() != nil && ser.GetTLSConfig().Mode != 0 {
		_ = listener.Close()
		zlog.Error("TCP reactor mode does not support TLS")
		return
	}

	zlog.Info("TCP reactor server started", zap.String("addr", listener.Addr().String()))
	ser.SetListener(listener)
	ser.SetSharedSendWorkerMode(true)
	defer func() {
		ser.OnceDo(func() { close(ser.GetClose()) })
		ser.BaseClose()
	}()

	acceptFn := func(conn net.Conn) (zreactor.ReactorChannel, bool) {
		defer zlog.Recover("TServer reactor accept recover")
		channelId := ser.NextId()
		channel := NewChannel(channelId, conn, ser)
		if !ser.HandleAccept(channel) {
			_ = conn.Close()
			return nil, false
		}
		ser.AddChannel(channel)
		if !ser.SyncMode() {
			_ = ser.BindSharedSendHook(ctx, channel)
		}
		return channel, true
	}

	metrics := ser.reactorMetrics
	// BatchRead 在高连接数 + 高频小包场景下能显著降低 fd 切换与函数调度开销。
	_ = zreactor.ServeWithConfig(ctx, listener, acceptFn, metrics, &zreactor.ServeConfig{
		MinEvents: 1024,
		BatchRead: true,
	})
}
