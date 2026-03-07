package ztcp

import (
	"context"
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/zlog"
	"github.com/aiyang-zh/zhenyi-core/znet"
	"net"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-util/zerrs"
	"github.com/aiyang-zh/zhenyi-util/zsafe"
)

/*tcp连接管理器*/

type Server struct {
	*znet.BaseServer
}

func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

// 监听
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
			defer zsafe.Recover("TServer handler recover")

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
			channel.StartSend(ctx)
			channel.Start()
		}(conn)
	}
}

// Server 开启服务
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zsafe.Recover("TServer Server recover")
		ser.start(ctx)
	}()
}

func (ser *Server) Close() {
	ser.OnceDo(func() {
		close(ser.GetClose())
	})
	ser.BaseClose() // ✅ 修复：清理 listener 和所有 channels
	zlog.Info("TCP server closed successfully", zap.String("addr", ser.GetAddr()))
}
