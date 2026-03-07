package zws

import (
	"github.com/aiyang-zh/zhenyi-core/znet"
	"github.com/gorilla/websocket"
)

/*tcp连接对象*/

type Channel struct {
	*znet.BaseChannel
}

// NewChannel 创建连接
func NewChannel(channelId uint64, conn *websocket.Conn, server *Server) *Channel {
	return &Channel{
		BaseChannel: znet.NewBaseChannel(channelId, conn.NetConn(), server),
	}
}
