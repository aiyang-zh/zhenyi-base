package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zbrand"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zkcp"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"github.com/aiyang-zh/zhenyi-base/zws"
)

var (
	addr     = flag.String("addr", "127.0.0.1:9001", "server address")
	bench    = flag.Bool("bench", false, "run benchmark mode")
	total    = flag.Int("n", 10000, "total messages (benchmark mode)")
	clients  = flag.Int("c", 1, "concurrent clients (benchmark mode)")
	protocol = flag.String("p", "tcp", "protocol: tcp|ws|kcp (or 1|2|3)")
	size     = flag.Int("size", 23, "payload size in bytes (benchmark mode)")
)

func main() {
	flag.Parse()
	logConfig := zlog.NewDefaultLoggerConfig()
	logConfig.WithOptions(
		zlog.WithProduction(),
		zlog.WithFilename("client"),
	)
	zlog.NewDefaultLoggerWithConfig(logConfig)
	defer func() {
		_ = zlog.CloseDefaultLog()
	}()
	if *bench {
		runBenchmark()
	} else {
		runInteractive()
	}
}

func newClient() (ziface.IClient, error) {
	switch parseProtocol(*protocol) {
	case znet.TCP:
		return ztcp.NewClient(*addr, znet.WithAsyncMode())
	case znet.WebSocket:
		return zws.NewClient(*addr, znet.WithAsyncMode())
	case znet.KCP:
		return zkcp.NewClient(*addr, znet.WithAsyncMode())
	default:
		return nil, zerrs.New(zerrs.ErrTypeNetwork, "protocol error: use tcp|ws|kcp")
	}
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

func makePayload(size int) []byte {
	if size <= 0 {
		size = 23
	}
	base := []byte("hello zhenyi benchmark")
	if size <= len(base) {
		return base[:size]
	}
	b := make([]byte, size)
	copy(b, base)
	for i := len(base); i < size; i++ {
		b[i] = base[i%len(base)]
	}
	return b
}

func runInteractive() {
	var client ziface.IClient
	var err error
	client, err = newClient()
	if err != nil {
		fmt.Printf("connect failed: %v\n", err)
		return
	}

	fmt.Print(zbrand.Banner)
	fmt.Printf("[echobench/client] connected to %s (type message and press Enter, Ctrl+C to quit)\n", *addr)

	client.SetReadCall(func(msg ziface.IWireMessage) {
		fmt.Printf("< recv msgId=%d: %s\n", msg.GetMsgId(), string(msg.GetMessageData()))
	})
	client.Read()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		msg := &znet.NetMessage{MsgId: 1, Data: []byte(text)}
		client.SendMsg(msg)
		fmt.Printf("> sent: %s\n", text)
	}
}

func runBenchmark() {
	perClient := *total / *clients
	payload := makePayload(*size)

	fmt.Printf("benchmark: %d clients x %d msgs = %d total, payload=%d bytes\n",
		*clients, perClient, perClient**clients, len(payload))

	var sendWg sync.WaitGroup
	var totalRecv atomic.Int64
	var totalSent atomic.Int64

	clientList := make([]ziface.IClient, 0, *clients)
	for i := 0; i < *clients; i++ {
		client, err := newClient()
		if err != nil {
			fmt.Printf("client %d connect failed: %v\n", i, err)
			continue
		}
		client.SetReadCall(func(msg ziface.IWireMessage) {
			totalRecv.Add(1)
		})
		client.Read()
		clientList = append(clientList, client)
	}

	fmt.Printf("%d clients connected, sending...\n", len(clientList))
	os.Stdout.Sync()
	start := time.Now()

	// 进度输出：1k1k 等场景可能跑 1～2 分钟，避免误以为卡住
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s, r := totalSent.Load(), totalRecv.Load()
				fmt.Printf("\r  progress: sent %d  recv %d (waiting for responses...)  ", s, r)
				os.Stdout.Sync()
			}
		}
	}()

	msg := &znet.NetMessage{MsgId: 1, Data: payload}
	for _, client := range clientList {
		sendWg.Add(1)
		go func(c ziface.IClient) {
			defer sendWg.Done()
			for j := 0; j < perClient; j++ {
				c.SendMsg(msg)
			}
			totalSent.Add(int64(perClient))
		}(client)
		// time.Sleep(1 * time.Millisecond)
	}

	// 发送阶段最多等 5 分钟，避免服务端过载时 Write 永久阻塞
	sendDone := make(chan struct{})
	go func() { sendWg.Wait(); close(sendDone) }()
	select {
	case <-sendDone:
		// 正常
	case <-time.After(5 * time.Minute):
		fmt.Printf("\ntimeout: sends did not finish in 5 min (server overloaded? try fewer clients)\n")
		cancel()
		for _, c := range clientList {
			c.Close()
		}
		return
	}
	sent := totalSent.Load()

	// 等收齐回包，带超时与无进展检测，避免无限卡住
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	recvDeadline := time.Now().Add(5 * time.Minute)
	lastRecv := totalRecv.Load()
	lastProgress := time.Now()
	for totalRecv.Load() < sent {
		<-ticker.C
		now := time.Now()
		if now.After(recvDeadline) {
			fmt.Printf("\ntimeout: did not receive all responses within 5 min (recv %d / sent %d)\n", totalRecv.Load(), sent)
			break
		}
		r := totalRecv.Load()
		if r > lastRecv {
			lastRecv = r
			lastProgress = now
		} else if now.Sub(lastProgress) > 5*time.Minute {
			fmt.Printf("\nstuck: recv unchanged at %d for 5 min (sent %d), aborting\n", r, sent)
			break
		}
	}
	cancel() // 停止进度 goroutine

	elapsed := time.Since(start)
	recv := totalRecv.Load()

	for _, c := range clientList {
		c.Close()
	}

	pct := float64(recv) / float64(max(sent, 1)) * 100
	fmt.Printf("\n--- benchmark result ---\n")
	fmt.Printf("elapsed:  %v\n", elapsed)
	fmt.Printf("sent:     %d\n", sent)
	fmt.Printf("recv:     %d (%.1f%%)\n", recv, pct)
	if recv < sent {
		fmt.Printf("warning:  %d responses not received (possible loss)\n", sent-recv)
	} else {
		fmt.Printf("loss:     none (recv == sent)\n")
	}
	fmt.Printf("qps:      %.0f msg/s\n", float64(recv)/elapsed.Seconds())
}
