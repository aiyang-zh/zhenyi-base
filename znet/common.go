package znet

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"time"
)

// ConnProtocol 表示传输层协议类型。
type ConnProtocol int

const (
	TCP ConnProtocol = iota + 1
	KCP
	WebSocket
)

const (
	WebSocketTimeout = 5 * time.Second // WebSocket 握手超时
	ReadBufferSize   = 1024            // WebSocket 读缓冲默认大小
	WriteBufferSize  = 1024            // WebSocket 写缓冲默认大小
)

// NetOperator 用于内部或扩展场景的操作描述符（Op/ServiceId/ChannelId/Message/Args）。
type NetOperator struct {
	Op        int
	ServiceId uint
	ChannelId uint64
	Message   ziface.IWireMessage
	Args      interface{}
}

// Socket 安全配置常量。
const (
	DefaultMaxHeaderLength = 10 * 1024   // 单包 header 最大长度（10KB）
	DefaultMaxDataLength   = 1024 * 1024 // 单包 body 最大长度（1MB）
	DefaultMaxMsgId        = 1000000000  // 消息 ID 合法范围 [-DefaultMaxMsgId, DefaultMaxMsgId]
)

// SocketConfig 为 BaseSocket 的解析与安全限制配置。
type SocketConfig struct {
	MaxHeaderLength int   // 单包 header 最大长度
	MaxDataLength   int   // 单包 body 最大长度
	MaxMsgId        int   // 消息 ID 合法绝对值上限
	ProtocolVersion uint8 // 协议版本：0=v0（12 字节 header），1=v1（13 字节，首字节为 version）
}

// DefaultSocketConfig 返回默认安全配置（兼容旧协议）。
func DefaultSocketConfig() SocketConfig {
	return SocketConfig{
		MaxHeaderLength: DefaultMaxHeaderLength,
		MaxDataLength:   DefaultMaxDataLength,
		MaxMsgId:        DefaultMaxMsgId,
		ProtocolVersion: 0, // 默认兼容旧协议
	}
}
