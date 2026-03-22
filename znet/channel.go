package znet

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zqueue"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zbackoff"
	"github.com/aiyang-zh/zhenyi-base/zbatch"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi-base/ztime"
)

// 接口断言：确保 BaseChannel 实现 IChannel 与 IChannelMetricsSetter
var _ ziface.IChannel = (*BaseChannel)(nil)
var _ ziface.IChannelMetricsSetter = (*BaseChannel)(nil)

// ================================================================
// BaseChannel - 零拷贝通道
// ================================================================
//
// 使用 Ring Buffer 实现零拷贝网络读取
// 减少内存分配和 GC 压力
//
// 特性：
//   - 读取时不再每次分配 headBuffer/dataBuffer
//   - 数据直接从 Ring Buffer 零拷贝读取
//   - 批量从内核读取数据，减少系统调用
//   - 发送使用 writev 系统调用，减少内存拷贝

const MaxBatchLimit = 200

// BaseChannel 基础通道（零拷贝实现 + 会话管理）
// ✅ 已合并 Session 职责，减少循环依赖和双重管理
type BaseChannel struct {
	// ========== 8 字节字段 ==========
	channelId        uint64
	rpcId            uint64 // RPC ID（原子递增）
	authId           uint64 // 认证 ID（用户 ID）
	lastRecTime      int64  // 最后接收时间（毫秒）
	heartbeatTimeout int64  // 心跳超时时间（毫秒），0 = 使用默认 30s
	msgCount         int64  // ✅ 队列长度（仅用于监控，定期更新）

	// ========== 指针字段 ==========
	server       ziface.IServer
	conn         net.Conn
	state        *atomic.Bool
	rate         ziface.ILimit                          // 限流器
	mailBoxQueue *zqueue.UnboundedMPSC[ziface.IMessage] // 发送队列
	closeCall    func(ziface.IChannel)                  // 关闭回调（框架层 + 业务层）
	metrics      ziface.IChannelMetrics
	stats        ziface.ISessionStats
	batcher      *zbatch.FastAdaptiveBatcher // ✅ 极速自适应批量处理器

	// 零拷贝相关
	readBuffer   *RingBuffer // 读取缓冲区
	socketParser *BaseSocket // 复用 BaseSocket（实现 ISocketV2）
	parseData    ParseData   // read 协程复用，避免每条消息 pool Get/Put
	parseMsg     NetMessage  // read 协程复用的消息对象

	// ========== 发送复用 ==========
	headersBuf []byte      // runSend 单协程复用，避免每次从 bytespool Get/Put
	writeBufs  net.Buffers // runSend 单协程复用，避免每次池化 net.Buffers

	// ========== 复杂类型 ==========
	addr      string
	closeChan chan struct {
	} // 关闭信号
	sendDone  sync.WaitGroup // 等待 runSend 退出
	closeOnce sync.Once      // 确保只关闭一次
	isClose   atomic.Bool    // 是否已关闭
}

// NewBaseChannel 创建基础通道（已合并 Session 职责）
// 两种原生模式：async（默认，有发送队列+runSend） / sync（无队列，ReplyImmediate 直写）
func NewBaseChannel(channelId uint64, conn net.Conn, server ziface.IServer) *BaseChannel {
	state := &atomic.Bool{}
	state.Store(true)

	syncMode := server.SyncMode()

	c := &BaseChannel{
		// 网络层
		channelId:    channelId,
		server:       server,
		addr:         conn.RemoteAddr().String(),
		conn:         conn,
		state:        state,
		readBuffer:   GetRingBuffer(),
		socketParser: NewBaseSocket(),

		// 会话层（原 Session 字段）
		lastRecTime:      ztime.ServerNowUnixMilli(),
		heartbeatTimeout: 30 * time.Second.Milliseconds(), // 默认 30 秒
		closeChan:        make(chan struct{}, 1),

		// ✅ 极速自适应批量处理器（网络场景配置）
		batcher: zbatch.NewFastAdaptiveBatcher(
			1,                  // minBatch: 最小批次 1
			200,                // maxBatch: 最大批次 200
			5*time.Millisecond, // targetMeanLatency：与窗口平均延迟比较的控制目标
		),
	}

	// async 模式：创建发送队列；sync 模式：无队列，ReplyImmediate 直写
	if !syncMode {
		c.mailBoxQueue = zqueue.NewUnboundedMPSC[ziface.IMessage]()
	}

	// ✅ 初始化复用的 ParseData（避免每条消息 pool Get/Put）
	c.parseData = ParseData{
		Message:      &c.parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	// 设置 TCP 缓冲区（适度大小，避免 1k1k 等场景下 ENOBUFS / no buffer space available）
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		const tcpBufSize = 64 * 1024 // 64KB，多连接时降低内核缓冲占用
		_ = tcpConn.SetWriteBuffer(tcpBufSize)
		_ = tcpConn.SetReadBuffer(tcpBufSize)
		_ = tcpConn.SetNoDelay(true) // 禁用 Nagle 算法，减少延迟
	}

	return c
}

// IsOpen 检查通道是否打开
func (c *BaseChannel) IsOpen() bool {
	return c.state.Load()
}

// read 零拷贝读取（核心优化点）。返回 true 表示应退出读循环（错误或断开）。
func (c *BaseChannel) read() bool {
	// 1. 从网络读取数据到 Ring Buffer
	nRead, err := c.readBuffer.WriteFromReader(c.conn, 0)
	if nRead > 0 {
		if c.metrics != nil {
			c.metrics.BytesRecAdd(int64(nRead))
		}
		c.resetReadDeadline()
	}
	if err != nil {
		if err == ErrBufferFull {
			// 尝试扩容，避免连接卡死（RingBuffer 支持 Grow 且有 MaxSize 限制，调用安全）
			if c.readBuffer.Grow(65536) {
				var n2 int
				n2, err = c.readBuffer.WriteFromReader(c.conn, 0)
				if n2 > 0 {
					if c.metrics != nil {
						c.metrics.BytesRecAdd(int64(n2))
					}
					c.resetReadDeadline()
				}
				if err != nil && err != ErrBufferFull {
					// 重试时出现其他错误，交给下方统一处理
				} else {
					err = nil // 成功或仍满则进入解析循环消费以腾出空间，不断开连接
					if n2 == 0 && c.readBuffer.IsFull() {
						zlog.Warn("Read buffer full after grow, draining",
							zap.Uint64("channelId", c.channelId),
							zap.Int("bufLen", c.readBuffer.Len()))
					}
				}
			} else {
				zlog.Warn("Read buffer full (at MaxSize), draining",
					zap.Uint64("channelId", c.channelId),
					zap.Int("bufLen", c.readBuffer.Len()))
				err = nil // 已达上限则仅靠解析循环消费，不断开
			}
		}
		if err != nil {
			if err == io.EOF {
				return true
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if c.metrics != nil {
					c.metrics.ConnHeartbeatTimeoutInc()
				}
				zlog.Warn("Connection heartbeat timeout, closing",
					zap.Uint64("channelId", c.channelId),
					zap.Uint64("authId", c.GetAuthId()),
					zap.String("addr", c.addr))
				return true
			} else if !c.isNormalCloseError(err) {
				if c.metrics != nil {
					c.metrics.ConnErrorsInc()
				}
				zlog.Warn("Read error",
					zap.Uint64("channelId", c.channelId),
					zap.Error(err))
				return true
			} else {
				return true
			}
		}
	}

	// 2. 循环解析所有完整消息
	for {
		c.parseData.ResetForReuse()
		parsed, parseErr := c.socketParser.ParseFromRingBuffer(c.readBuffer, &c.parseData)
		if parseErr != nil {
			if c.metrics != nil {
				c.metrics.ConnErrorsInc()
			}
			zlog.Warn("ParseSocket error",
				zap.Uint64("channelId", c.channelId),
				zap.Error(parseErr))
			return true
		}
		if !parsed {
			if c.readBuffer.IsFull() {
				zlog.Error("Single packet exceeds buffer capacity, disconnecting",
					zap.Uint64("channelId", c.channelId),
					zap.Int("bufferCap", c.readBuffer.Cap()),
					zap.Int("bufferLen", c.readBuffer.Len()))
				return true
			}
			break
		}
		c.handleParsedMessage()
	}

	return false
}

// WriteToReadBuffer 将数据追加到读缓冲，供 reactor 等外部驱动在可读时写入后调用。
func (c *BaseChannel) WriteToReadBuffer(p []byte) (n int, err error) {
	return c.readBuffer.Write(p)
}

// ParseAndDispatch 从读缓冲解析并分发所有完整消息；供 reactor 在 WriteToReadBuffer 后调用。
// 返回 true 表示应关闭连接（解析错误或单包超缓冲容量）。
func (c *BaseChannel) ParseAndDispatch() bool {
	for {
		c.parseData.ResetForReuse()
		parsed, parseErr := c.socketParser.ParseFromRingBuffer(c.readBuffer, &c.parseData)
		if parseErr != nil {
			if c.metrics != nil {
				c.metrics.ConnErrorsInc()
			}
			zlog.Warn("ParseSocket error",
				zap.Uint64("channelId", c.channelId),
				zap.Error(parseErr))
			return true
		}
		if !parsed {
			if c.readBuffer.IsFull() {
				zlog.Error("Single packet exceeds buffer capacity, disconnecting",
					zap.Uint64("channelId", c.channelId),
					zap.Int("bufferCap", c.readBuffer.Cap()),
					zap.Int("bufferLen", c.readBuffer.Len()))
				return true
			}
			return false
		}
		c.handleParsedMessage()
	}
}

// handleParsedMessage 解密并分派已解析的消息（提取公共逻辑）
func (c *BaseChannel) handleParsedMessage() {
	netMsg := c.parseData.Message
	if encryptedData := netMsg.GetMessageData(); len(encryptedData) > 0 {
		decryptedData, err1 := c.server.GetEncrypt().Decrypt(encryptedData)
		if err1 != nil {
			zlog.Error("Decrypt error",
				zap.Uint64("channelId", c.channelId),
				zap.Int32("msgId", netMsg.GetMsgId()),
				zap.Error(err1))
			return
		}
		netMsg.SetMessageData(decryptedData)
	}
	c.server.HandleRead(c, netMsg)
}

// isNormalCloseError 判断是否是正常的连接关闭错误（纯判断，无副作用）
func (c *BaseChannel) isNormalCloseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "use of closed network connection") ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "forcibly closed by the remote host")
}

// Start 开启读写业务
func (c *BaseChannel) Start() {
	defer func() {
		c.Close()

		// 释放 parseData 持有的 pool buffer（跨边界拷贝的残留）
		for _, buf := range c.parseData.OwnedBuffers {
			buf.Release()
		}
		c.parseData.OwnedBuffers = c.parseData.OwnedBuffers[:0]

		if c.readBuffer != nil {
			PutRingBuffer(c.readBuffer)
			c.readBuffer = nil
		}
	}()

	// 设置初始读超时（心跳检测）
	c.resetReadDeadline()

	for {
		if c.read() {
			return
		}
	}
}

// Flush 刷新缓冲区
// Flush 刷新写缓冲；零拷贝模式下无应用层缓冲，直接返回 nil。
func (c *BaseChannel) Flush() error {
	return nil
}

// GetWriterTier 获取 writer 当前等级
// GetWriterTier 返回写缓冲层级；零拷贝模式下为 TierNone。
func (c *BaseChannel) GetWriterTier() ziface.BufferTier {
	return ziface.TierNone
}

// GetBuffered 获取缓冲区中已写入但未刷新的字节数
// GetBuffered 返回当前未刷写的字节数；零拷贝模式下为 0。
func (c *BaseChannel) GetBuffered() int {
	return 0
}

// SendBatchMsg 批量发送消息（实现 IChannel 接口）
// SendBatchMsg 批量发送消息（零拷贝 writev）；调用方不应再使用 messages 或 Release。
func (c *BaseChannel) SendBatchMsg(messages []ziface.IMessage) {
	if messages == nil || len(messages) == 0 {
		return
	}
	// 连接已关闭则不再发送，避免对已关闭连接做加密/写（runSend 可能与 Close 并发）
	if !c.IsOpen() {
		return
	}

	hdrLen := c.socketParser.HeaderLen()
	headersSize := len(messages) * hdrLen
	if cap(c.headersBuf) >= headersSize {
		c.headersBuf = c.headersBuf[:headersSize]
	} else {
		c.headersBuf = make([]byte, headersSize)
	}
	headers := c.headersBuf

	// ✅ 复用 Channel 上预分配的 writeBufs（runSend 单协程，无并发）
	c.writeBufs = c.writeBufs[:0]

	// 复用消息包装器
	wrapper := GetNetMessage()
	defer wrapper.Release()

	for i, msg := range messages {
		// 加密
		data, err := c.server.GetEncrypt().Encrypt(msg.GetMessageData())
		if err != nil {
			// ⚠️ 加密失败跳过该消息（不会发送脏数据）
			// 注意：如果协议要求序列号连续，客户端可能会认为丢包
			// 建议：加密失败通常是严重错误，考虑断开连接
			zlog.Error("SendBatchMsg Encrypt error",
				zap.Uint64("channelId", c.channelId),
				zap.Int32("msgId", msg.GetMsgId()),
				zap.Uint32("seqId", msg.GetSeqId()),
				zap.Error(err))
			continue
		}

		// 设置消息
		wrapper.Reset()
		wrapper.SetMsgId(msg.GetMsgId())
		wrapper.SetSeqId(msg.GetSeqId())
		wrapper.SetMessageData(data)

		offset := i * hdrLen
		headerLen, body := c.socketParser.PreparePacket(wrapper, headers[offset:offset+hdrLen])

		c.writeBufs = append(c.writeBufs, headers[offset:offset+headerLen])
		if len(body) > 0 {
			c.writeBufs = append(c.writeBufs, body)
		}
	}

	if err := c.writeBuffers(c.writeBufs); err != nil {
		var netErr net.Error
		if zerrs.As(err, &netErr) && netErr.Timeout() {
			return
		}
		if !c.isNormalCloseError(err) {
			zlog.Warn("SendBatchMsg Write error",
				zap.Uint64("channelId", c.channelId),
				zap.Error(err))
		}
	}
}

// WriteImmediate 读协程内同步直写，sync 场景原生支持，直接写出
func (c *BaseChannel) WriteImmediate(msg ziface.IWireMessage) error {
	if !c.IsOpen() {
		return zerrs.New(zerrs.ErrTypeNetwork, "channel not open")
	}
	var header [13]byte
	hdrLen, body := c.socketParser.PreparePacketFromWire(msg, header[:])
	var bufs net.Buffers
	if len(body) > 0 {
		bufs = net.Buffers{header[:hdrLen], body}
	} else {
		bufs = net.Buffers{header[:hdrLen]}
	}
	return c.writeBuffers(bufs)
}

// writeBuffers 使用 net.Buffers 批量写入 (writev 系统调用)
func (c *BaseChannel) writeBuffers(buffers net.Buffers) error {
	if !c.IsOpen() {
		return zerrs.New(zerrs.ErrTypeNetwork, "channel not open")
	}

	// ✅ 直接写到 conn，触发 writev 系统调用
	nWritten, err := buffers.WriteTo(c.conn)
	if nWritten > 0 && c.metrics != nil {
		c.metrics.BytesSentAdd(nWritten)
	}
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to write buffers to connection")
	}
	return nil
}

// Close 关闭通道（优雅关闭：等待发送协程退出并排空已入队发送）。
//
// 关闭顺序要点（与 Send 的竞态语义）：
//   - 先从 Server 摘除 channel，再 StopEnqueue，使后续 TryEnqueue 失败；
//   - 再置 state、关闭 closeChan、关闭连接，等待 runSend 退出后回收队列。
//
// 因此与 Close 并发的 Send 可能入队失败：此时 Send 会对消息调用 Release，业务不得假定「Send 返回即已发出」。
func (c *BaseChannel) Close() {
	if c.isClose.Load() {
		return
	}

	c.closeOnce.Do(func() {
		c.isClose.Store(true)

		// Step 0: 从 Server 的 map 中移除，阻止新消息路由到此 channel
		c.server.RemoveChannel(c.channelId)

		// Step 0.5: 必须在通知 runSend 退出之前禁止入队，否则 Send 在 state 检查与入队之间存在 TOCTOU：
		// runSend 已退出后仍可能 Enqueue 成功，导致消息永不出队、回调永不触发（见 UnboundedMPSC.StopEnqueue 注释）。
		if c.mailBoxQueue != nil {
			c.mailBoxQueue.StopEnqueue()
		}

		// Step 1: 标记不可写 + 通知 runSend 退出
		c.state.Store(false)
		close(c.closeChan)

		// Step 2: 关闭底层连接，强制 runSend 中阻塞的 WriteTo 立即返回错误
		// 这保证 sendDone.Wait() 不会无限挂起（消除 goroutine 泄漏）
		if err := c.conn.Close(); err != nil {
			// 仅记录非正常关闭错误
			errMsg := err.Error()
			if !strings.Contains(errMsg, "use of closed network connection") {
				zlog.Warn("Close connection error",
					zap.Uint64("channelId", c.channelId),
					zap.Error(err))
			}
		}

		// Step 3: 等待 runSend 完全退出（conn 已关闭，保证快速返回）
		c.sendDone.Wait()

		// Step 4: 执行关闭回调（框架层 + 业务层）
		if c.closeCall != nil {
			c.closeCall(c)
		}

		// Step 5: 关闭发送队列并清理残留消息
		if c.mailBoxQueue != nil {
			c.mailBoxQueue.Close()
		}
	})
}

// GetChannelId 获取通道 ID
func (c *BaseChannel) GetChannelId() uint64 {
	return c.channelId
}

// GetAddr 获取远程地址
func (c *BaseChannel) GetAddr() string {
	return c.addr
}

// GetReadBufferStats 获取读缓冲区统计（用于监控）
func (c *BaseChannel) GetReadBufferStats() (len, cap int, totalRead, totalWrite int64) {
	if c.readBuffer == nil {
		return
	}
	len = c.readBuffer.Len()
	cap = c.readBuffer.Cap()
	totalRead, totalWrite = c.readBuffer.Stats()
	return
}

// ================================================================
// 会话层方法（原 Session 职责）
// ================================================================

// GetAuthId 获取认证 ID
func (c *BaseChannel) GetAuthId() uint64 {
	return atomic.LoadUint64(&c.authId)
}

// SetAuthId 设置认证 ID
func (c *BaseChannel) SetAuthId(authId uint64) {
	atomic.SwapUint64(&c.authId, authId)
}

// GetRpcId 获取并递增 RPC ID
func (c *BaseChannel) GetRpcId() uint64 {
	return atomic.AddUint64(&c.rpcId, 1)
}

// SetLimit 设置限流器
func (c *BaseChannel) SetLimit(rate ziface.ILimit) {
	c.rate = rate
}

// SetChannelMetrics 注入单连接维度指标收集器（实现 ziface.IChannelMetricsSetter）。由 BaseServer.AddChannel 在 SetChannelMetrics 后自动调用。
func (c *BaseChannel) SetChannelMetrics(m ziface.IChannelMetrics) {
	c.metrics = m
}

// Allow 检查是否允许通过（限流检查）
func (c *BaseChannel) Allow() bool {
	if c.rate == nil {
		return true
	}
	return c.rate.Allow()
}

// UpdateLastRecTime 更新最后接收时间（同时刷新读超时）
func (c *BaseChannel) UpdateLastRecTime() {
	atomic.StoreInt64(&c.lastRecTime, ztime.ServerNowUnixMilli())
	c.resetReadDeadline()
}

// Check 检查是否心跳超时（基于 lastRecTime）
func (c *BaseChannel) Check() bool {
	timeout := atomic.LoadInt64(&c.heartbeatTimeout)
	if timeout <= 0 {
		return true
	}
	elapsed := ztime.ServerNowUnixMilli() - atomic.LoadInt64(&c.lastRecTime)
	return elapsed <= timeout
}

// SetHeartbeatTimeout 设置心跳超时（由 Server.AddChannel 调用），0 表示禁用。
func (c *BaseChannel) SetHeartbeatTimeout(d time.Duration) {
	atomic.StoreInt64(&c.heartbeatTimeout, d.Milliseconds())
}

// resetReadDeadline 重置 conn 的读超时（内核级心跳检测）
func (c *BaseChannel) resetReadDeadline() {
	timeout := atomic.LoadInt64(&c.heartbeatTimeout)
	if timeout <= 0 {
		return
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
}

// SetCloseCall 设置关闭回调（框架层 + 业务层）
func (c *BaseChannel) SetCloseCall(closeCall func(ziface.IChannel)) {
	c.closeCall = closeCall
}

// Send 异步发送消息（仅 async 模式：入发送队列，由 runSend 批量写出）。
//
// 语义与生命周期：
//   - 成功入队后，由 runSend 在写出完成后调用 IMessage.Release（每条消息恰好一次）。
//   - 若 channel 已关闭、未打开、sync 模式、或 TryEnqueue 失败（例如 Close 已 StopEnqueue），
//     本方法会立即调用 msg.Release()，消息不会发送；调用方不得再使用该 msg。
//   - 与 Close 并发时，可能出现「检查时尚可写，入队前队列已停」——此时 Release 即表示发送取消，并非泄漏。
//
// Sync 模式无发送队列，请使用 ReplyImmediate；误用 Send 将打日志并 Release。
func (c *BaseChannel) Send(msg ziface.IMessage) {
	// ✅ 快速检查：避免向已关闭的 Channel 发送
	if c.isClose.Load() {
		msg.Release()
		return
	}
	if !c.IsOpen() {
		msg.Release()
		return
	}
	// sync 模式：无发送队列，应使用 ReplyImmediate；误用 Send 时打日志并丢弃，避免 panic 打崩进程
	if c.mailBoxQueue == nil {
		zlog.Error("sync mode: Send not supported, use ReplyImmediate in handlers; message dropped",
			zap.Uint64("channelId", c.channelId))
		msg.Release()
		return
	}
	if !c.mailBoxQueue.TryEnqueue(msg) {
		// StopEnqueue 已生效（Close 与 Send 竞态）或节点分配后二次观察到停止；须由调用方 Release（Enqueue 会静默丢引用）
		msg.Release()
		return
	}
	// ✅ 无锁设计：无需通知，无需计数，runSend 轮询队列（消费结果驱动）
}

// StartSend 启动异步发送 goroutine
func (c *BaseChannel) StartSend(ctx context.Context) {
	c.sendDone.Add(1)
	go c.runSend(ctx)
}

// runSend 异步发送循环（原 Session.runSend）
func (c *BaseChannel) runSend(ctx context.Context) {
	defer c.sendDone.Done()
	defer zlog.Recover("Channel.runSend")

	// ✅ 使用动态批量大小
	readBatch := make([]ziface.IMessage, MaxBatchLimit)

	// 监控指标
	var totalMsgsSent int64

	// processBatch: 处理一批消息（count 已由外部保证 <= MaxBatchLimit）
	processBatch := func(msgs []ziface.IMessage) {
		count := len(msgs)
		// count == 0 由外部保证不会发生，此处省略检查

		// 零拷贝发送（writev）
		c.SendBatchMsg(msgs)

		// 记录字节数并释放消息
		totalBytes := 0
		for i := 0; i < count; i++ {
			totalBytes += len(msgs[i].GetMessageData())
			msgs[i].Release()
			msgs[i] = nil
		}

		// 记录监控统计
		if c.stats != nil {
			c.stats.RecordSend(count, totalBytes)
		}
		totalMsgsSent += int64(count)
	}

	// processBatch: 每次处理一批消息（防止信号检查饿死）
	// ✅ 使用"消费结果驱动"而非"队列长度"（避免原子RMW操作）
	lastBatchSize := 1 // 初始建议大小
	processBatchOnce := func() int {
		// ✅ 根据上次消费结果获取推荐的批量大小（无原子操作）
		batchSize := c.batcher.GetBatchSize(int64(lastBatchSize))

		// 确保批量大小不超过预分配的缓冲区
		if batchSize > MaxBatchLimit {
			batchSize = MaxBatchLimit
		}

		// ✅ 记录批处理开始时间
		startTime := time.Now()

		// 按推荐的批量大小取消息
		n := c.mailBoxQueue.DequeueBatch(readBatch[:batchSize])
		if n == 0 {
			return 0 // 队列为空
		}

		// ✅ 更新上次批量大小（驱动下次预测）
		lastBatchSize = n

		processBatch(readBatch[:n])

		// ✅ 记录批处理延迟
		elapsed := time.Since(startTime)
		c.batcher.RecordLatency(elapsed)

		return n
	}

	// ✅ 主循环：无锁设计，消费结果驱动 + Backoff 策略
	var shouldExit bool          // 优雅关闭标记
	idleCount := 0               // 空闲计数，用于 Backoff
	processedTotal := int64(0)   // 总处理数（用于定期更新监控计数）
	lastSyncCount := int64(0)    // 上次同步时的计数
	lastShrinkTime := time.Now() // 上次缩容时间

	for {
		// 1. 非阻塞检查退出信号
		if !shouldExit {
			select {
			case <-ctx.Done():
				shouldExit = true
			case <-c.closeChan:
				shouldExit = true
			default:
				// 继续执行
			}
		}

		// 2. 尝试处理一批消息
		processedCount := processBatchOnce()

		// 3. 如果收到退出信号且队列为空，退出
		if shouldExit && processedCount == 0 {
			return
		}

		// 4. 空闲处理：Backoff 策略，避免 CPU 空转
		if processedCount == 0 {
			idleCount++
			zbackoff.Backoff(idleCount, 10, 30, time.Microsecond)

			// ✅ 空闲时重置 lastBatchSize，避免使用过期的预测数据
			lastBatchSize = 1

			// ✅ 空闲时归零监控计数，避免监控假阳性（显示队列有积压但实际为空）
			atomic.StoreInt64(&c.msgCount, 0)

			// ✅ 持续空闲 30 秒后缩容节点池，释放内存
			if idleCount > 100 && time.Since(lastShrinkTime) > 30*time.Second {
				c.mailBoxQueue.Shrink()
				lastShrinkTime = time.Now()
			}

			continue
		}

		// 有消息处理，重置空闲计数
		idleCount = 0

		// ✅ 定期更新监控计数（每处理 1000 条消息同步一次）
		// 单消费者写入，无伪共享，性能影响可忽略
		processedTotal += int64(processedCount)
		if processedTotal-lastSyncCount >= 1000 {
			// 估算当前队列长度：lastBatchSize 作为近似值
			// 监控容忍秒级延迟，此估算足够准确
			atomic.StoreInt64(&c.msgCount, int64(lastBatchSize))
			lastSyncCount = processedTotal
		}
	}
}

// RecordRecv 记录接收统计（供 GateServer 使用）
func (c *BaseChannel) RecordRecv(dataLen int) {
	if c.stats != nil {
		c.stats.RecordRec(dataLen)
	}
}
