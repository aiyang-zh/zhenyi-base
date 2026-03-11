package ztcp

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

// Server 为 TCP 协议的服务端实现，嵌入 BaseServer 并完成 Listen/Accept 与 TLS 包装。
type Server struct {
	*znet.BaseServer
}

// NewServer 创建 TCP 服务端；addr 为监听地址，handlers 必须提供 OnAccept 与 OnRead。
func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

// start 在 goroutine 中执行：绑定端口、可选 TLS、然后进入 listen 循环。
func (ser *Server) start(ctx context.Context) {
	listener, err := net.Listen("tcp", ser.GetAddr())
	if err != nil {
		zlog.Error("Failed to start TCP server",
			zap.String("addr", ser.GetAddr()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to listen")))
		return
	}

	// 如果配置了 TLS/GM-TLS，包装 listener
	if ser.GetTLSConfig() != nil && ser.GetTLSConfig().Mode != ziface.TLSModeNone {
		listener = ser.GetTLSConfig().WrapListener(listener)
		zlog.Info("TCP server started with TLS",
			zap.String("addr", ser.GetAddr()),
			zap.Int("tlsMode", int(ser.GetTLSConfig().Mode)))
	} else {
		zlog.Info("TCP server started successfully", zap.String("addr", ser.GetAddr()))
	}

	ser.SetListener(listener)
	ser.listen(ctx)
}

func (ser *Server) listen(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
		case <-ser.GetClose():
		}
		ser.Close()
	}()
	for {
		// 等待客户端连接
		conn, err1 := ser.GetListener().Accept()
		if err1 != nil {
			var opError *net.OpError
			if zerrs.As(err1, &opError) {
				zlog.Info("TCP server listener closed")
				return
			}
			zlog.Warn("Failed to accept connection",
				zap.Error(zerrs.Wrap(err1, zerrs.ErrTypeNetwork, "accept failed")))
			continue
		}

		// ✅ Accept 后立即启动 goroutine，避免阻塞 Accept 循环
		go func(c net.Conn) {
			defer zlog.Recover("TServer handler recover")

			channelId := ser.NextId()
			channel := NewChannel(channelId, c, ser)

			if !ser.HandleAccept(channel) {
				err := c.Close()
				if err != nil {
					zlog.Warn("Failed to close connection", zap.Error(err))
				}
				zlog.Warn("TServer OnAccept rejected", zap.Uint64("channelId", channelId))
				return
			}
			ser.AddChannel(channel)
			defer channel.Close() // ✅ 自动清理（从 map 中移除 + 触发回调）
			ser.RunChannel(ctx, channel)
		}(conn)
	}
}

// Server 实现 ziface.IServer：启动监听并阻塞直至 ctx 取消或 Close。
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zlog.Recover("TServer Server recover")
		ser.start(ctx)
	}()
}

// Close 关闭 TCP 服务（关闭 listener 与所有连接）。
func (ser *Server) Close() {
	ser.OnceDo(func() {
		close(ser.GetClose())
	})
	ser.BaseClose() // ✅ 修复：清理 listener 和所有 channels
	zlog.Info("TCP server closed successfully", zap.String("addr", ser.GetAddr()))
}
