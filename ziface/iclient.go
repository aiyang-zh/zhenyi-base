package ziface

import (
	"net"
)

// IClient 通用客户端接口。
//
// 封装了连接、发送、接收和加密配置等操作，具体实现由 ztcp/zws/zkcp 提供。
type IClient interface {
	// Connect 连接到指定地址，例如 "127.0.0.1:9001"。
	Connect(addr string) error

	// Close 关闭底层连接并释放资源。
	Close() error

	// SendMsg 发送一条消息。
	SendMsg(message IMessage)

	// Read 启动读循环，从服务器持续接收消息。
	// 通常应在单独 goroutine 中调用。
	Read()

	// SetReadCall 设置收到消息时的回调函数。
	SetReadCall(readCall func(IWireMessage))

	// SetEncrypt 配置加密实现（如不需要加密可不调用）。
	SetEncrypt(iEncrypt IEncrypt)

	// IsOpen 返回连接是否处于打开状态。
	IsOpen() bool

	// GetConn 返回底层 net.Conn。
	// 一般仅用于调试或非常特殊的场景。
	GetConn() net.Conn
}
