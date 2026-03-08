package zkcp

import (
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/zap"
)

// Client 为 KCP 协议客户端，嵌入 BaseClient 并实现 Connect。
type Client struct {
	*znet.BaseClient
}

// NewClient 创建 KCP 客户端并连接 addr；失败返回错误。
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

// Connect 使用 KCP 连接到指定地址（FEC 关闭，与 Server 一致）。
func (n *Client) Connect(addr string) error {
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
