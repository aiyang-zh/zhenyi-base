package zws

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

// Server 为 WebSocket 协议的服务端实现，基于 HTTP 升级与 BaseServer。
type Server struct {
	*znet.BaseServer
	upgrader websocket.Upgrader
	sl       *http.Server
}

// NewServer 创建 WebSocket 服务端；addr 为监听地址，handlers 必须提供 OnAccept 与 OnRead。
func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

// CheckOrigin 用于 Upgrader，默认允许任意 Origin。
//
// 生产环境建议按需覆盖并校验 Origin，避免 CSWS（跨站 WebSocket）风险。
// 可按需覆盖。
func (ser *Server) CheckOrigin(r *http.Request) bool {
	return true
}

// start 启动 HTTP 服务并在 "/" 上处理 WebSocket 升级与连接。
func (ser *Server) start(ctx context.Context) {
	ser.upgrader = websocket.Upgrader{
		HandshakeTimeout: znet.WebSocketTimeout,
		ReadBufferSize:   znet.ReadBufferSize,
		WriteBufferSize:  znet.WriteBufferSize,
		CheckOrigin:      ser.CheckOrigin,
	}
	s := http.NewServeMux()
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ser.handler(ctx, w, r)
	})
	ser.sl = &http.Server{
		Addr:    ser.GetAddr(),
		Handler: s,
	}

	// 启动HTTP服务器的goroutine
	errCh := make(chan error, 1)
	go func() {
		defer zlog.Recover("WServer start recover")
		err := ser.sl.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 等待Context取消或closeCh
	select {
	case <-ctx.Done():
		ser.Close()
	case <-ser.GetClose():
		ser.Close()
	case err := <-errCh:
		zlog.Error("Failed to start WebSocket server",
			zap.String("addr", ser.GetAddr()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "ListenAndServe failed")))
	}
}

func (ser *Server) handler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	defer zlog.Recover("WServer handler recover")
	conn, err := ser.upgrader.Upgrade(w, r, nil)
	if err != nil {
		zlog.Warn("Failed to upgrade WebSocket connection",
			zap.String("remoteAddr", r.RemoteAddr),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "upgrade failed")))
		return
	}
	channelId := ser.NextId()
	channel := NewChannel(channelId, conn, ser)

	if !ser.HandleAccept(channel) {
		err := conn.Close()
		if err != nil {
			zlog.Warn("Failed to close WebSocket connection", zap.Error(err))
			return
		}
		zlog.Warn("WServer OnAccept rejected", zap.Uint64("channelId", channelId))
		return
	}
	ser.AddChannel(channel)
	defer channel.Close() // ✅ 自动清理（从 map 中移除 + 触发回调）
	channel.StartSend(ctx)
	channel.Start() // 阻塞直到 WebSocket 连接关闭
}

// Server 实现 ziface.IServer：启动 HTTP 监听，收到 WebSocket 后按连接处理直至 ctx 取消或 Close。
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zlog.Recover("WServer Server recover")
		ser.start(ctx)
	}()
}

// Close 优雅关闭 WebSocket 服务（Shutdown HTTP 并关闭所有连接）。
func (ser *Server) Close() {
	ser.OnceDo(func() {
		close(ser.GetClose())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ser.sl != nil {
		err := ser.sl.Shutdown(ctx)
		if err != nil {
			zlog.Warn("Failed to shutdown WebSocket server gracefully",
				zap.String("addr", ser.GetAddr()),
				zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "shutdown failed")))
		}
	} else {
		zlog.Debug("WebSocket server not started yet, skip HTTP shutdown", zap.String("addr", ser.GetAddr()))
	}

	ser.BaseClose()
	zlog.Info("WebSocket server closed successfully", zap.String("addr", ser.GetAddr()))
}
