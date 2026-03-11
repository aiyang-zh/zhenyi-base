package zkcp

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/xtaci/kcp-go/v5"
	"net"

	"go.uber.org/zap"
)

// Server 为 KCP 协议的服务端实现，嵌入 BaseServer 并完成 KCP Listen/Accept。
type Server struct {
	*znet.BaseServer
	kcpListener *kcp.Listener
}

// NewServer 创建 KCP 服务端；addr 为监听地址，handlers 必须提供 OnAccept 与 OnRead。
func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

// start 启动 KCP 监听并进入 accept 循环（FEC 已关闭，与 Client 一致）。
func (ser *Server) start(ctx context.Context) {
	// 启动kcp连接
	zlog.Info("Starting KCP server", zap.String("addr", ser.GetAddr()))
	// dataShards=0, parityShards=0 关闭 FEC，与 Client 一致。localhost 无丢包时 FEC 纯属开销。
	listener, err := kcp.ListenWithOptions(ser.GetAddr(), nil, 0, 0)
	if err != nil {
		zlog.Error("Failed to start KCP server",
			zap.String("addr", ser.GetAddr()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to listen")))
		return
	}
	listener.SetReadBuffer(4 * 1024 * 1024)
	listener.SetWriteBuffer(4 * 1024 * 1024)
	zlog.Info("KCP server started successfully", zap.String("addr", ser.GetAddr()))
	ser.kcpListener = listener
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
		conn, err1 := ser.kcpListener.AcceptKCP()
		if err1 != nil {
			var opError *net.OpError
			if zerrs.As(err1, &opError) {
				zlog.Info("KCP server listener closed")
				return
			}
			zlog.Warn("Failed to accept KCP connection",
				zap.Error(zerrs.Wrap(err1, zerrs.ErrTypeNetwork, "accept failed")))
			continue
		}

		go func(c *kcp.UDPSession) {
			defer zlog.Recover("KServer handler recover")
			// KCP 参数设置
			c.SetNoDelay(1, 20, 2, 1)
			c.SetWindowSize(128, 128)
			c.SetACKNoDelay(true)
			c.SetMtu(1200)
			c.SetReadBuffer(4 * 1024 * 1024)
			c.SetWriteBuffer(4 * 1024 * 1024)

			channelId := ser.NextId()
			channel := NewChannel(channelId, c, ser)

			if !ser.HandleAccept(channel) {
				err := c.Close()
				if err != nil {
					zlog.Warn("Failed to close KCP connection", zap.Error(err))
				}
				zlog.Warn("KServer OnAccept rejected", zap.Uint64("channelId", channelId))
				return
			}
			ser.AddChannel(channel)
			defer channel.Close() // ✅ 自动清理（从 map 中移除 + 触发回调）
			ser.RunChannel(ctx, channel)
		}(conn)
	}
}

// Server 实现 ziface.IServer：启动 KCP 监听，直至 ctx 取消或 Close。
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zlog.Recover("KServer Server recover")
		ser.start(ctx)
	}()
}

// Close 关闭 KCP 服务（关闭 listener 与所有连接）。
func (ser *Server) Close() {
	ser.OnceDo(func() {
		close(ser.GetClose())
	})
	ser.BaseClose()
	zlog.Info("KCP server closed successfully", zap.String("addr", ser.GetAddr()))
}
