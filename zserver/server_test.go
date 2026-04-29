package zserver

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	s := New()
	if s.addr != ":9001" {
		t.Errorf("default addr: got %s, want :9001", s.addr)
	}
	if s.protocol != znet.TCP {
		t.Errorf("default protocol: got %d, want TCP", s.protocol)
	}
}

func TestNew_WithOptions(t *testing.T) {
	s := New(
		WithAddr(":8888"),
		WithProtocol(znet.WebSocket),
		WithWorkers(8),
		WithMaxConnections(1000),
	)
	if s.addr != ":8888" {
		t.Errorf("addr: got %s, want :8888", s.addr)
	}
	if s.protocol != znet.WebSocket {
		t.Errorf("protocol: got %d, want WebSocket", s.protocol)
	}
	if s.workerSize != 8 {
		t.Errorf("workerSize: got %d, want 8", s.workerSize)
	}
	if s.maxConn != 1000 {
		t.Errorf("maxConns: got %d, want 1000", s.maxConn)
	}
}

func TestHandle_Registration(t *testing.T) {
	s := New()
	called := false
	s.Handle(1001, func(req *Request) { called = true })

	handler, ok := s.router[1001]
	if !ok {
		t.Fatal("handler 1001 not registered")
	}
	handler(&Request{})
	if !called {
		t.Error("handler was not called")
	}
}

func TestRequest_Fields(t *testing.T) {
	req := &Request{
		msgId: 42,
		seqId: 7,
		data:  []byte("hello"),
	}
	if req.MsgId() != 42 {
		t.Errorf("MsgId: got %d, want 42", req.MsgId())
	}
	if req.SeqId() != 7 {
		t.Errorf("SeqId: got %d, want 7", req.SeqId())
	}
	if string(req.Data()) != "hello" {
		t.Errorf("Data: got %s, want hello", req.Data())
	}
}

func TestRequest_DataCopy_Independent(t *testing.T) {
	req := &Request{data: []byte("hello")}
	cp := req.DataCopy()
	if string(cp) != "hello" {
		t.Fatalf("DataCopy: got %q", string(cp))
	}
	req.data[0] = 'X'
	if string(cp) != "hello" {
		t.Fatalf("DataCopy should be independent, got %q", string(cp))
	}
}

func TestEchoRequestMode(t *testing.T) {
	s := New(WithAddr("127.0.0.1:0"))

	s.Handle(1, func(req *Request) {
		req.Reply(1, req.Data())
	})

	s.Start()
	time.Sleep(100 * time.Millisecond)
	defer s.Stop()

	addr := s.Addr()
	client, err := ztcp.NewClient(addr) // 默认 Request 模式
	if err != nil {
		t.Fatalf("connect to %s failed: %v", addr, err)
	}
	defer client.Close()

	// 单次 Request
	resp, err := client.Request(&znet.NetMessage{MsgId: 1, Data: []byte("ping")})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if string(resp.GetMessageData()) != "ping" {
		t.Errorf("echo data: got %s, want ping", string(resp.GetMessageData()))
	}

	// 多次 Request
	for _, payload := range [][]byte{[]byte("a"), []byte("b"), []byte("c")} {
		resp, err = client.Request(&znet.NetMessage{MsgId: 1, Data: payload})
		if err != nil {
			t.Fatalf("Request %s failed: %v", string(payload), err)
		}
		if string(resp.GetMessageData()) != string(payload) {
			t.Errorf("Request: got %s, want %s", string(resp.GetMessageData()), string(payload))
		}
	}
}

func TestEchoIntegration(t *testing.T) {
	s := New(WithAddr("127.0.0.1:0"))

	s.Handle(1, func(req *Request) {
		req.Reply(1, req.Data())
	})

	s.Start()
	time.Sleep(100 * time.Millisecond)
	defer s.Stop()

	addr := s.Addr()
	client, err := ztcp.NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("connect to %s failed: %v", addr, err)
	}
	defer client.Close()

	var received atomic.Int32
	var lastData atomic.Value

	client.SetReadCall(func(msg ziface.IWireMessage) {
		received.Add(1)
		lastData.Store(append([]byte{}, msg.GetMessageData()...))
	})
	client.Read()

	msg := &znet.NetMessage{MsgId: 1, Data: []byte("ping")}
	client.SendMsg(msg)

	time.Sleep(300 * time.Millisecond)

	if received.Load() == 0 {
		t.Fatal("no response received")
	}

	if data, ok := lastData.Load().([]byte); !ok || string(data) != "ping" {
		t.Errorf("echo data: got %s, want ping", data)
	}
}

func TestEchoMultipleMessages(t *testing.T) {
	s := New(WithAddr("127.0.0.1:0"))

	s.Handle(1, func(req *Request) {
		req.Reply(1, req.Data())
	})

	s.Start()
	time.Sleep(100 * time.Millisecond)
	defer s.Stop()

	addr := s.Addr()
	client, err := ztcp.NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		t.Fatalf("connect to %s failed: %v", addr, err)
	}
	defer client.Close()

	var received atomic.Int32
	client.SetReadCall(func(msg ziface.IWireMessage) {
		received.Add(1)
	})
	client.Read()

	total := 50
	for i := 0; i < total; i++ {
		msg := &znet.NetMessage{MsgId: 1, Data: []byte("test")}
		client.SendMsg(msg)
		time.Sleep(time.Millisecond)
	}

	time.Sleep(2 * time.Second)

	count := received.Load()
	if count < int32(total) {
		t.Errorf("received %d/%d messages", count, total)
	}
	t.Logf("echo: sent %d, received %d", total, count)
}
