package zserver

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
)

// Conn 是 network.IChannel 的轻量封装，只暴露给业务侧使用的安全 API。
type Conn struct {
	channel ziface.IChannel
	server  *Server
}

func newConn(ch ziface.IChannel, s *Server) *Conn {
	return &Conn{channel: ch, server: s}
}

// Id 返回连接在当前服务器中的唯一 ChannelID。
func (c *Conn) Id() uint64 { return c.channel.GetChannelId() }

// AuthId 返回业务侧设置的认证 ID（例如用户 ID / 设备 ID）。
func (c *Conn) AuthId() uint64 { return c.channel.GetAuthId() }

// SetAuthId 绑定业务认证 ID，之后可通过 Server.GetConnByAuthId 查询到该连接。
func (c *Conn) SetAuthId(id uint64) {
	c.server.net.SetChannelAuth(c.channel.GetChannelId(), id)
}

// Send 发送消息给该连接。data 是业务层 payload（已编码）。异步入队，由 send 协程写出。
func (c *Conn) Send(msgId int32, data []byte) {
	msg := znet.GetNetMessage()
	msg.MsgId = msgId
	msg.SetDataCopy(data)
	c.channel.Send(msg)
}

// SendImmediate 读协程内同步直写，sync/RPC 场景原生支持，直接写出降低延迟。
// 必须从 OnRead 回调所在的 goroutine 调用（如 DirectDispatch 的 handler 内），否则与 send 协程并发写会出错。
func (c *Conn) SendImmediate(msgId int32, seqId uint32, data []byte) error {
	msg := &znet.NetMessage{MsgId: msgId, SeqId: seqId, Data: data}
	return c.channel.WriteImmediate(msg)
}

// Close 关闭连接。
func (c *Conn) Close() { c.channel.Close() }

func (s *Server) getOrCreateConn(ch ziface.IChannel) *Conn {
	key := ch.GetChannelId()
	if v, ok := s.connCache.Load(key); ok {
		return v
	}
	c := newConn(ch, s)
	s.connCache.Store(key, c)
	return c
}

func (s *Server) removeConn(ch ziface.IChannel) {
	s.connCache.Delete(ch.GetChannelId())
}
