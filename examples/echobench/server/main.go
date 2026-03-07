package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserver"
)

func main() {
	addr := flag.String("addr", ":9001", "listen address")
	protocol := flag.String("p", "tcp", "protocol: tcp|ws|kcp")
	quiet := flag.Bool("quiet", false, "disable logs (for benchmark)")
	flag.Parse()

	proto := parseProtocol(*protocol)
	s := zserver.New(
		zserver.WithAddr(*addr),
		zserver.WithProtocol(proto),
		zserver.WithDirectDispatch(), // 纯 echo：handler 在读 goroutine 内直接执行，无 worker pool 开销
	)

	if !*quiet {
		s.OnConnect(func(conn *zserver.Conn) {
			fmt.Printf("[echo] client connected: %d\n", conn.Id())
		})
		s.OnDisconnect(func(conn *zserver.Conn) {
			fmt.Printf("[echo] client disconnected: %d\n", conn.Id())
		})
	}

	s.Handle(1, func(req *zserver.Request) {
		if !*quiet {
			fmt.Printf("[echo] recv msgId=%d len=%d\n", req.MsgId(), len(req.Data()))
		}
		req.Reply(1, req.Data())
	})

	if !*quiet {
		fmt.Printf("[echo] starting server on %s (%s)\n", *addr, *protocol)
	}
	s.Run()
}

func parseProtocol(s string) znet.ConnProtocol {
	switch strings.ToLower(s) {
	case "tcp", "1":
		return znet.TCP
	case "kcp", "2":
		return znet.KCP
	case "ws", "websocket", "3":
		return znet.WebSocket
	default:
		return znet.TCP
	}
}
