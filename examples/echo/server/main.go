package main

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/znet"
	"github.com/aiyang-zh/zhenyi-core/zserver"
)

func main() {
	s := zserver.New(zserver.WithAddr(":9001"), zserver.WithProtocol(znet.KCP))

	s.OnConnect(func(conn *zserver.Conn) {
		fmt.Printf("[echo] client connected: %d\n", conn.Id())
	})

	s.OnDisconnect(func(conn *zserver.Conn) {
		fmt.Printf("[echo] client disconnected: %d\n", conn.Id())
	})

	s.Handle(1, func(req *zserver.Request) {
		fmt.Printf("[echo] recv msgId=%d len=%d\n", req.MsgId(), len(req.Data()))
		req.Reply(1, req.Data())
	})

	fmt.Println("[echo] starting server on :9001")
	s.Run()
}
