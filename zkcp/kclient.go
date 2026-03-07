package zkcp

import (
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/zlog"
	"github.com/aiyang-zh/zhenyi-core/znet"
	"github.com/aiyang-zh/zhenyi-util/zerrs"
	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/zap"
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
	conn, err := kcp.DialWithOptions(addr, nil, 0, 0)
	if err != nil {
		zlog.Error("Failed to dial KCP server",
			zap.String("addr", addr),
			zap.Error(err))
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to dial KCP server")
	}
	conn.SetNoDelay(1, 20, 2, 1)
	conn.SetWindowSize(128, 128)
	conn.SetACKNoDelay(true)
	conn.SetMtu(1200)

	// 5. 客户端特有：设置写入缓冲区
	// 压测时瞬间发送大量包，系统 UDP 缓冲区可能会满
	conn.SetWriteBuffer(4 * 1024 * 1024) // 4MB
	conn.SetReadBuffer(4 * 1024 * 1024)  // 4MB
	n.SetConn(conn)

	zlog.Info("KCP client connected successfully", zap.String("addr", addr))
	return nil
}
