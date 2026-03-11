package zws

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/aiyang-zh/zhenyi-base/zerrs"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client 为 WebSocket 协议客户端，嵌入 BaseClient 并实现 Connect（ws://addr/）。
type Client struct {
	*znet.BaseClient
}

// NewClient 创建 WebSocket 客户端并连接 addr；失败返回错误。
// 默认 sync（Request）；可选 znet.WithAsyncMode() 启用 async（Read），与 ziface.ModeAsync 对应。
func NewClient(addr string, opts ...znet.ClientOption) (ziface.IClient, error) {
	client := &Client{
		BaseClient: znet.NewBaseClient(opts...),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// setNoDelay 对底层 TCP 连接禁用 Nagle（支持 net.TCPConn 与 tls.Conn 封装）。
func setNoDelay(c net.Conn) {
	if tcpConn, ok := c.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		return
	}
	if tlsConn, ok := c.(*tls.Conn); ok {
		if raw := tlsConn.NetConn(); raw != nil {
			if tcpConn, ok := raw.(*net.TCPConn); ok {
				_ = tcpConn.SetNoDelay(true)
			}
		}
	}
}

// Connect 使用 WebSocket 连接到 ws://addr/。
func (n *Client) Connect(addr string) error {
	addrInfo := fmt.Sprintf("ws://%s/", addr)
	conn, _, err := websocket.DefaultDialer.Dial(addrInfo, nil)
	if err != nil {
		zlog.Error("Failed to dial WebSocket server",
			zap.String("addr", addr),
			zap.Error(err))
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to dial WebSocket server")
	}
	conn1 := conn.NetConn()
	setNoDelay(conn1) // 禁用底层 TCP Nagle，降低 sync/RPC 场景延迟
	n.SetConn(conn1)

	zlog.Info("WebSocket client connected successfully", zap.String("addr", addr))
	return nil
}
