package zserver

import (
	"context"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zgrace"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zkcp"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"github.com/aiyang-zh/zhenyi-base/zws"
	"runtime"
	"sync"

	"github.com/panjf2000/ants/v2"
)

// HandlerFunc 业务处理函数
type HandlerFunc func(req *Request)

// Server 轻量级服务器，基于 network 层封装。
// 提供 MsgID 路由、Worker Pool、连接管理，3 步上手。
type Server struct {
	net      ziface.IServer
	protocol znet.ConnProtocol
	addr     string
	name     string
	maxConn  int64

	router       map[int32]HandlerFunc
	onConnect    func(*Conn)
	onDisconnect func(*Conn)

	workerSize     int
	directDispatch bool
	tlsConfig      *ziface.TLSConfig
	pool           *ants.PoolWithFunc // 零闭包分配
	ctx            context.Context
	cancel         context.CancelFunc
	connCache      sync.Map // channelId → *Conn，实例级缓存
	stopOnce       sync.Once
}

// New 创建轻量服务器
func New(opts ...Option) *Server {
	s := &Server{
		protocol:   znet.TCP,
		addr:       ":9001",
		name:       "zynet",
		router:     make(map[int32]HandlerFunc),
		workerSize: runtime.NumCPU(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handle 注册消息路由（MsgID → HandlerFunc）
func (s *Server) Handle(msgId int32, handler HandlerFunc) {
	s.router[msgId] = handler
}

// OnConnect 设置连接建立回调
func (s *Server) OnConnect(fn func(conn *Conn)) {
	s.onConnect = fn
}

// OnDisconnect 设置连接断开回调
func (s *Server) OnDisconnect(fn func(conn *Conn)) {
	s.onDisconnect = fn
}

// Start 非阻塞启动服务器，配合 Stop 使用。
// 启动后可通过 Addr() 获取实际监听地址（适用于端口 :0 场景）。
func (s *Server) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if !s.directDispatch {
		size := s.workerSize
		if size <= 0 {
			size = runtime.NumCPU()
		}
		var err error
		s.pool, err = ants.NewPoolWithFunc(size, func(i interface{}) {
			req := i.(*Request)
			req.handler(req)
			putRequest(req)
		}, ants.WithPreAlloc(true))
		if err != nil {
			panic(fmt.Sprintf("server: create worker pool failed: %v", err))
		}
	}

	s.net = s.createServer(s.handleAccept, s.handleRead)
	if s.maxConn > 0 {
		s.net.SetMaxConnections(s.maxConn)
	}
	if s.tlsConfig != nil {
		s.net.SetTLSConfig(s.tlsConfig)
	}

	s.net.Server(s.ctx)

	mode := "worker pool"
	if s.directDispatch {
		mode = "direct dispatch"
	}
	fmt.Printf("[%s] server listening on %s (%s, %s)\n", s.name, s.addr, s.protocolName(), mode)
}

// Run 启动服务器（阻塞，直到收到 SIGINT/SIGTERM）
func (s *Server) Run() {
	s.Start()

	cm := zgrace.New()
	cm.Register(func() {
		fmt.Printf("[%s] shutting down...\n", s.name)
		s.Stop()
	})
	cm.Wait()
}

// Addr 返回实际监听地址（Start 后调用，适用于端口 :0 场景）。
func (s *Server) Addr() string {
	if s.net != nil {
		return s.net.GetAddr()
	}
	return s.addr
}

// Stop 优雅关闭（幂等，多次调用安全）
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.net != nil {
			s.net.Close()
		}
		if s.pool != nil {
			s.pool.Release()
		}
	})
}

// GetConn 按 ChannelID 查找连接
func (s *Server) GetConn(id uint64) *Conn {
	ch := s.net.GetChannel(id)
	if ch == nil {
		return nil
	}
	return s.getOrCreateConn(ch)
}

// GetConnByAuthId 按认证 ID 查找连接
func (s *Server) GetConnByAuthId(authId int64) *Conn {
	ch := s.net.GetChannelByAuthId(authId)
	if ch == nil {
		return nil
	}
	return s.getOrCreateConn(ch)
}

func (s *Server) handleAccept(ch ziface.IChannel) bool {
	conn := s.getOrCreateConn(ch)
	ch.SetCloseCall(func(c ziface.IChannel) {
		if s.onDisconnect != nil {
			s.onDisconnect(conn)
		}
		s.removeConn(c)
	})
	if s.onConnect != nil {
		s.onConnect(conn)
	}
	return true
}

func (s *Server) handleRead(ch ziface.IChannel, msg ziface.IWireMessage) {
	msgId := msg.GetMsgId()
	handler, ok := s.router[msgId]
	if !ok {
		return
	}

	conn := s.getOrCreateConn(ch)
	req := getRequest()
	req.conn = conn
	req.msgId = msgId
	req.seqId = msg.GetSeqId()
	req.data = append(req.data[:0], msg.GetMessageData()...)

	if s.directDispatch {
		handler(req)
		putRequest(req)
	} else {
		req.handler = handler
		if err := s.pool.Invoke(req); err != nil {
			putRequest(req)
		}
	}
}

func (s *Server) createServer(onAccept func(ziface.IChannel) bool, onRead func(ziface.IChannel, ziface.IWireMessage)) ziface.IServer {
	handlers := znet.ServerHandlers{OnAccept: onAccept, OnRead: onRead}
	switch s.protocol {
	case znet.WebSocket:
		return zws.NewServer(s.addr, handlers)
	case znet.KCP:
		return zkcp.NewServer(s.addr, handlers)
	default:
		return ztcp.NewServer(s.addr, handlers)
	}
}

func (s *Server) protocolName() string {
	switch s.protocol {
	case znet.WebSocket:
		return "WebSocket"
	case znet.KCP:
		return "KCP"
	default:
		return "TCP"
	}
}
