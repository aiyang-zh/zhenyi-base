package zkcp

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zsafe"
	"github.com/xtaci/kcp-go/v5"
	"net"

	"go.uber.org/zap"
)

/*kcp连接管理器*/

type Server struct {
	*znet.BaseServer
	kcpListener *kcp.Listener
}

func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

// 监听
func (ser *Server) start(ctx context.Context) {
	// 启动kcp连接
	zlog.Info("Starting KCP server", zap.String("addr", ser.GetAddr()))
	listener, err := kcp.ListenWithOptions(ser.GetAddr(), nil, 10, 3)
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
	for {
		select {
		case <-ctx.Done():
			ser.Close()
			return
		case <-ser.GetClose():
			ser.Close()
			return
		default:
			// 等待客户端连接
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

			// ✅ Accept 后立即启动 goroutine，避免阻塞 Accept 循环
			go func(c *kcp.UDPSession) {
				defer zsafe.Recover("KServer handler recover")
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
				channel.StartSend(ctx)
				channel.Start()
			}(conn)
		}
	}
}

// Server 开启服务
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zsafe.Recover("KServer Server recover")
		ser.start(ctx)
	}()
}

func (ser *Server) Close() {
	ser.OnceDo(func() {
		close(ser.GetClose())
	})
	ser.BaseClose()
	zlog.Info("KCP server closed successfully", zap.String("addr", ser.GetAddr()))
}
