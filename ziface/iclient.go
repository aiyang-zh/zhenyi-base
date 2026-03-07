package ziface

import (
	"net"
)

type IClient interface {
	Connect(addr string) error
	Close() error
	SendMsg(message IMessage)
	Read()
	SetReadCall(readCall func(IWireMessage))
	SetEncrypt(iEncrypt IEncrypt)
	IsOpen() bool
	GetConn() net.Conn
}
