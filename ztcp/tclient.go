package ztcp

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

type TClient struct {
	*znet.BaseClient
}

func NewClient(addr string) (ziface.IClient, error) {
	// 创建连接
	client := &TClient{
		BaseClient: znet.NewBaseClient(),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (n *TClient) Connect(addr string) error {
	// 开始连接
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
