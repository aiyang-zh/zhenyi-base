package znet

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zbackoff"
	"github.com/aiyang-zh/zhenyi-base/zbatch"
	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi-base/zqueue"
	"go.uber.org/zap"
)

// ClientOption 客户端创建时的可选配置。
type ClientOption func(*BaseClient)

// WithSocketConfig 设置客户端协议解析与安全限制（如 ProtocolVersion、MaxDataLength）。
func WithSocketConfig(cfg SocketConfig) ClientOption {
	return func(b *BaseClient) {
		b.socketParser = NewBaseSocket(cfg)
		if b.readBuffer != nil {
			b.readBuffer.SetMaxSize(RingBufferMaxSizeForSocket(cfg))
		}
		hdrLen := b.socketParser.HeaderLen()
		tuning := GetSendLoopTuning()
		if tuning.BatchMax > 0 && hdrLen > 0 {
			b.headersBuf = make([]byte, 0, tuning.BatchMax*hdrLen)
			b.writeBufs = make(net.Buffers, 0, tuning.BatchMax*2)
		}
	}
}

// WithAsyncMode 启用异步模式：SetReadCall + Read() 流式收包。
func WithAsyncMode() ClientOption {
	return func(b *BaseClient) { b.mode = ziface.ModeAsync }
}

// WithTLSConfig 设置客户端 TLS/GM-TLS 配置；由 ztcp/zws 在 Connect 时使用。
func WithTLSConfig(cfg *ziface.TLSConfig) ClientOption {
	return func(b *BaseClient) { b.tlsConfig = cfg }
}

// WithDialTimeout 设置客户端建连超时；0 表示使用系统默认（无额外超时）。
func WithDialTimeout(timeout time.Duration) ClientOption {
	return func(b *BaseClient) { b.dialTimeout = timeout }
}

// WithWebSocketPath 设置 WebSocket 握手路径（默认 "/"）。
func WithWebSocketPath(path string) ClientOption {
	return func(b *BaseClient) { b.wsPath = path }
}

// WithWebSocketHeaders 设置 WebSocket 握手附加 HTTP 头。
func WithWebSocketHeaders(headers http.Header) ClientOption {
	return func(b *BaseClient) { b.wsHeaders = headers }
}

// BaseClient 基础客户端（零拷贝、热路径无锁）。
// 设计目标：高性能低延迟、热路径无锁、0 分配；activeConn 供 read/Close 协调；async 有发送队列+runSend；sync（Request）同步直写。
type BaseClient struct {
	// 零拷贝相关
	readBuffer   *RingBuffer // 读取缓冲区
	socketParser *BaseSocket // 协议解析器

	// 零拷贝解析复用，避免每条消息 pool Get/Put
	parseData ParseData
	parseMsg  NetMessage

	// 数据加密
	iEncrypt   ziface.IEncrypt
	readCall   func(ziface.IWireMessage)
	state      atomic.Bool
	activeConn atomic.Value // 底层 net.Conn；Load 供 read/写，Close 时关闭

	readWg        sync.WaitGroup // 读 goroutine 退出后 Close 才做 buffer 等清理
	syncWriteLock sync.Mutex     // 仅 sync 路径 writeImmediate 使用；async 由 runSend 单协程写

	mode ziface.ConnMode // 默认 ModeSync（Request）；ModeAsync 时用 Read

	requestReader    *bufio.Reader      // Request 路径用，懒创建
	requestHeaderBuf [headerSizeV1]byte // Request 直读路径复用，覆盖 v0/v1 最大头长

	mailBoxQueue  *zqueue.UnboundedMPSC[ziface.IMessage]
	sendDone      sync.WaitGroup
	sendCloseChan chan struct{}
	sendLoopOnce  sync.Once
	headersBuf    []byte
	writeBufs     net.Buffers
	batcher       *zbatch.FastAdaptiveBatcher
	queuedMsgs    int64

	tlsConfig   *ziface.TLSConfig
	dialTimeout time.Duration
	wsPath      string
	wsHeaders   http.Header
}

// NewBaseClient 创建网络层客户端基类。默认 sync（Request）；可选 WithAsyncMode() 启用 async（Read）。
// 默认 iEncrypt 为 zencrypt.BaseEncrypt（空操作，等价不加密）；真实加密请 SetEncrypt 替换。
func NewBaseClient(opts ...ClientOption) *BaseClient {
	tuning := GetSendLoopTuning()
	client := &BaseClient{
		readBuffer:    GetRingBuffer(),
		socketParser:  NewBaseSocket(),
		mailBoxQueue:  zqueue.NewUnboundedMPSC[ziface.IMessage](),
		sendCloseChan: make(chan struct{}, 1),
		batcher: zbatch.NewFastAdaptiveBatcher(
			tuning.BatchMin,
			tuning.BatchMax,
			tuning.BatchTargetMean,
		),
		iEncrypt: zencrypt.NewBaseEncrypt(),
	}
	client.state.Store(true)

	hdrLen := client.socketParser.HeaderLen()
	if tuning.BatchMax > 0 && hdrLen > 0 {
		client.headersBuf = make([]byte, 0, tuning.BatchMax*hdrLen)
		client.writeBufs = make(net.Buffers, 0, tuning.BatchMax*2)
	}

	client.parseData = ParseData{
		Message:      &client.parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	for _, opt := range opts {
		opt(client)
	}
	client.readBuffer.SetMaxSize(RingBufferMaxSizeForSocket(client.socketParser.config))
	return client
}

// TLSConfig 返回客户端 TLS 配置（可能为 nil）。
func (b *BaseClient) TLSConfig() *ziface.TLSConfig {
	if b == nil {
		return nil
	}
	return b.tlsConfig
}

// DialTimeout 返回客户端建连超时；0 表示无额外超时。
func (b *BaseClient) DialTimeout() time.Duration {
	if b == nil {
		return 0
	}
	return b.dialTimeout
}

// WebSocketPath 返回 WebSocket 握手路径；空字符串表示默认 "/"。
func (b *BaseClient) WebSocketPath() string {
	if b == nil {
		return ""
	}
	return b.wsPath
}

// WebSocketHeaders 返回 WebSocket 握手附加 HTTP 头（可能为 nil）。
func (b *BaseClient) WebSocketHeaders() http.Header {
	if b == nil {
		return nil
	}
	return b.wsHeaders
}

// SetReadCall 设置收到完整消息时的回调（在 Read 循环中同步调用）。
func (b *BaseClient) SetReadCall(readCall func(ziface.IWireMessage)) {
	b.readCall = readCall
}

// SetEncrypt 设置加解密实现；传 nil 时恢复为 BaseEncrypt（空操作，等价不加密）。
func (b *BaseClient) SetEncrypt(iEncrypt ziface.IEncrypt) {
	if iEncrypt == nil {
		b.iEncrypt = zencrypt.NewBaseEncrypt()
		return
	}
	b.iEncrypt = iEncrypt
}

// IsOpen 返回连接是否仍处于打开状态。
func (b *BaseClient) IsOpen() bool {
	return b.state.Load()
}

// SetConn 注入底层连接（由 ztcp/zws/zkcp 的 Connect 内部调用）。
// async 模式在 SetConn 时启动 runSend；sync 模式不入队、不启动发送协程。
func (b *BaseClient) SetConn(conn net.Conn) {
	b.activeConn.Store(conn)
	if b.mode == ziface.ModeAsync {
		b.StartSend()
	}
}

// GetConn 返回底层 net.Conn，一般仅用于调试或特殊场景。
func (b *BaseClient) GetConn() net.Conn {
	return b.loadConn()
}

func (b *BaseClient) loadConn() net.Conn {
	if !b.state.Load() {
		return nil
	}
	v := b.activeConn.Load()
	if v == nil {
		return nil
	}
	return v.(net.Conn)
}

// StartSend 启动异步发送协程（async 模式；重复调用仅生效一次）。
func (b *BaseClient) StartSend() {
	b.sendLoopOnce.Do(func() {
		b.sendDone.Add(1)
		go b.runSend()
	})
}

// Close 优雅关闭：禁止入队 → 通知 runSend 退出 → 关 conn → 等待 runSend 排空 → 等待读协程 → 回收资源。
//
// 关闭顺序：
//   - StopEnqueue 须在通知 runSend 之前，避免 TOCTOU 入队后永不写出；
//   - 关闭 conn 使阻塞在 WriteTo/Read 的协程尽快返回；
//   - sendDone.Wait 等待 runSend 把已入队消息处理完再退出。
func (b *BaseClient) Close() error {
	if !b.state.CompareAndSwap(true, false) {
		return nil // 已关闭
	}

	// Step 0.5: 禁止入队（async 有 runSend；sync 无 runSend 但调用无害）
	if b.mailBoxQueue != nil {
		b.mailBoxQueue.StopEnqueue()
	}

	// Step 1: 通知 runSend 退出（state 已由 CAS 置 false）
	select {
	case <-b.sendCloseChan:
	default:
		close(b.sendCloseChan)
	}

	// Step 2: 关闭底层连接，强制 runSend / read 中阻塞 I/O 立即返回
	if v := b.activeConn.Load(); v != nil {
		if err := v.(net.Conn).Close(); err != nil {
			// 关连清理：过滤「连接已关闭」类预期错误，其余 Close 错误记 Warn，见 docs/API.md「日志级别」。
			errMsg := err.Error()
			if !strings.Contains(errMsg, "use of closed network connection") {
				zlog.Warn("Close connection error", zap.Error(err))
			}
		}
	}

	// Step 3: 等待 runSend 排空队列并退出
	b.sendDone.Wait()

	// Step 4: 等待 Read() 协程退出（客户端特有）
	b.readWg.Wait()

	// Step 5: 释放读缓冲与解析临时资源
	for _, buf := range b.parseData.OwnedBuffers {
		buf.Release()
	}
	b.parseData.OwnedBuffers = b.parseData.OwnedBuffers[:0]

	if b.readBuffer != nil {
		PutRingBuffer(b.readBuffer)
		b.readBuffer = nil
	}
	if b.mailBoxQueue != nil {
		b.mailBoxQueue.Close()
	}
	b.requestReader = nil
	return nil
}

// SendMsg 发送一条消息。async：拷贝入队由 runSend 写出；sync：同步直写（调用方可在返回后 Release）。
func (b *BaseClient) SendMsg(message ziface.IMessage) {
	if message == nil {
		return
	}
	if !b.IsOpen() {
		zlog.Debug("SendMsg: client is closed")
		return
	}
	conn := b.loadConn()
	if conn == nil {
		return
	}
	if b.mode == ziface.ModeAsync {
		out := GetNetMessage()
		out.SetMsgId(message.GetMsgId())
		out.SetSeqId(message.GetSeqId())
		out.SetDataCopy(message.GetMessageData())
		b.SendMsgAsync(out)
		return
	}
	b.writeImmediate(message, conn)
}

// SendMsgAsync 异步入队（仅 async）；成功入队后不得再 Release，写出后由 runSend Release。
func (b *BaseClient) SendMsgAsync(message ziface.IMessage) {
	if message == nil {
		return
	}
	if b.mode != ziface.ModeAsync {
		zlog.Warn("SendMsgAsync: client not in async mode, message dropped")
		message.Release()
		return
	}
	if !b.IsOpen() {
		message.Release()
		return
	}
	if !b.mailBoxQueue.TryEnqueue(message) {
		message.Release()
		return
	}
	atomic.AddInt64(&b.queuedMsgs, 1)
}

// Request 同步请求：发送消息并阻塞直到收到一条响应。默认模式。
// 适用于 sync/RPC 场景。使用 io.ReadFull 直读，无 RingBuffer 开销。
//
// 与 Read() 互斥。若创建时用了 WithAsyncMode()，则 Request 返回错误。
func (b *BaseClient) Request(msg ziface.IMessage) (ziface.IWireMessage, error) {
	if !b.IsOpen() {
		return nil, io.EOF
	}
	conn := b.loadConn()
	if conn == nil {
		return nil, errors.New("client: not connected")
	}
	if b.mode == ziface.ModeAsync {
		return nil, errors.New("client: created with WithAsyncMode(), use Read() instead of Request()")
	}
	b.writeImmediate(msg, conn)
	if b.requestReader == nil {
		b.requestReader = bufio.NewReader(conn)
	}
	// 直读路径：按 HeaderLen 读取 header + body。
	hdrLen := b.socketParser.HeaderLen()
	hdr := b.requestHeaderBuf[:hdrLen]
	if _, err := io.ReadFull(b.requestReader, hdr); err != nil {
		if err == io.EOF || b.isNormalCloseError(err) {
			return nil, io.EOF
		}
		return nil, err
	}
	off := 0
	if b.socketParser.config.ProtocolVersion >= 1 {
		if hdr[0] != b.socketParser.config.ProtocolVersion {
			return nil, zerrs.Newf(zerrs.ErrTypeValidation, "protocol version mismatch: got %d, expect %d", hdr[0], b.socketParser.config.ProtocolVersion)
		}
		off = 1
	}
	msgId := int32(binary.BigEndian.Uint32(hdr[off : off+4]))
	maxMsgId := b.socketParser.config.MaxMsgId
	if int(msgId) < -maxMsgId || int(msgId) > maxMsgId {
		return nil, zerrs.InvalidParameterf("msgId %d out of range", msgId)
	}
	seqId := binary.BigEndian.Uint32(hdr[off+4 : off+8])
	dataLen := binary.BigEndian.Uint32(hdr[off+8 : off+12])
	maxDataLen := b.socketParser.config.MaxDataLength
	var body []byte
	if dataLen > 0 {
		if int(dataLen) > maxDataLen {
			return nil, zerrs.InvalidParameterf("invalid data length: %d (max: %d)", dataLen, maxDataLen)
		}
		body = make([]byte, dataLen)
		if _, err := io.ReadFull(b.requestReader, body); err != nil {
			if err == io.EOF || b.isNormalCloseError(err) {
				return nil, io.EOF
			}
			return nil, err
		}
		decrypted, decryptErr := b.iEncrypt.Decrypt(body)
		if decryptErr != nil {
			zlog.Warn("Request decrypt error", zap.Error(decryptErr))
			return nil, decryptErr
		}
		body = decrypted
	}
	return &NetMessage{MsgId: msgId, SeqId: seqId, Data: body}, nil
}

// Read 在单独 goroutine 中启动读循环，直至连接关闭；应在 Connect 后调用。
// 需创建时传入 WithAsyncMode() 启用；默认 ModeSync 下调用会 panic。
func (b *BaseClient) Read() {
	if b.mode != ziface.ModeAsync {
		panic("client: use WithAsyncMode() when creating client to enable Read(); default is sync (Request) mode")
	}
	b.readWg.Add(1)
	go func() {
		defer zlog.Recover("BaseClient Read recover")
		defer b.Close()
		for {
			n1 := b.read()
			if n1 != 0 {
				b.readWg.Done()
				return
			}
		}
	}()
}

// writeImmediate sync 路径同步写出（Request / sync SendMsg）。
func (b *BaseClient) writeImmediate(message ziface.IMessage, conn net.Conn) {
	if !b.IsOpen() {
		return
	}

	data := message.GetMessageData()
	encryptedData, err := b.iEncrypt.Encrypt(data)
	if err != nil {
		zlog.Warn("writeImmediate Encrypt error", zap.Error(err))
		return
	}

	wrapper := GetNetMessage()
	defer wrapper.Release()
	wrapper.SetMsgId(message.GetMsgId())
	wrapper.SetSeqId(message.GetSeqId())
	wrapper.SetMessageData(encryptedData)

	var headerBuf [headerSizeV1]byte
	headerLen, body := b.socketParser.PreparePacket(wrapper, headerBuf[:])

	buffers := net.Buffers{headerBuf[:headerLen]}
	if len(body) > 0 {
		buffers = append(buffers, body)
	}

	b.syncWriteLock.Lock()
	_, err = buffers.WriteTo(conn)
	b.syncWriteLock.Unlock()
	if err != nil {
		zlog.Warn("writeImmediate Write error", zap.Error(err))
	}
}

func (b *BaseClient) runSend() {
	defer b.sendDone.Done()
	defer zlog.Recover("BaseClient.runSend")

	tuning := GetSendLoopTuning()
	readBatchLimit := tuning.MaxBatchLimit
	if readBatchLimit <= 0 || readBatchLimit > MaxBatchLimit {
		readBatchLimit = MaxBatchLimit
	}
	readBatch := make([]ziface.IMessage, readBatchLimit)

	processBatch := func(msgs []ziface.IMessage) {
		count := len(msgs)
		b.sendBatchMsg(msgs)
		for i := 0; i < count; i++ {
			msgs[i].Release()
			msgs[i] = nil
		}
		atomic.AddInt64(&b.queuedMsgs, -int64(count))
	}

	lastBatchSize := 1
	processBatchOnce := func() int {
		batchSize := b.batcher.GetBatchSize(int64(lastBatchSize))
		if batchSize > readBatchLimit {
			batchSize = readBatchLimit
		}
		startTime := time.Now()
		n := b.mailBoxQueue.DequeueBatch(readBatch[:batchSize])
		if n == 0 {
			return 0
		}
		lastBatchSize = n
		processBatch(readBatch[:n])
		b.batcher.RecordLatency(time.Since(startTime))
		return n
	}

	var shouldExit bool
	idleCount := 0
	lastShrinkTime := time.Now()

	for {
		if !shouldExit {
			select {
			case <-b.sendCloseChan:
				shouldExit = true
			default:
			}
		}

		processedCount := processBatchOnce()
		if shouldExit && processedCount == 0 {
			return
		}

		if processedCount == 0 {
			idleCount++
			zbackoff.Backoff(idleCount, tuning.BackoffFirst, tuning.BackoffSecond, tuning.BackoffSleep)
			lastBatchSize = 1
			if idleCount > tuning.IdleShrinkAfter && time.Since(lastShrinkTime) > tuning.IdleShrinkEvery {
				b.mailBoxQueue.Shrink()
				lastShrinkTime = time.Now()
			}
			continue
		}
		idleCount = 0
	}
}

// sendBatchMsg 批量加密组包并 writev；仅 runSend 调用（async 单协程写，无锁）。
func (b *BaseClient) sendBatchMsg(messages []ziface.IMessage) {
	if len(messages) == 0 || !b.IsOpen() {
		return
	}

	conn := b.loadConn()
	if conn == nil {
		return
	}

	hdrLen := b.socketParser.HeaderLen()
	if cap(b.writeBufs) < 2*len(messages) {
		b.writeBufs = make(net.Buffers, 0, 2*len(messages))
	} else {
		b.writeBufs = b.writeBufs[:0]
	}

	wrapper := GetNetMessage()
	defer wrapper.Release()

	sent := 0
	for _, msg := range messages {
		data := msg.GetMessageData()
		wireData, err := b.iEncrypt.Encrypt(data)
		if err != nil {
			zlog.Warn("sendBatchMsg Encrypt error",
				zap.Int32("msgId", msg.GetMsgId()),
				zap.Uint32("seqId", msg.GetSeqId()),
				zap.Error(err))
			continue
		}

		wrapper.Reset()
		wrapper.SetMsgId(msg.GetMsgId())
		wrapper.SetSeqId(msg.GetSeqId())
		wrapper.SetMessageData(wireData)

		needHeaders := (sent + 1) * hdrLen
		if cap(b.headersBuf) < needHeaders {
			grow := needHeaders
			if cap(b.headersBuf) > 0 {
				grow = cap(b.headersBuf) * 2
				if grow < needHeaders {
					grow = needHeaders
				}
			}
			nb := make([]byte, needHeaders, grow)
			copy(nb, b.headersBuf[:sent*hdrLen])
			b.headersBuf = nb
		} else {
			b.headersBuf = b.headersBuf[:needHeaders]
		}
		offset := sent * hdrLen
		headerLen, body := b.socketParser.PreparePacket(wrapper, b.headersBuf[offset:offset+hdrLen])

		b.writeBufs = append(b.writeBufs, b.headersBuf[offset:offset+headerLen])
		if len(body) > 0 {
			b.writeBufs = append(b.writeBufs, body)
		}
		sent++
	}

	if len(b.writeBufs) == 0 {
		return
	}
	if _, err := b.writeBufs.WriteTo(conn); err != nil {
		var netErr net.Error
		if zerrs.As(err, &netErr) && netErr.Timeout() {
			return
		}
		if !b.isNormalCloseError(err) {
			zlog.Warn("sendBatchMsg Write error", zap.Error(err))
		}
	}
}

func (b *BaseClient) read() int {
	if !b.IsOpen() {
		return 1
	}
	conn := b.loadConn()
	rb := b.readBuffer
	if conn == nil || rb == nil {
		return 1
	}
	// 不在持锁下做阻塞 I/O，避免 Close() 等待时与 read 死锁；Close 关 conn 后此处会得到错误并 return 1
	_, err := rb.WriteFromReader(conn, 0)
	if err != nil {
		if err == ErrBufferFull {
			if rb.Grow(readRingGrowStepBytes) {
				_, err = rb.WriteFromReader(conn, 0)
				if err != nil && err != ErrBufferFull {
				} else {
					err = nil
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				zlog.Debug("connection closed by remote (EOF)")
				return 1
			}
			if err == ErrBufferFull {
			} else if b.isNormalCloseError(err) {
				return 1
			} else {
				var opError *net.OpError
				if errors.As(err, &opError) {
					return 1
				}
				zlog.Warn("read error", zap.Error(err))
				return 1
			}
		}
	}

	// 循环解析所有完整消息（复用 b.parseData，零池化开销）
	for {
		b.parseData.ResetForReuse()

		parsed, parseErr := b.socketParser.ParseFromRingBuffer(rb, &b.parseData)
		if parseErr != nil {
			zlog.Warn("ParseFromRingBuffer error", zap.Error(parseErr))
			return 1
		}

		if !parsed {
			if rb.IsFull() {
				// 初始池化 RingBuffer 仅 4KB；单帧（header+body）可大于容量（如应用层大包）。
				// 扩容后继续读，避免误判为协议错误并 Close 导致连接半残。
				if !rb.Grow(readRingGrowStepBytes) {
					zlog.Warn("single packet exceeds buffer capacity (Grow failed)")
					return 1
				}
				break
			}
			break
		}

		wireMsg := b.parseData.Message
		if encData := wireMsg.GetMessageData(); len(encData) > 0 {
			decrypted, decryptErr := b.iEncrypt.Decrypt(encData)
			if decryptErr != nil {
				zlog.Warn("decrypt error", zap.Error(decryptErr))
				continue
			}
			wireMsg.SetMessageData(decrypted)
		}

		if b.readCall != nil {
			b.readCall(wireMsg)
		}
	}

	return 0
}

// isNormalCloseError 判断是否是正常的连接关闭错误
func (b *BaseClient) isNormalCloseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "use of closed network connection") ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "forcibly closed by the remote host")
}
