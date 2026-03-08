package znet

import (
	"errors"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// BaseClient 基础客户端（零拷贝实现）
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
	writeLock sync.Mutex // 写入锁，保护并发写入
}

// NewBaseClient 创建网络层客户端基类（零拷贝读、协议解析复用、可选加密）。
func NewBaseClient() *BaseClient {
	client := &BaseClient{
		readBuffer:   GetRingBuffer(), // 从池获取
		socketParser: NewBaseSocket(),
		state:        atomic.Bool{},
	}
	client.state.Store(true)

	// 初始化复用的 ParseData（与 BaseChannel 一致的零分配模式）
	client.parseData = ParseData{
		Message:      &client.parseMsg,
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
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
func (b *BaseClient) Close() error {
	if !b.state.CompareAndSwap(true, false) {
		return nil // 已关闭
	}

	// 释放 parseData 持有的 pool buffer
	for _, buf := range b.parseData.OwnedBuffers {
		buf.Release()
	}
	b.parseData.OwnedBuffers = b.parseData.OwnedBuffers[:0]

	// 归还 RingBuffer 到池
	if b.readBuffer != nil {
		PutRingBuffer(b.readBuffer)
		b.readBuffer = nil
	}

	// 关闭连接
	if b.conn != nil {
		return b.conn.Close()
	}
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

// Read 在单独 goroutine 中启动读循环，直至连接关闭；应在 Connect 后调用。
func (b *BaseClient) Read() {
	go func() {
		defer zlog.Recover("BaseClient Read recover")
		defer b.Close()
		for {
			n1 := b.read()
			if n1 != 0 {
				return
			}
		}
	}()
}

func (b *BaseClient) read() int {
	if !b.IsOpen() {
		return 1
	}

	// 从网络读取数据到 RingBuffer
	_, err := b.readBuffer.WriteFromReader(b.conn, 0)
	if err != nil {
		if err == ErrBufferFull {
			// 缓冲区满，继续处理已有数据
		} else if err == io.EOF {
			zlog.Debug("connection closed by remote (EOF)")
			return 1
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

	// 循环解析所有完整消息（复用 b.parseData，零池化开销）
	for {
		b.parseData.ResetForReuse()

		parsed, parseErr := b.socketParser.ParseFromRingBuffer(b.readBuffer, &b.parseData)
		if parseErr != nil {
			zlog.Warn("ParseFromRingBuffer error", zap.Error(parseErr))
			return 1
		}

		if !parsed {
			if b.readBuffer.IsFull() {
				zlog.Warn("single packet exceeds buffer capacity")
				return 1
			}
			break
		}

		// 解密
		wireMsg := b.parseData.Message
		if encData := wireMsg.GetMessageData(); len(encData) > 0 && b.iEncrypt != nil {
			decrypted, decryptErr := b.iEncrypt.Decrypt(encData)
			if decryptErr != nil {
				zlog.Warn("decrypt error", zap.Error(decryptErr))
				continue
			}
			wireMsg.SetMessageData(decrypted)
		}

		// 回调（传递 IWireMessage，回调方同步处理或自行拷贝数据）
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
