package znet

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/ziface"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
)

// ServerHandlers 服务器事件处理器，所有字段必须设置
type ServerHandlers struct {
	OnAccept func(ziface.IChannel) bool                 // 必须：连接建立时调用，返回 false 拒绝连接
	OnRead   func(ziface.IChannel, ziface.IWireMessage) // 必须：收到消息时调用（消息为线协议解析产物）
	SyncMode bool                                       // 可选：sync 模式，原生支持；不建发送队列，handler 用 ReplyImmediate 直写
}

// listenerHolder 供 atomic.Pointer 存储，避免 interface 的 atomic.Value 分配；嵌入 BaseServer 实现 0 分配。
type listenerHolder struct{ L net.Listener }

// BaseServer 是网络层通用服务端基类，负责连接管理、TLS、心跳与认证映射。
// 具体协议（TCP/WebSocket/KCP）由子包嵌入并实现 Server(ctx) 与 listen。
type BaseServer struct {
	idGen           uint64
	handlers        ServerHandlers
	channels        *zcoll.SyncMap[uint64, ziface.IChannel] // channelId → IChannel
	authChannels    *zcoll.SyncMap[uint64, ziface.IChannel] // authId → IChannel
	connCount       atomic.Int64
	maxConn         int64 // 0 = 不限
	addr            string
	iEncrypt        ziface.IEncrypt
	closeCh         chan struct{}
	listener        atomic.Pointer[listenerHolder] // 无锁：Set/Get/Close 通过 atomic 协调
	listenerH       listenerHolder                 // 嵌入，SetListener 仅写字段后 Store(&listenerH)，0 分配
	Once            sync.Once
	iMetrics        ziface.IMetrics
	iChannelMetrics ziface.IChannelMetrics // 单连接维度指标，AddChannel 时注入到支持 IChannelMetricsSetter 的 channel
	// sessionStatsFactory 非 nil 时，NewBaseChannel 为每条连接创建独立 ISessionStats（如业务层会话计数）。
	sessionStatsFactory  func() ziface.ISessionStats
	heartbeatTimeout     time.Duration     // 心跳超时（0 = 禁用，默认 30s）
	tlsConfig            *ziface.TLSConfig // TLS 配置（nil = 不启用 TLS）
	sharedSendWorkerMode bool
	sharedSendWorkersMu  sync.Mutex
	sharedSendWorkers    *sharedSendWorkers
}

// NewBaseServer 创建网络层服务端基类。
// addr 为监听地址，handlers 必须同时提供 OnAccept 与 OnRead。
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
		channels:         zcoll.NewSyncMap[uint64, ziface.IChannel](),
		authChannels:     zcoll.NewSyncMap[uint64, ziface.IChannel](),
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

// GetListener 返回当前使用的 net.Listener（启动后由具体协议设置）。无锁。
func (b *BaseServer) GetListener() net.Listener {
	p := b.listener.Load()
	if p == nil {
		return nil
	}
	return p.L
}

// SetListener 设置或替换底层 listener（供 TCP/WebSocket/KCP 在 start 时注入）。无锁、0 分配。
func (b *BaseServer) SetListener(l net.Listener) {
	b.listenerH.L = l
	b.listener.Store(&b.listenerH)
}

// GetAddr 返回实际监听地址。如果 listener 已绑定则返回实际地址（含 OS 分配端口），否则返回配置地址。无锁。
func (b *BaseServer) GetAddr() string {
	p := b.listener.Load()
	if p == nil || p.L == nil {
		return b.addr
	}
	return p.L.Addr().String()
}

// SetMaxConnections 设置最大连接数；0 表示不限制。
func (b *BaseServer) SetMaxConnections(max int64) {
	b.maxConn = max
}

// SetEncrypt 设置加解密实现，nil 表示不加密。
func (b *BaseServer) SetEncrypt(iEncrypt ziface.IEncrypt) {
	b.iEncrypt = iEncrypt
}

// SetMetrics 设置服务级连接指标收集器（可选）。ConnInc/ConnDec/ConnRejectedInc。
func (b *BaseServer) SetMetrics(iMetrics ziface.IMetrics) {
	b.iMetrics = iMetrics
}

// SetChannelMetrics 设置单连接维度指标收集器（可选）。AddChannel 时会注入到实现了 IChannelMetricsSetter 的 channel（如 *znet.BaseChannel）。
func (b *BaseServer) SetChannelMetrics(m ziface.IChannelMetrics) {
	b.iChannelMetrics = m
}

// SetSessionStatsFactory 设置每条新连接使用的会话统计工厂（可选）。在 NewBaseChannel 内调用一次 factory，注入到 BaseChannel.stats。
// 传 nil 表示不采集（RecordSend/RecordRec 路径保持 no-op）。
func (b *BaseServer) SetSessionStatsFactory(f func() ziface.ISessionStats) {
	b.sessionStatsFactory = f
}

// GetSessionStatsFactory 返回当前工厂，供 NewBaseChannel 探测；未设置时为 nil。
func (b *BaseServer) GetSessionStatsFactory() func() ziface.ISessionStats {
	return b.sessionStatsFactory
}

// SyncMode 返回是否使用 sync 模式（原生支持：无发送队列，ReplyImmediate 直写）
func (b *BaseServer) SyncMode() bool {
	return b.handlers.SyncMode
}

// SetSharedSendWorkerMode 设置是否启用共享写 worker 模式（默认 false）。
// 启用后（且非 SyncMode），连接发送改为共享 worker 批量 flush，而不是每连接 runSend goroutine。
func (b *BaseServer) SetSharedSendWorkerMode(enabled bool) {
	b.sharedSendWorkerMode = enabled
}

// SharedSendWorkerMode 返回当前共享写 worker 开关状态。
func (b *BaseServer) SharedSendWorkerMode() bool {
	return b.sharedSendWorkerMode
}

// ChannelRunner 启动 channel 需实现此接口（StartSend + Start）
type ChannelRunner interface {
	StartSend(ctx context.Context)
	Start()
}

type sharedSendHookSetter interface {
	SetSharedSendHook(func(*BaseChannel))
}

func (b *BaseServer) ensureSharedSendWorkers(ctx context.Context) *sharedSendWorkers {
	b.sharedSendWorkersMu.Lock()
	defer b.sharedSendWorkersMu.Unlock()
	if b.sharedSendWorkers == nil {
		b.sharedSendWorkers = newSharedSendWorkers(ctx)
	}
	return b.sharedSendWorkers
}

// BindSharedSendHook 尝试为 channel 绑定共享写 worker。
// 返回 true 表示已绑定；返回 false 表示未启用或 channel 不支持该能力。
func (b *BaseServer) BindSharedSendHook(ctx context.Context, ch any) bool {
	if !b.sharedSendWorkerMode {
		return false
	}
	setter, ok := ch.(sharedSendHookSetter)
	if !ok {
		return false
	}
	setter.SetSharedSendHook(b.ensureSharedSendWorkers(ctx).enqueue)
	return true
}

// RunChannel 启动 channel 读写循环。SyncMode 时无 runSend（无发送队列）。
// ztcp/zws/zkcp 统一调用此方法，避免各协议重复判断。
func (b *BaseServer) RunChannel(ctx context.Context, ch ChannelRunner) {
	if !b.SyncMode() {
		if !b.BindSharedSendHook(ctx, ch) {
			ch.StartSend(ctx)
		}
	}
	ch.Start()
}

// AcceptAllowed 返回当前是否允许接受新连接（未超 maxConn 时为 true）。
func (b *BaseServer) AcceptAllowed() bool {
	if b.maxConn <= 0 {
		return true
	}
	return b.connCount.Load() < b.maxConn
}

// GetClose 返回关闭信号通道，收到信号后具体协议应停止 Accept 并调用 Close。
func (b *BaseServer) GetClose() chan struct{} {
	return b.closeCh
}

// OnceDo 执行一次给定的函数（用于关闭时只执行一次的逻辑）。
func (b *BaseServer) OnceDo(f func()) {
	b.Once.Do(f)
}

// AddChannel 将新连接加入管理并递增连接计数；会应用心跳超时与指标。
// heartbeatTimeout 由 SetHeartbeatTimeout 设置；0 表示禁用（覆盖 channel 默认 30s）。
// 若已 SetChannelMetrics，会对实现了 IChannelMetricsSetter 的 channel 注入单连接指标。
func (b *BaseServer) AddChannel(channel ziface.IChannel) {
	channel.SetHeartbeatTimeout(b.heartbeatTimeout)
	if b.iChannelMetrics != nil {
		if setter, ok := channel.(ziface.IChannelMetricsSetter); ok {
			setter.SetChannelMetrics(b.iChannelMetrics)
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

// NextId 生成并返回下一个全局唯一的 ChannelID。
func (b *BaseServer) NextId() uint64 {
	return atomic.AddUint64(&b.idGen, 1)
}

// HandleAccept 在 Accept 后调用，执行 OnAccept 并检查 maxConn；返回 false 表示拒绝连接。
func (b *BaseServer) HandleAccept(channel ziface.IChannel) bool {
	if !b.AcceptAllowed() {
		if b.iMetrics != nil {
			b.iMetrics.ConnRejectedInc()
		}
		return false
	}
	if b.handlers.OnAccept != nil {
		if b.handlers.OnAccept(channel) {
			return true
		}
		if b.iMetrics != nil {
			b.iMetrics.ConnRejectedInc()
		}
		return false
	}
	return false
}

// HandleRead 将收到的消息分发给 OnRead 回调。
func (b *BaseServer) HandleRead(channel ziface.IChannel, message ziface.IWireMessage) {
	if b.handlers.OnRead != nil {
		b.handlers.OnRead(channel, message)
	}
}

// GetChannel 根据 ChannelID 返回对应连接，不存在则返回 nil。
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

// RemoveChannel 从连接表中移除指定 Channel（由 channel.Close() 内部调用，业务勿直接调用）。
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

// SetChannelAuth 将指定 Channel 绑定到业务侧认证 ID，便于通过 GetChannelByAuthId 查询。
func (b *BaseServer) SetChannelAuth(channelId uint64, authId uint64) {
	channel := b.GetChannel(channelId)
	if channel == nil {
		return
	}
	channel.SetAuthId(authId)
	b.authChannels.Store(authId, channel)
}

// GetChannelByAuthId 根据业务侧认证 ID 查找对应连接，不存在则返回 nil。
func (b *BaseServer) GetChannelByAuthId(authId uint64) ziface.IChannel {
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

// AggregateChannelSessionStats 遍历当前连接，累加实现了 ISessionStatsSnapshot 的会话统计（如 zmonitor.SessionStats）。
// channelsWithStats 为成功导出快照的连接数；未配置工厂或自定义 ISessionStats 未实现快照接口的连接会跳过。
func (b *BaseServer) AggregateChannelSessionStats() (channelsWithStats int, sumSendCount, sumRecvCount, sumSendBytes, sumRecvBytes int64) {
	b.channels.Range(func(_ uint64, ch ziface.IChannel) bool {
		snap, ok := ch.(ziface.IChannelSessionStatsSnapshot)
		if !ok {
			return true
		}
		sc, rc, sb, rb, _, _, ok := snap.SessionStatsSnapshot()
		if !ok {
			return true
		}
		channelsWithStats++
		sumSendCount += sc
		sumRecvCount += rc
		sumSendBytes += sb
		sumRecvBytes += rb
		return true
	})
	return
}

// BaseClose 关闭 listener 并关闭所有已管理的 Channel（由具体协议的 Close 调用）。无锁。
func (b *BaseServer) BaseClose() {
	p := b.listener.Swap(nil)
	if p == nil || p.L == nil {
		return
	}
	if err := p.L.Close(); err != nil {
		zlog.Error("Server listener close failed", zap.String("addr", b.addr), zap.Error(err))
	}
	b.channels.Range(func(key uint64, value ziface.IChannel) bool {
		value.Close()
		return true
	})
}
