package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserver"
)

//go:embed web/*
var webFS embed.FS

const (
	msgJoin = 1
	msgChat = 2
	msgEvt  = 10
	msgErr  = 99
)

type chatEvent struct {
	Type string `json:"type"`
	User string `json:"user,omitempty"`
	Text string `json:"text,omitempty"`
}

func main() {
	wsAddr := flag.String("addr", ":9001", "WebSocket listen address")
	httpAddr := flag.String("http", ":8080", "HTTP static (index.html)")
	flag.Parse()

	var mu sync.Mutex
	nicks := make(map[uint64]string)
	conns := make(map[uint64]*zserver.Conn)

	s := zserver.New(
		zserver.WithAddr(*wsAddr),
		zserver.WithProtocol(znet.WebSocket),
		zserver.WithName("examples/groupchat"),
		zserver.WithAsyncMode(), // 广播需 Send 入队，sync 模式下会丢弃
	)

	s.OnConnect(func(c *zserver.Conn) {
		mu.Lock()
		conns[c.Id()] = c
		mu.Unlock()
	})

	s.OnDisconnect(func(c *zserver.Conn) {
		mu.Lock()
		nick := nicks[c.Id()]
		delete(nicks, c.Id())
		delete(conns, c.Id())
		others := make([]*zserver.Conn, 0, len(conns))
		for _, x := range conns {
			others = append(others, x)
		}
		mu.Unlock()
		if nick == "" {
			return
		}
		b, err := json.Marshal(chatEvent{Type: "leave", User: nick})
		if err != nil {
			return
		}
		for _, x := range others {
			x.Send(msgEvt, b)
		}
	})

	s.Handle(msgJoin, func(req *zserver.Request) {
		nick := strings.TrimSpace(string(req.Data()))
		if nick == "" || len(nick) > 24 {
			req.Reply(msgErr, []byte("invalid nick"))
			return
		}
		id := req.Conn().Id()
		mu.Lock()
		if nicks[id] != "" {
			mu.Unlock()
			req.Reply(msgErr, []byte("already joined"))
			return
		}
		nicks[id] = nick
		mu.Unlock()
		broadcast(conns, &mu, msgEvt, chatEvent{Type: "join", User: nick})
	})

	s.Handle(msgChat, func(req *zserver.Request) {
		text := strings.TrimSpace(string(req.Data()))
		if text == "" || len(text) > 512 {
			req.Reply(msgErr, []byte("invalid text"))
			return
		}
		mu.Lock()
		nick := nicks[req.Conn().Id()]
		mu.Unlock()
		if nick == "" {
			req.Reply(msgErr, []byte("join first"))
			return
		}
		broadcast(conns, &mu, msgEvt, chatEvent{Type: "say", User: nick, Text: text})
	})

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}

	go func() {
		fmt.Printf("[groupchat] open http://127.0.0.1%s (WS ws://127.0.0.1%s)\n", *httpAddr, *wsAddr)
		if e := http.ListenAndServe(*httpAddr, http.FileServer(http.FS(sub))); e != nil {
			fmt.Println("http:", e)
		}
	}()

	s.Run()
}

func broadcast(conns map[uint64]*zserver.Conn, mu *sync.Mutex, msgID int32, ev chatEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	mu.Lock()
	out := make([]*zserver.Conn, 0, len(conns))
	for _, c := range conns {
		out = append(out, c)
	}
	mu.Unlock()
	for _, c := range out {
		c.Send(msgID, b)
	}
}
