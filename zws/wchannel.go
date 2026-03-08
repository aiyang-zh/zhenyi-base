package zws

import (
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/gorilla/websocket"
)

// Channel 为 WebSocket 连接的通道封装，嵌入 BaseChannel。
type Channel struct {
	*znet.BaseChannel
}

// NewChannel 创建 WebSocket 通道（由 Server 在 Upgrade 后调用）。
func NewChannel(channelId uint64, conn *websocket.Conn, server *Server) *Channel {
	return &Channel{
		BaseChannel: znet.NewBaseChannel(channelId, conn.NetConn(), server),
	}
}
