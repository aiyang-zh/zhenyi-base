package zws

import (
	"context"
	"github.com/aiyang-zh/zhenyi-core/zlog"
	"github.com/aiyang-zh/zhenyi-core/znet"
	"github.com/aiyang-zh/zhenyi-util/zsafe"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-util/zerrs"
)

/*tcp连接管理器*/

type Server struct {
	*znet.BaseServer
	upgrader websocket.Upgrader
	sl       *http.Server
}

func NewServer(addr string, handlers znet.ServerHandlers) *Server {
	return &Server{BaseServer: znet.NewBaseServer(addr, handlers)}
}

func (ser *Server) CheckOrigin(r *http.Request) bool {
	return true
}

// 监听
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
		defer zsafe.Recover("WServer start recover")
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
	defer zsafe.Recover("WServer handler recover")
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

// Server 开启服务
func (ser *Server) Server(ctx context.Context) {
	go func() {
		defer zsafe.Recover("WServer Server recover")
		ser.start(ctx)
	}()
}
func (ser *Server) Close() {
	// ✅ 修复：once.Do 确保只执行一次，先关闭 closeCh 通知 goroutine
	ser.OnceDo(func() {
		close(ser.GetClose())
	})

	// 优雅关闭 HTTP 服务器，带超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ser.sl.Shutdown(ctx)
	if err != nil {
		zlog.Warn("Failed to shutdown WebSocket server gracefully",
			zap.String("addr", ser.GetAddr()),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeNetwork, "shutdown failed")))
	}

	// 清理 listener 和所有 channels
	ser.BaseClose()
	zlog.Info("WebSocket server closed successfully", zap.String("addr", ser.GetAddr()))
}
