package znet

import (
	"context"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zqueue"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zbackoff"
	"github.com/aiyang-zh/zhenyi-base/zbatch"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi-base/ztime"
)

// 接口断言：确保 BaseChannel 实现 IChannel
var _ ziface.IChannel = (*BaseChannel)(nil)

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
	authId           int64  // 认证 ID（用户 ID）
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
func NewBaseChannel(channelId uint64, conn net.Conn, server ziface.IServer) *BaseChannel {
	state := &atomic.Bool{}
	state.Store(true)

	c := &BaseChannel{
		// 网络层
		channelId:    channelId,
		server:       server,
		addr:         conn.RemoteAddr().String(),
		conn:         conn,
		state:        state,
		readBuffer:   GetRingBuffer(), // ✅ 从池中获取，减少内存分配
		socketParser: NewBaseSocket(),

		// 会话层（原 Session 字段）
		lastRecTime:      ztime.ServerNowUnixMilli(),
		heartbeatTimeout: 30 * time.Second.Milliseconds(), // 默认 30 秒
		closeChan:        make(chan struct{}, 1),
		mailBoxQueue:     zqueue.NewUnboundedMPSC[ziface.IMessage](),

		// ✅ 极速自适应批量处理器（网络场景配置）
		batcher: zbatch.NewFastAdaptiveBatcher(
			1,                  // minBatch: 最小批次 1
			200,                // maxBatch: 最大批次 200
			5*time.Millisecond, // targetP99: 网络场景目标延迟 5ms
		),
	}

	// ✅ 初始化复用的 ParseData（避免每条消息 pool Get/Put）
	c.parseData = ParseData{
		Message:      &c.parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	// 设置 TCP 缓冲区
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetWriteBuffer(64 * 1024)
		_ = tcpConn.SetReadBuffer(64 * 1024)
		_ = tcpConn.SetNoDelay(true) // 禁用 Nagle 算法，减少延迟
	}

	return c
}

// IsOpen 检查通道是否打开
func (c *BaseChannel) IsOpen() bool {
	return c.state.Load()
}

// read 零拷贝读取（核心优化点）
func (c *BaseChannel) read() int {
	// 1. 从网络读取数据到 Ring Buffer
	nRead, err := c.readBuffer.WriteFromReader(c.conn, 0)
	if nRead > 0 {
		if c.metrics != nil {
			c.metrics.BytesRecAdd(int64(nRead))
		}
		c.resetReadDeadline()
	}
	if err != nil {
		// 缓冲区满：背压机制，先处理已有数据
		if err == ErrBufferFull {
			// 不返回错误，继续处理已有数据
			// 下次读取时如果还是满的，说明消费者处理不过来
			zlog.Warn("Read buffer full, applying backpressure",
				zap.Uint64("channelId", c.channelId),
				zap.Int("bufLen", c.readBuffer.Len()))
		} else if err == io.EOF {
			return 1
		} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			if c.metrics != nil {
				c.metrics.ConnHeartbeatTimeoutInc()
			}
			zlog.Warn("Connection heartbeat timeout, closing",
				zap.Uint64("channelId", c.channelId),
				zap.Int64("authId", c.GetAuthId()),
				zap.String("addr", c.addr))
			return 1
		} else if !c.isNormalCloseError(err) {
			if c.metrics != nil {
				c.metrics.ConnErrorsInc()
			}
			zlog.Warn("Read error",
				zap.Uint64("channelId", c.channelId),
				zap.Error(err))
			return 1
		} else {
			return 1
		}
	}

	// 2. 循环解析所有完整的消息（复用 c.parseData，零池化开销）
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
			return 1
		}

		if !parsed {
			// ⚠️ 防止死循环：Buffer 满了却解析不出包，说明单个包超过 Buffer 容量
			if c.readBuffer.IsFull() {
				zlog.Error("Single packet exceeds buffer capacity, disconnecting",
					zap.Uint64("channelId", c.channelId),
					zap.Int("bufferCap", c.readBuffer.Cap()),
					zap.Int("bufferLen", c.readBuffer.Len()))
				return 1
			}

			break
		}

		// 3. 解密（如果需要）
		netNessage := c.parseData.Message
		if encryptedData := netNessage.GetMessageData(); len(encryptedData) > 0 {
			decryptedData, err1 := c.server.GetEncrypt().Decrypt(encryptedData)
			if err1 != nil {
				zlog.Error("Decrypt error",
					zap.Uint64("channelId", c.channelId),
					zap.Int32("msgId", netNessage.GetMsgId()),
					zap.Error(err1))
				return 0
			}
			netNessage.SetMessageData(decryptedData)
		}

		// 4. 处理消息（✅ 直接传 channel，避免 map 查找）
		c.server.HandleRead(c, netNessage)
	}

	return 0
}

// isNormalCloseError 判断是否是正常的连接关闭错误
func (c *BaseChannel) isNormalCloseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	normalErrors := []string{
		"use of closed network connection",
		"connection reset by peer",
		"forcibly closed by the remote host",
	}
	for _, e := range normalErrors {
		if strings.Contains(errMsg, e) {
			c.Close()
			return true
		}
	}
	return false
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
		n := c.read()
		if n != 0 {
			return
		}
	}
}

// Flush 刷新缓冲区
// 零拷贝模式下使用 writev 直接发送，无应用层缓冲，直接返回 nil
func (c *BaseChannel) Flush() error {
	return nil
}

// GetWriterTier 获取 writer 当前等级
// 零拷贝模式下无缓冲层，返回 TierNone
func (c *BaseChannel) GetWriterTier() ziface.BufferTier {
	return ziface.TierNone
}

// GetBuffered 获取缓冲区中已写入但未刷新的字节数
// 零拷贝模式下无缓冲层，返回 0
func (c *BaseChannel) GetBuffered() int {
	return 0
}

// SendBatchMsg 批量发送消息（实现 IChannel 接口）
// ✅ 零拷贝优化：使用 ISocketV2.PreparePacket + net.Buffers (writev)
func (c *BaseChannel) SendBatchMsg(messages []ziface.IMessage) {
	if messages == nil || len(messages) == 0 {
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

// Close 关闭通道（优雅关闭：等待发送完成）
func (c *BaseChannel) Close() {
	if c.isClose.Load() {
		return
	}

	c.closeOnce.Do(func() {
		c.isClose.Store(true)

		// Step 0: 从 Server 的 map 中移除，阻止新消息路由到此 channel
		c.server.RemoveChannel(c.channelId)

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
func (c *BaseChannel) GetAuthId() int64 {
	return atomic.LoadInt64(&c.authId)
}

// SetAuthId 设置认证 ID
func (c *BaseChannel) SetAuthId(authId int64) {
	atomic.StoreInt64(&c.authId, authId)
}

// GetRpcId 获取并递增 RPC ID
func (c *BaseChannel) GetRpcId() uint64 {
	return atomic.AddUint64(&c.rpcId, 1)
}

// SetLimit 设置限流器
func (c *BaseChannel) SetLimit(rate ziface.ILimit) {
	c.rate = rate
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

// setHeartbeatTimeout 设置心跳超时（由 BaseServer.AddChannel 内部调用）
func (c *BaseChannel) setHeartbeatTimeout(d time.Duration) {
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

// Send 异步发送消息（入队列）
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
	c.mailBoxQueue.Enqueue(msg)
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
