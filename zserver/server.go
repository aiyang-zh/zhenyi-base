package zserver

import (
	"context"
	_ "embed"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zbrand"
	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"github.com/aiyang-zh/zhenyi-base/zgrace"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zkcp"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zreactor"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"github.com/aiyang-zh/zhenyi-base/zws"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
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

	workerSize        int
	directDispatch    bool
	directDispatchRef bool            // 为 true 时 directDispatch 下 req.data 直接引用，不 copy；默认 false 会 copy
	mode              ziface.ConnMode // 默认 ModeSync（与 client 一致）；WithAsyncMode 为 ModeAsync
	heartbeatTimeout  *time.Duration  // nil=使用底层默认 30s；非 nil 则 Start 时设置
	showBanner        bool
	tlsConfig         *ziface.TLSConfig
	useReactor        bool               // 仅 TCP + Linux 时用 ztcp.ServerReactor
	reactorMetrics    *zreactor.Metrics  // 仅 useReactor 时生效，可选
	pool              *ants.PoolWithFunc // 零闭包分配
	ctx               context.Context
	cancel            context.CancelFunc
	connCache         *zcoll.SyncMap[uint64, *Conn] // channelId → *Conn，实例级缓存
	stopOnce          sync.Once
}

// New 创建轻量服务器
func New(opts ...Option) *Server {
	s := &Server{
		protocol:   znet.TCP,
		addr:       ":9001",
		name:       "zhenyi",
		router:     make(map[int32]HandlerFunc),
		workerSize: runtime.NumCPU(),
		mode:       ziface.ModeSync,
		showBanner: true,
		connCache:  zcoll.NewSyncMap[uint64, *Conn](),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.ctx == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
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

// SetReactorMetrics 设置 reactor 模式的监控回调；仅在使用 WithReactorMode 且协议为 TCP 时生效，传 nil 表示不埋点。
func (s *Server) SetReactorMetrics(m *zreactor.Metrics) {
	s.reactorMetrics = m
}

// Start 非阻塞启动服务器，配合 Stop 使用。
// 启动后可通过 Addr() 获取实际监听地址（适用于端口 :0 场景）。
func (s *Server) Start() {
	// sync 模式必须 directDispatch（handler 在读协程内才能 ReplyImmediate）
	if s.mode == ziface.ModeSync {
		s.directDispatch = true
	}
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
	if s.heartbeatTimeout != nil {
		s.net.SetHeartbeatTimeout(*s.heartbeatTimeout)
	}

	// reactor 模式：仅 TCP、且未启用 TLS、且 Linux 时，用 ztcp.ServerReactor 阻塞运行；非 Linux 回退为普通 Server
	if s.useReactor && s.protocol == znet.TCP && s.tlsConfig == nil && runtime.GOOS == "linux" {
		if tcpServer, ok := s.net.(*ztcp.Server); ok {
			if s.reactorMetrics != nil {
				tcpServer.SetReactorMetrics(s.reactorMetrics)
			}
			tcpServer.ServerReactor(s.ctx)
			return
		}
	}

	s.net.Server(s.ctx)

	mode := "worker pool"
	if s.directDispatch {
		mode = "direct dispatch"
	}
	if s.showBanner {
		s.printBanner(mode)
	}
	fmt.Printf("[%s] server listening on %s (%s, %s)\n", s.name, s.addr, s.protocolName(), mode)
}

// Run 启动服务器（阻塞，直到收到 SIGINT/SIGTERM）。
// 若使用了 WithReactorMode 且协议为 TCP，Start 会在 reactor 循环内阻塞；Run 在子 goroutine 中执行 Start，主 goroutine 等信号后 Stop 并等待 Start 退出。
func (s *Server) Run() {
	if s.useReactor && s.protocol == znet.TCP && s.tlsConfig == nil && runtime.GOOS == "linux" {
		cm := zgrace.New()
		cm.Register(func(ctx context.Context) {
			_ = ctx
			fmt.Printf("[%s] shutting down...\n", s.name)
			s.Stop()
		})
		var startDone sync.WaitGroup
		var startPanic interface{}
		startDone.Add(1)
		go func() {
			defer func() {
				startPanic = recover()
				startDone.Done()
			}()
			s.Start() // 阻塞在 ztcp.ServerReactor 直到 ctx 取消
		}()
		cm.Wait() // 主 goroutine 等信号，触发 Stop → cancel → reactor 退出
		startDone.Wait()
		if startPanic != nil {
			panic(startPanic)
		}
		return
	}
	s.Start()
	cm := zgrace.New()
	cm.Register(func(ctx context.Context) {
		_ = ctx
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
func (s *Server) GetConnByAuthId(authId uint64) *Conn {
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
		zlog.Warn("unknown msgId, dropped",
			zap.Int32("msgId", msgId),
			zap.Uint64("channelId", ch.GetChannelId()))
		return
	}

	conn := s.getOrCreateConn(ch)
	req := getRequest()
	req.conn = conn
	req.msgId = msgId
	req.seqId = msg.GetSeqId()
	if s.directDispatch && s.directDispatchRef {
		req.data = msg.GetMessageData()
	} else {
		req.data = append(req.data[:0], msg.GetMessageData()...)
	}

	if s.directDispatch {
		handler(req)
		putRequest(req)
	} else {
		req.handler = handler
		if err := s.pool.Invoke(req); err != nil {
			zlog.Warn("pool.Invoke failed, request dropped",
				zap.Int32("msgId", msgId),
				zap.Uint64("channelId", ch.GetChannelId()),
				zap.Error(err))
			putRequest(req)
		}
	}
}

func (s *Server) createServer(onAccept func(ziface.IChannel) bool, onRead func(ziface.IChannel, ziface.IWireMessage)) ziface.IServer {
	handlers := znet.ServerHandlers{OnAccept: onAccept, OnRead: onRead, SyncMode: s.mode == ziface.ModeSync}
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

func (s *Server) printBanner(mode string) {
	fmt.Print(zbrand.Banner)
	fmt.Printf("  [zhenyi-base] %s | %s | %s\n", s.name, s.protocolName(), mode)
	fmt.Printf("  [Github] https://github.com/aiyang-zh/zhenyi-base\n\n")
}
