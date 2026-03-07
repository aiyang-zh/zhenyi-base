package znet

import (
	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
)

// heartbeatConfigurable 内部接口，用于设置 channel 的心跳超时
type heartbeatConfigurable interface {
	setHeartbeatTimeout(d time.Duration)
}

// ServerHandlers 服务器事件处理器，所有字段必须设置
// 将来扩展新的必须/可选回调只需在此添加字段，构造函数签名不变
type ServerHandlers struct {
	OnAccept func(ziface.IChannel) bool                 // 必须：连接建立时调用，返回 false 拒绝连接
	OnRead   func(ziface.IChannel, ziface.IWireMessage) // 必须：收到消息时调用（消息为线协议解析产物）
}

type BaseServer struct {
	idGen            uint64
	handlers         ServerHandlers
	channels         sync.Map // channelId → IChannel
	authChannels     sync.Map // authId → IChannel
	connCount        atomic.Int64
	maxConn          int64 // 0 = 不限
	addr             string
	iEncrypt         ziface.IEncrypt
	closeCh          chan struct{}
	listener         net.Listener
	Once             sync.Once
	iMetrics         ziface.IMetrics
	heartbeatTimeout time.Duration     // 心跳超时（0 = 禁用，默认 30s）
	tlsConfig        *ziface.TLSConfig // TLS 配置（nil = 不启用 TLS）
}

func NewBaseServer(addr string, handlers ServerHandlers) *BaseServer {
	if handlers.OnAccept == nil {
		panic("network: ServerHandlers.OnAccept must not be nil")
	}
	if handlers.OnRead == nil {
		panic("network: ServerHandlers.OnRead must not be nil")
	}
	s := &BaseServer{
		handlers:         handlers,
		addr:             addr,
		channels:         sync.Map{},
		closeCh:          make(chan struct{}, 1),
		Once:             sync.Once{},
		heartbeatTimeout: 30 * time.Second,
		iEncrypt:         zencrypt.NewBaseEncrypt(),
	}
	return s
}

// SetHeartbeatTimeout 配置心跳超时（基于 conn.SetReadDeadline，内核级超时）
// timeout <= 0 表示禁用心跳检测
func (b *BaseServer) SetHeartbeatTimeout(timeout time.Duration) {
	b.heartbeatTimeout = timeout
}

// SetTLSConfig 配置 TLS（支持标准 TLS 和国密 GM-TLS）。
// 必须在 Server() 启动前调用。nil 表示不启用 TLS。
func (b *BaseServer) SetTLSConfig(cfg *ziface.TLSConfig) {
	b.tlsConfig = cfg
}

// GetTLSConfig 获取当前 TLS 配置。
func (b *BaseServer) GetTLSConfig() *ziface.TLSConfig {
	return b.tlsConfig
}

func (b *BaseServer) GetListener() net.Listener {
	return b.listener
}

func (b *BaseServer) SetListener(l net.Listener) {
	b.listener = l
}

// GetAddr 返回实际监听地址。如果 listener 已绑定则返回实际地址（含 OS 分配端口），
// 否则返回配置地址。
func (b *BaseServer) GetAddr() string {
	if b.listener != nil {
		return b.listener.Addr().String()
	}
	return b.addr
}

func (b *BaseServer) SetMaxConnections(max int64) {
	b.maxConn = max
}
func (b *BaseServer) SetEncrypt(iEncrypt ziface.IEncrypt) {
	b.iEncrypt = iEncrypt
}
func (b *BaseServer) SetMetrics(iMetrics ziface.IMetrics) {
	b.iMetrics = iMetrics
}
func (b *BaseServer) AcceptAllowed() bool {
	if b.maxConn <= 0 {
		return true
	}
	return b.connCount.Load() < b.maxConn
}
func (b *BaseServer) GetClose() chan struct{} {
	return b.closeCh
}
func (b *BaseServer) OnceDo(f func()) {
	b.Once.Do(f)
}
func (b *BaseServer) AddChannel(channel ziface.IChannel) {
	if b.heartbeatTimeout > 0 {
		if hc, ok := channel.(heartbeatConfigurable); ok {
			hc.setHeartbeatTimeout(b.heartbeatTimeout)
		}
	}
	b.channels.Store(channel.GetChannelId(), channel)
	b.connCount.Add(1)
	if b.iMetrics != nil {
		b.iMetrics.ConnInc()
	}
}
func (b *BaseServer) GetEncrypt() ziface.IEncrypt {
	return b.iEncrypt
}

func (b *BaseServer) NextId() uint64 {
	return atomic.AddUint64(&b.idGen, 1)
}

func (b *BaseServer) HandleAccept(channel ziface.IChannel) bool {
	if !b.AcceptAllowed() {
		if b.iMetrics != nil {
			b.iMetrics.ConnRejectedInc()
		}
		return false
	}
	if b.handlers.OnAccept != nil {
		if b.handlers.OnAccept(channel) {
			if b.iMetrics != nil {
				b.iMetrics.ConnRejectedInc()
			}
			return true
		}
		return false
	}
	return false
}

func (b *BaseServer) HandleRead(channel ziface.IChannel, message ziface.IWireMessage) {
	if b.handlers.OnRead != nil {
		b.handlers.OnRead(channel, message)
	}
}

func (b *BaseServer) GetChannel(channelId uint64) ziface.IChannel {
	channel, ok := b.channels.Load(channelId)
	if !ok {
		return nil
	}
	ch, ok1 := channel.(ziface.IChannel)
	if !ok1 {
		return nil
	}
	return ch
}

// RemoveChannel 从 map 中移除 channel（私有方法）
// ⚠️ 注意：此方法不调用 channel.Close()，避免循环依赖
// channel.Close() 会自动调用此方法
func (b *BaseServer) RemoveChannel(channelId uint64) {
	ch, ok := b.channels.LoadAndDelete(channelId)
	if !ok {
		return
	}
	b.connCount.Add(-1)
	if b.iMetrics != nil {
		b.iMetrics.ConnDec()
	}
	channel, ok1 := ch.(ziface.IChannel)
	if !ok1 {
		return
	}

	authId := channel.GetAuthId()
	if authId > 0 {
		b.authChannels.Delete(authId)
	}
}

func (b *BaseServer) SetChannelAuth(channelId uint64, authId int64) {
	channel := b.GetChannel(channelId)
	if channel == nil {
		return
	}
	channel.SetAuthId(authId)
	b.authChannels.Store(authId, channel)
}

func (b *BaseServer) GetChannelByAuthId(authId int64) ziface.IChannel {
	ch, ok := b.authChannels.Load(authId)
	if !ok {
		return nil
	}
	channel, ok1 := ch.(ziface.IChannel)
	if !ok1 {
		return nil
	}
	return channel
}

func (b *BaseServer) BaseClose() {
	if b.listener == nil {
		return
	}
	if err := b.listener.Close(); err != nil {
		zlog.Error("Server listener close failed", zap.String("addr", b.addr), zap.Error(err))
	}
	b.channels.Range(func(key, value any) bool {
		channel, ok := value.(ziface.IChannel)
		if !ok {
			return true
		}
		channel.Close()
		return true
	})
}
