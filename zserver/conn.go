package zserver

import (
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/znet"
)

// Conn 是 network.IChannel 的轻量封装，只暴露安全的 API。
type Conn struct {
	channel ziface.IChannel
	server  *Server
}

func newConn(ch ziface.IChannel, s *Server) *Conn {
	return &Conn{channel: ch, server: s}
}

func (c *Conn) Id() uint64    { return c.channel.GetChannelId() }
func (c *Conn) AuthId() int64 { return c.channel.GetAuthId() }
func (c *Conn) SetAuthId(id int64) {
	c.server.net.SetChannelAuth(c.channel.GetChannelId(), id)
}

// Send 发送消息给该连接。data 是业务层 payload（已编码）。
func (c *Conn) Send(msgId int32, data []byte) {
	msg := znet.GetNetMessage()
	msg.MsgId = msgId
	msg.Data = append(msg.Data[:0], data...)
	c.channel.Send(msg)
}

func (c *Conn) Close() { c.channel.Close() }

func (s *Server) getOrCreateConn(ch ziface.IChannel) *Conn {
	key := ch.GetChannelId()
	if v, ok := s.connCache.Load(key); ok {
		return v.(*Conn)
	}
	c := newConn(ch, s)
	s.connCache.Store(key, c)
	return c
}

func (s *Server) removeConn(ch ziface.IChannel) {
	s.connCache.Delete(ch.GetChannelId())
}
