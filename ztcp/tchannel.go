package ztcp

import (
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net"
)

// Channel 为 TCP 连接的通道封装，嵌入 BaseChannel。
type Channel struct {
	*znet.BaseChannel
}

// NewChannel 创建 TCP 通道（由 Server Accept 后调用）。
func NewChannel(channelId uint64, conn net.Conn, server *Server) *Channel {
	return &Channel{
		BaseChannel: znet.NewBaseChannel(channelId, conn, server),
	}
}
