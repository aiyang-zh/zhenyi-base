package zkcp

import (
	"github.com/aiyang-zh/zhenyi-base/znet"
	"net"
)

/*kcp连接对象*/

type Channel struct {
	*znet.BaseChannel
}

// NewChannel 创建连接
func NewChannel(channelId uint64, conn net.Conn, server *Server) *Channel {
	return &Channel{
		BaseChannel: znet.NewBaseChannel(channelId, conn, server),
	}
}
