package zws

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

// Client 为 WebSocket 协议客户端，嵌入 BaseClient 并实现 Connect（ws://addr/）。
type Client struct {
	*znet.BaseClient
}

// NewClient 创建 WebSocket 客户端并连接 addr；失败返回错误。
func NewClient(addr string) (ziface.IClient, error) {
	client := &Client{
		BaseClient: znet.NewBaseClient(),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
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
	n.SetConn(conn1)

	zlog.Info("WebSocket client connected successfully", zap.String("addr", addr))
	return nil
}
