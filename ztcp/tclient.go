package ztcp

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

// TClient 为 TCP 协议客户端，嵌入 BaseClient 并实现 Connect。
type TClient struct {
	*znet.BaseClient
}

// NewClient 创建 TCP 客户端并连接 addr；失败返回错误。
func NewClient(addr string) (ziface.IClient, error) {
	client := &TClient{
		BaseClient: znet.NewBaseClient(),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Connect 使用 TCP 连接到指定地址（格式如 "host:port"）。
func (n *TClient) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		zlog.Error("Failed to dial TCP server",
			zap.String("addr", addr),
			zap.Error(err))
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to dial TCP server")
	}
	n.SetConn(conn)

	zlog.Info("TCP client connected successfully", zap.String("addr", addr))
	return nil
}
