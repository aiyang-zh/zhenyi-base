package zws

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/zlog"
	"github.com/aiyang-zh/zhenyi-core/znet"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-util/zerrs"
)

type Client struct {
	*znet.BaseClient
}

func NewClient(addr string) (ziface.IClient, error) {
	// 创建连接
	client := &Client{
		BaseClient: znet.NewBaseClient(),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (n *Client) Connect(addr string) error {
	// 开始连接
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
