//go:build linux

package ztcp

import (
	"context"
	"net"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zreactor"
	"go.uber.org/zap"
)

// ServerReactor 使用 reactor（epoll）驱动读，减少 goroutine 数量；仅 Linux。
// 与 Server() 二选一：本方法阻塞直到 ctx 取消；不启动每连接读 goroutine。
// 当前不支持 TLS（listener 须为 *net.TCPListener）。
func (ser *Server) ServerReactor(ctx context.Context) {
	listener, err := net.Listen("tcp", ser.GetAddr())
	if err != nil {
		zlog.Error("Failed to start TCP reactor server",
			zap.String("addr", ser.GetAddr()),
			zap.Error(err))
		return
	}
	if ser.GetTLSConfig() != nil && ser.GetTLSConfig().Mode != 0 {
		listener.Close()
		zlog.Error("TCP reactor mode does not support TLS")
		return
	}
	zlog.Info("TCP reactor server started", zap.String("addr", listener.Addr().String()))
	ser.SetListener(listener)
	defer func() {
		ser.OnceDo(func() { close(ser.GetClose()) })
		ser.BaseClose()
	}()

	acceptFn := func(conn net.Conn) (zreactor.ReactorChannel, bool) {
		defer zlog.Recover("TServer reactor accept recover")
		channelId := ser.NextId()
		channel := NewChannel(channelId, conn, ser)
		if !ser.HandleAccept(channel) {
			channel.Close() // 拒绝连接时释放 channel 持有的 readBuffer 等资源
			return nil, false
		}
		ser.AddChannel(channel)
		if !ser.SyncMode() {
			channel.StartSend(ctx)
		}
		return channel, true
	}
	metrics := ser.reactorMetrics
	_ = zreactor.Serve(ctx, listener, acceptFn, metrics)
}
