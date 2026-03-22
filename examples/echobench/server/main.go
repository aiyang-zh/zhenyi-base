package main

import (
	"flag"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"strings"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserver"
)

func main() {
	addr := flag.String("addr", ":9001", "listen address")
	protocol := flag.String("p", "tcp", "protocol: tcp|ws|kcp")
	quiet := flag.Bool("quiet", false, "disable logs (for benchmark)")
	flag.Parse()
	logConfig := zlog.NewDefaultLoggerConfig()
	logConfig.WithOptions(
		zlog.WithProduction(),
		zlog.WithFilename("server"),
	)
	zlog.NewDefaultLoggerWithConfig(logConfig)
	defer func() {
		_ = zlog.CloseDefaultLog()
	}()
	proto := parseProtocol(*protocol)
	s := zserver.New(
		zserver.WithAddr(*addr),
		zserver.WithProtocol(proto),
		zserver.WithName("echobench/server"),
		zserver.WithAsyncMode(), // 异步模式：发送队列，Reply 异步入队
		zserver.WithDirectDispatch(),
		zserver.WithDirectDispatchRef(),
		zserver.WithHeartbeatTimeout(0), // 压测时禁用心跳，避免 1k1k 下调度延迟导致误断
		zserver.WithBanner(!*quiet),     // 压测 -quiet 时不打印 ASCII 标识
	)

	if !*quiet {
		s.OnConnect(func(conn *zserver.Conn) {
			fmt.Printf("[echo] client connected: %d\n", conn.Id())
		})
		s.OnDisconnect(func(conn *zserver.Conn) {
			fmt.Printf("[echo] client disconnected: %d\n", conn.Id())
		})
	}
	var count int64
	s.Handle(1, func(req *zserver.Request) {
		if !*quiet {
			fmt.Printf("[echo] recv msgId=%d len=%d\n", req.MsgId(), len(req.Data()))
		}
		fmt.Printf("count=%d\n", atomic.AddInt64(&count, 1))
		req.Reply(1, req.Data())
	})

	if !*quiet {
		fmt.Printf("[echobench/server] listen %s (%s)\n", *addr, *protocol)
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
