package znet

import (
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"time"
)

type ConnProtocol int

const (
	TCP ConnProtocol = iota + 1
	KCP
	WebSocket
)

const (
	WebSocketTimeout = 5 * time.Second
	ReadBufferSize   = 1024
	WriteBufferSize  = 1024
)

type NetOperator struct {
	Op        int
	ServiceId uint
	ChannelId uint64
	Message   ziface.IWireMessage
	Args      interface{}
}

// Socket安全配置常量
const (
	// DefaultMaxHeaderLength header最大长度限制 (10KB)
	DefaultMaxHeaderLength = 10 * 1024
	// DefaultMaxDataLength data最大长度限制 (1MB)
	DefaultMaxDataLength = 1024 * 1024
	// DefaultMaxMsgId 最大消息ID
	DefaultMaxMsgId = 1000000000
)

// SocketConfig Socket解析配置
type SocketConfig struct {
	MaxHeaderLength int   // header最大长度
	MaxDataLength   int   // data最大长度
	MaxMsgId        int   // 最大消息ID
	ProtocolVersion uint8 // 协议版本：0=v0 (12-byte header), 1=v1 (13-byte header, 首字节为 version)
}

// DefaultSocketConfig 返回默认安全配置
func DefaultSocketConfig() SocketConfig {
	return SocketConfig{
		MaxHeaderLength: DefaultMaxHeaderLength,
		MaxDataLength:   DefaultMaxDataLength,
		MaxMsgId:        DefaultMaxMsgId,
		ProtocolVersion: 0, // 默认兼容旧协议
	}
}
