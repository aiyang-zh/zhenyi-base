package znet

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"go.uber.org/zap"
)

// ClientOption 客户端创建时的可选配置。
type ClientOption func(*BaseClient)

// WithAsyncMode 启用异步模式：SetReadCall + Read() 流式收包。
func WithAsyncMode() ClientOption {
	return func(b *BaseClient) { b.mode = ziface.ModeAsync }
}

// BaseClient 基础客户端（零拷贝、热路径无锁）。
// 设计目标：高性能低延迟、热路径无锁、0 分配；connMu 仅用于 Close 与 read 的关闭协调，非热路径。
type BaseClient struct {
	// 零拷贝相关
	readBuffer   *RingBuffer // 读取缓冲区
	socketParser *BaseSocket // 协议解析器

	// 零拷贝解析复用（与 BaseChannel 一致，避免每条消息 pool Get/Put）
	parseData ParseData
	parseMsg  NetMessage

	// 数据加密
	iEncrypt  ziface.IEncrypt
	readCall  func(ziface.IWireMessage)
	state     atomic.Bool
	conn      net.Conn
	connMu    sync.RWMutex    // 保护 conn/readBuffer 的关闭与读；Close 先关 conn 再 readWg.Wait 再持锁清理，避免死锁
	readWg    sync.WaitGroup  // 读 goroutine 退出后 Close 才做 buffer 等清理
	writeLock sync.Mutex      // 写入锁，保护并发写入
	mode      ziface.ConnMode // 默认 ModeSync（Request）；ModeAsync 时用 Read

	requestReader    *bufio.Reader // Request 路径用，懒创建
	requestHeaderBuf [12]byte      // Request 直读路径复用，避免每请求分配
}

// NewBaseClient 创建网络层客户端基类。默认 sync（Request）；可选 WithAsyncMode() 启用 async（Read）。
func NewBaseClient(opts ...ClientOption) *BaseClient {
	client := &BaseClient{
		readBuffer:   GetRingBuffer(),
		socketParser: NewBaseSocket(),
		state:        atomic.Bool{},
	}
	client.state.Store(true)

	client.parseData = ParseData{
		Message:      &client.parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}

	for _, opt := range opts {
		opt(client)
	}
	return client
}

// SetReadCall 设置收到完整消息时的回调（在 Read 循环中同步调用）。
func (b *BaseClient) SetReadCall(readCall func(ziface.IWireMessage)) {
	b.readCall = readCall
}

// SetEncrypt 设置加解密实现，nil 表示不加密。
func (b *BaseClient) SetEncrypt(iEncrypt ziface.IEncrypt) {
	b.iEncrypt = iEncrypt
}

// IsOpen 返回连接是否仍处于打开状态。
func (b *BaseClient) IsOpen() bool {
	return b.state.Load()
}

// SetConn 注入底层连接（由 ztcp/zws/zkcp 的 Connect 内部调用）。
func (b *BaseClient) SetConn(conn net.Conn) {
	b.conn = conn
}

// GetConn 返回底层 net.Conn，一般仅用于调试或特殊场景。
func (b *BaseClient) GetConn() net.Conn {
	return b.conn
}

// Close 关闭连接并释放 RingBuffer/ParseData 等资源；幂等。
// 先关 conn 唤醒可能阻塞在 read 的 goroutine，再 readWg.Wait 后持锁清理，避免 read 持 RLock 时 Close 等 Lock 死锁。
func (b *BaseClient) Close() error {
	if !b.state.CompareAndSwap(true, false) {
		return nil // 已关闭
	}

	b.connMu.RLock()
	conn := b.conn
	b.connMu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}
	b.readWg.Wait()

	b.connMu.Lock()
	defer b.connMu.Unlock()

	for _, buf := range b.parseData.OwnedBuffers {
		buf.Release()
	}
	b.parseData.OwnedBuffers = b.parseData.OwnedBuffers[:0]

	if b.readBuffer != nil {
		PutRingBuffer(b.readBuffer)
		b.readBuffer = nil
	}
	b.requestReader = nil
	b.conn = nil
	return nil
}

// SendMsg 发送一条消息（会加密、组包、writev 写入）；连接已关闭时静默忽略。
func (b *BaseClient) SendMsg(message ziface.IMessage) {
	if !b.IsOpen() {
		zlog.Debug("SendMsg: client is closed")
		return
	}

	// 加密
	data := message.GetMessageData()
	var encryptedData []byte
	var err error
	if b.iEncrypt != nil {
		encryptedData, err = b.iEncrypt.Encrypt(data)
		if err != nil {
			zlog.Warn("SendMsg Encrypt error", zap.Error(err))
			return
		}
	} else {
		encryptedData = data
	}

	// 复用临时 NetMessage 构建发送数据包
	wrapper := GetNetMessage()
	defer wrapper.Release()
	wrapper.SetMsgId(message.GetMsgId())
	wrapper.SetSeqId(message.GetSeqId())
	wrapper.SetMessageData(encryptedData)

	// 零拷贝准备数据包
	var headerBuf [headerSize]byte
	headerLen, body := b.socketParser.PreparePacket(wrapper, headerBuf[:])

	// 使用 net.Buffers (writev) 发送
	buffers := net.Buffers{headerBuf[:headerLen]}
	if len(body) > 0 {
		buffers = append(buffers, body)
	}

	// 保护并发写入
	b.writeLock.Lock()
	_, err = buffers.WriteTo(b.conn)
	b.writeLock.Unlock()

	if err != nil {
		zlog.Warn("SendMsg Write error", zap.Error(err))
		return
	}
}

// Request 同步请求：发送消息并阻塞直到收到一条响应。默认模式。
// 适用于 sync/RPC 场景。使用 io.ReadFull 直读，无 RingBuffer 开销。
//
// 与 Read() 互斥。若创建时用了 WithAsyncMode()，则 Request 返回错误。
func (b *BaseClient) Request(msg ziface.IMessage) (ziface.IWireMessage, error) {
	if !b.IsOpen() {
		return nil, io.EOF
	}
	if b.conn == nil {
		return nil, errors.New("client: not connected")
	}
	if b.mode == ziface.ModeAsync {
		return nil, errors.New("client: created with WithAsyncMode(), use Read() instead of Request()")
	}
	b.SendMsg(msg)
	if b.requestReader == nil {
		b.requestReader = bufio.NewReader(b.conn)
	}
	// 直读路径：io.ReadFull(header 12B) + io.ReadFull(body)，与 Zinx 同结构
	hdr := b.requestHeaderBuf[:]
	if _, err := io.ReadFull(b.requestReader, hdr); err != nil {
		if err == io.EOF || b.isNormalCloseError(err) {
			return nil, io.EOF
		}
		return nil, err
	}
	msgId := int32(binary.BigEndian.Uint32(hdr[0:4]))
	seqId := binary.BigEndian.Uint32(hdr[4:8])
	dataLen := binary.BigEndian.Uint32(hdr[8:12])
	var body []byte
	if dataLen > 0 {
		if dataLen > uint32(DefaultMaxDataLength) {
			return nil, ErrBufferFull
		}
		body = make([]byte, dataLen)
		if _, err := io.ReadFull(b.requestReader, body); err != nil {
			if err == io.EOF || b.isNormalCloseError(err) {
				return nil, io.EOF
			}
			return nil, err
		}
		if b.iEncrypt != nil {
			decrypted, decryptErr := b.iEncrypt.Decrypt(body)
			if decryptErr != nil {
				zlog.Warn("Request decrypt error", zap.Error(decryptErr))
				return nil, decryptErr
			}
			body = decrypted
		}
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

func (b *BaseClient) read() int {
	if !b.IsOpen() {
		return 1
	}
	b.connMu.RLock()
	conn := b.conn
	rb := b.readBuffer
	b.connMu.RUnlock()
	if conn == nil || rb == nil {
		return 1
	}
	// 不在持锁下做阻塞 I/O，避免 Close() 等待 Lock 时与 read 死锁；Close 关 conn 后此处会得到错误并 return 1
	_, err := rb.WriteFromReader(conn, 0)
	if err != nil {
		if err == ErrBufferFull {
			if rb.Grow(65536) {
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
				zlog.Warn("single packet exceeds buffer capacity")
				return 1
			}
			break
		}

		wireMsg := b.parseData.Message
		if encData := wireMsg.GetMessageData(); len(encData) > 0 && b.iEncrypt != nil {
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
