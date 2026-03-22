package znet

import (
	"errors"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"io"
	"sync/atomic"
)

// ================================================================
// Ring Buffer - 零拷贝环形缓冲区
// ================================================================
//
// 设计目标：
//   - 零拷贝：读取数据时直接返回内部 slice 引用，无需复制
//   - 高性能：预分配固定大小，避免运行时内存分配
//   - 简单：单生产者单消费者模型，无需加锁
//
// 使用场景：
//   - 网络 I/O 读取缓冲
//   - 协议解析
//
// ⚠️ 线程安全性：
//   - 非线程安全：同一 RingBuffer 上禁止并发调用 Read / Write / WriteFromReader / Discard / Peek* / Reset
//   - 适用于 goroutine-per-conn 模型（每连接一个 RingBuffer，单协程驱动读缓冲）
//
// ⚠️ Peek* 与生命周期：
//   - Peek / PeekBytes 等返回的是底层缓冲区的切片视图（零拷贝），在调用 Discard、Read、Write 或 Reset 之后
//     不得再使用该切片，否则可能读到后续写入覆盖的数据（与 use-after-free 类似语义）。
//   - 从池中取出时 PutRingBuffer 会 Reset（含 clear），避免连接间残留字节；调用方仍须遵守单协程与 Peek 约束。
//
// ⚠️ 指针溢出说明：
//   - read/write 使用 uint64，即使每秒写入 10GB，也需要 58 年才会溢出
//   - 在补码运算下，write - read 总能正确计算数据长度

var (
	ErrBufferFull       = errors.New("ring buffer is full")
	ErrBufferEmpty      = errors.New("ring buffer is empty")
	ErrInsufficientData = errors.New("insufficient data in buffer")
)

// ================================================================
// RingBuffer 对象池
// ================================================================
//
// 问题：每个连接创建一个 64KB RingBuffer，5000 连接 = 320MB
// 优化：连接关闭时归还 RingBuffer，新连接复用
//
// 使用方式：
//
//	rb := GetRingBuffer()       // 从池获取（默认 4KB，多连接时节省内存）
//	defer PutRingBuffer(rb)     // 归还到池
//
// 注意：池默认 4KB，DefaultRingBufferConfig 为 64KB，二者不一致是刻意的：
// 池用于每连接复用，取小以控制内存；直接 NewRingBuffer 默认 64KB 适合单缓冲场景。
const defaultRingBufferSize = 4 * 1024 // 4KB

var ringBufferPool = zpool.NewPool(func() *RingBuffer {
	return NewRingBuffer(RingBufferConfig{Size: defaultRingBufferSize})
})

// GetRingBuffer 从池中获取 RingBuffer
func GetRingBuffer() *RingBuffer {
	rb := ringBufferPool.Get()
	rb.Reset() // 确保干净状态
	return rb
}

// PutRingBuffer 归还 RingBuffer 到池
func PutRingBuffer(rb *RingBuffer) {
	if rb == nil {
		return
	}
	// 只归还标准大小的 buffer，避免内存膨胀
	if rb.size == defaultRingBufferSize {
		rb.Reset()
		ringBufferPool.Put(rb)
	}
	// 非标准大小的直接丢弃，让 GC 回收
}

// RingBuffer 环形缓冲区
type RingBuffer struct {
	buf  []byte // 底层缓冲区
	size uint64 // 缓冲区大小（必须是 2 的幂）
	mask uint64 // size - 1，用于快速取模

	// ✅ 使用 uint64 防止长期运行溢出
	// 即使每秒写入 10GB，也需要 58 年才会溢出
	read  uint64 // 读指针（单调递增）
	write uint64 // 写指针（单调递增）

	// 统计信息（可选，用于监控）
	totalRead  uint64
	totalWrite uint64

	// 扩容相关
	maxSize uint64 // 最大允许扩容到的大小，0 表示不限制
}

// RingBufferConfig Ring Buffer 配置
type RingBufferConfig struct {
	Size    int // 初始缓冲区大小；DefaultRingBufferConfig 为 64KB；池 GetRingBuffer 为 4KB
	MaxSize int // 最大扩容大小，默认 1MB，0 表示不限制
}

// DefaultRingBufferConfig 默认配置
func DefaultRingBufferConfig() RingBufferConfig {
	return RingBufferConfig{
		Size:    64 * 1024,   // 64KB
		MaxSize: 1024 * 1024, // 1MB
	}
}

// NewRingBuffer 创建新的 Ring Buffer
func NewRingBuffer(config ...RingBufferConfig) *RingBuffer {
	cfg := DefaultRingBufferConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	// 确保 size 是 2 的幂
	size := nextPowerOfTwo64(uint64(cfg.Size))
	maxSize := uint64(0)
	if cfg.MaxSize > 0 {
		maxSize = nextPowerOfTwo64(uint64(cfg.MaxSize))
	}

	return &RingBuffer{
		buf:     make([]byte, size),
		size:    size,
		mask:    size - 1,
		maxSize: maxSize,
	}
}

// nextPowerOfTwo64 返回大于等于 n 的最小 2 的幂
func nextPowerOfTwo64(n uint64) uint64 {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}

// ================================================================
// 基本操作
// ================================================================

// Len 返回缓冲区中可读数据长度
func (rb *RingBuffer) Len() int {
	// uint64 减法在溢出时仍然正确（补码运算）
	return int(rb.write - rb.read)
}

// Cap 返回缓冲区总容量
func (rb *RingBuffer) Cap() int {
	return int(rb.size)
}

// Free 返回缓冲区剩余可写空间
func (rb *RingBuffer) Free() int {
	return int(rb.size) - rb.Len()
}

// IsEmpty 检查缓冲区是否为空
func (rb *RingBuffer) IsEmpty() bool {
	return rb.read == rb.write
}

// IsFull 检查缓冲区是否已满
func (rb *RingBuffer) IsFull() bool {
	return rb.Len() == int(rb.size)
}

// Reset 重置逻辑读写到空，并清零底层 buf、复位统计计数。
// 对象池借还时依赖本方法消除残留字节，避免池化复用后 Peek 悬挂引用读到上一连接数据。
func (rb *RingBuffer) Reset() {
	rb.read = 0
	rb.write = 0
	if len(rb.buf) > 0 {
		clear(rb.buf)
	}
	atomic.StoreUint64(&rb.totalRead, 0)
	atomic.StoreUint64(&rb.totalWrite, 0)
}

// ================================================================
// 扩容操作
// ================================================================

// Grow 扩容缓冲区以容纳至少 n 字节的额外数据
// 返回是否成功扩容
func (rb *RingBuffer) Grow(n int) bool {
	if rb.Free() >= n {
		return true // 空间足够，无需扩容
	}

	// 计算需要的新大小
	needed := uint64(rb.Len() + n)
	newSize := rb.size * 2
	for newSize < needed {
		newSize *= 2
	}

	// 检查是否超过最大限制
	if rb.maxSize > 0 && newSize > rb.maxSize {
		if rb.maxSize > rb.size {
			newSize = rb.maxSize
		} else {
			return false // 已达最大限制
		}
	}

	if newSize <= rb.size {
		return false // 无法扩容
	}

	// 分配新缓冲区
	newBuf := make([]byte, newSize)

	// ✅ 优化：复用 PeekAll 简化搬运逻辑，减少重复代码
	dataLen := rb.Len()
	if dataLen > 0 {
		first, second := rb.PeekAll()
		copy(newBuf, first)
		if len(second) > 0 {
			copy(newBuf[len(first):], second)
		}
	}

	// 更新状态
	rb.buf = newBuf
	rb.size = newSize
	rb.mask = newSize - 1
	rb.read = 0
	rb.write = uint64(dataLen)

	return true
}

// ================================================================
// 零拷贝读取操作
// ================================================================

// Peek 零拷贝查看数据（不移动读指针）
// 返回最多 n 字节的数据切片
// ⚠️ 返回的切片直接引用内部缓冲区：在 Discard、Read、Write、WriteFromReader、Reset 或任何覆盖该区间
// 的写入之后均不得再使用；跨边界时仅返回第一段，需配合 PeekTwoSlices/PeekAll 取全量。
func (rb *RingBuffer) Peek(n int) ([]byte, error) {
	if rb.IsEmpty() {
		return nil, ErrBufferEmpty
	}

	available := rb.Len()
	if n > available {
		n = available
	}

	readPos := rb.read & rb.mask
	endPos := readPos + uint64(n)

	// 情况1：数据不跨越边界，直接返回切片
	if endPos <= rb.size {
		return rb.buf[readPos:endPos], nil
	}

	// 情况2：数据跨越边界，返回第一段
	// 调用方应使用 PeekAll 或 PeekTwoSlices 处理跨边界情况
	return rb.buf[readPos:rb.size], nil
}

// PeekAll 零拷贝查看所有可读数据
// 如果数据跨越边界，返回两个切片。
// ⚠️ 切片生命周期同 Peek：任何后续修改缓冲区的操作后均失效。
func (rb *RingBuffer) PeekAll() (first, second []byte) {
	if rb.IsEmpty() {
		return nil, nil
	}

	readPos := rb.read & rb.mask
	writePos := rb.write & rb.mask

	if writePos > readPos {
		// 数据连续
		return rb.buf[readPos:writePos], nil
	}

	// 数据跨越边界
	return rb.buf[readPos:], rb.buf[:writePos]
}

// PeekTwoSlices 零拷贝查看指定长度的数据，返回两个切片
// 用于协议解析层处理跨边界数据，避免 PeekBytes 的隐性拷贝。
// ⚠️ 切片生命周期同 Peek。
func (rb *RingBuffer) PeekTwoSlices(offset, length int) (first, second []byte, err error) {
	if rb.Len() < offset+length {
		return nil, nil, ErrInsufficientData
	}

	pos := (rb.read + uint64(offset)) & rb.mask
	endPos := pos + uint64(length)

	if endPos <= rb.size {
		// 不跨越边界
		return rb.buf[pos:endPos], nil, nil
	}

	// 跨越边界，返回两段
	firstLen := int(rb.size - pos)
	return rb.buf[pos:rb.size], rb.buf[:length-firstLen], nil
}

// Discard 丢弃已读取的数据（移动读指针）
func (rb *RingBuffer) Discard(n int) error {
	if n > rb.Len() {
		return ErrInsufficientData
	}
	rb.read += uint64(n)
	atomic.AddUint64(&rb.totalRead, uint64(n))
	return nil
}

// ================================================================
// 写入操作
// ================================================================

// Write 实现 io.Writer 接口
// 写入数据到缓冲区，如果空间不足会尝试扩容
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// 检查空间，必要时扩容
	if rb.Free() < len(p) {
		if !rb.Grow(len(p)) {
			// 扩容失败，只写入能写的部分
			if rb.Free() == 0 {
				return 0, ErrBufferFull
			}
			p = p[:rb.Free()]
		}
	}

	writePos := rb.write & rb.mask
	endPos := writePos + uint64(len(p))

	if endPos <= rb.size {
		// 不跨越边界
		copy(rb.buf[writePos:], p)
	} else {
		// 跨越边界，分两次写入
		firstPart := int(rb.size - writePos)
		copy(rb.buf[writePos:], p[:firstPart])
		copy(rb.buf[0:], p[firstPart:])
	}

	rb.write += uint64(len(p))
	atomic.AddUint64(&rb.totalWrite, uint64(len(p)))
	return len(p), nil
}

// WriteFromReader 从 io.Reader 读取数据写入缓冲区
// 返回实际写入的字节数
//
// ⚠️ 与 RingBuffer 上其它方法相同：须单 goroutine 串行调用，不得与 Read/Discard/Peek 等并发。
//
// ⚠️ 调用方注意事项：
//   - 如果返回 ErrBufferFull，说明缓冲区已满且达到 MaxSize 限制
//   - 调用方应暂停读取，等待消费者消费一些数据后（Free() > 0）再重试
//   - 对于非阻塞 socket，需要配合 epoll/select 使用背压机制
func (rb *RingBuffer) WriteFromReader(r io.Reader, maxRead int) (n int, err error) {
	if rb.IsFull() {
		// 尝试扩容
		if !rb.Grow(4096) { // 至少扩容 4KB
			return 0, ErrBufferFull
		}
	}

	free := rb.Free()
	if maxRead > 0 && maxRead < free {
		free = maxRead
	}

	writePos := rb.write & rb.mask
	endPos := writePos + uint64(free)

	if endPos <= rb.size {
		// 不跨越边界，直接读入
		n, err = r.Read(rb.buf[writePos : writePos+uint64(free)])
	} else {
		// 跨越边界，先读第一段
		firstPart := int(rb.size - writePos)
		n, err = r.Read(rb.buf[writePos:rb.size])

		// 如果第一段读满了且还有空间，继续读第二段
		if n == firstPart && err == nil {
			var n2 int
			n2, err = r.Read(rb.buf[:free-firstPart])
			n += n2
		}
	}

	if n > 0 {
		rb.write += uint64(n)
		atomic.AddUint64(&rb.totalWrite, uint64(n))
	}

	return n, err
}

// ================================================================
// io.Reader 接口实现
// ================================================================

// Read 实现 io.Reader 接口
// 从缓冲区读取数据到 p，返回实际读取的字节数
func (rb *RingBuffer) Read(p []byte) (n int, err error) {
	if rb.IsEmpty() {
		return 0, io.EOF
	}

	available := rb.Len()
	if len(p) > available {
		n = available
	} else {
		n = len(p)
	}

	readPos := rb.read & rb.mask
	endPos := readPos + uint64(n)

	if endPos <= rb.size {
		copy(p[:n], rb.buf[readPos:endPos])
	} else {
		firstPart := int(rb.size - readPos)
		copy(p[:firstPart], rb.buf[readPos:rb.size])
		copy(p[firstPart:n], rb.buf[:n-firstPart])
	}

	rb.read += uint64(n)
	atomic.AddUint64(&rb.totalRead, uint64(n))
	return n, nil
}

// ================================================================
// 便捷读取方法
// ================================================================

// ReadByte 实现 io.ByteReader 接口
func (rb *RingBuffer) ReadByte() (byte, error) {
	if rb.IsEmpty() {
		return 0, ErrBufferEmpty
	}

	b := rb.buf[rb.read&rb.mask]
	rb.read++
	atomic.AddUint64(&rb.totalRead, 1)
	return b, nil
}

// ReadUint32BE 读取大端序 uint32
func (rb *RingBuffer) ReadUint32BE() (uint32, error) {
	if rb.Len() < 4 {
		return 0, ErrInsufficientData
	}

	readPos := rb.read & rb.mask
	var v uint32

	if readPos+4 <= rb.size {
		// 不跨越边界
		v = uint32(rb.buf[readPos])<<24 |
			uint32(rb.buf[readPos+1])<<16 |
			uint32(rb.buf[readPos+2])<<8 |
			uint32(rb.buf[readPos+3])
	} else {
		// 跨越边界，逐字节读取
		for i := uint64(0); i < 4; i++ {
			v = v<<8 | uint32(rb.buf[(readPos+i)&rb.mask])
		}
	}

	rb.read += 4
	atomic.AddUint64(&rb.totalRead, 4)
	return v, nil
}

// ReadFull 读取指定长度的数据到目标缓冲区
// 这是有拷贝的操作，用于需要持久化数据的场景
func (rb *RingBuffer) ReadFull(p []byte) error {
	if rb.Len() < len(p) {
		return ErrInsufficientData
	}

	readPos := rb.read & rb.mask
	endPos := readPos + uint64(len(p))

	if endPos <= rb.size {
		copy(p, rb.buf[readPos:endPos])
	} else {
		firstPart := int(rb.size - readPos)
		copy(p[:firstPart], rb.buf[readPos:rb.size])
		copy(p[firstPart:], rb.buf[:len(p)-firstPart])
	}

	rb.read += uint64(len(p))
	atomic.AddUint64(&rb.totalRead, uint64(len(p)))
	return nil
}

// ================================================================
// 协议解析辅助方法
// ================================================================

// PeekByte 查看指定偏移处的单字节（不移动指针）
func (rb *RingBuffer) PeekByte(offset int) (uint8, error) {
	if rb.Len() < offset+1 {
		return 0, ErrInsufficientData
	}
	pos := (rb.read + uint64(offset)) & rb.mask
	return rb.buf[pos], nil
}

// PeekUint32BE 零拷贝查看大端序 uint32（不移动指针）
func (rb *RingBuffer) PeekUint32BE(offset int) (uint32, error) {
	if rb.Len() < offset+4 {
		return 0, ErrInsufficientData
	}

	pos := (rb.read + uint64(offset)) & rb.mask
	var v uint32

	if pos+4 <= rb.size {
		v = uint32(rb.buf[pos])<<24 |
			uint32(rb.buf[pos+1])<<16 |
			uint32(rb.buf[pos+2])<<8 |
			uint32(rb.buf[pos+3])
	} else {
		for i := uint64(0); i < 4; i++ {
			v = v<<8 | uint32(rb.buf[(pos+i)&rb.mask])
		}
	}

	return v, nil
}

// PeekHeader12 一次性读取 12 字节协议头，返回 3 个 BigEndian uint32。
// 调用方必须保证 rb.Len() >= offset+12，否则行为未定义。
func (rb *RingBuffer) PeekHeader12(offset int) (a, b, c uint32) {
	pos := (rb.read + uint64(offset)) & rb.mask
	if pos+12 <= rb.size {
		buf := rb.buf[pos:]
		a = uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		b = uint32(buf[4])<<24 | uint32(buf[5])<<16 | uint32(buf[6])<<8 | uint32(buf[7])
		c = uint32(buf[8])<<24 | uint32(buf[9])<<16 | uint32(buf[10])<<8 | uint32(buf[11])
	} else {
		for i := uint64(0); i < 4; i++ {
			a = a<<8 | uint32(rb.buf[(pos+i)&rb.mask])
		}
		for i := uint64(4); i < 8; i++ {
			b = b<<8 | uint32(rb.buf[(pos+i)&rb.mask])
		}
		for i := uint64(8); i < 12; i++ {
			c = c<<8 | uint32(rb.buf[(pos+i)&rb.mask])
		}
	}
	return
}

// PeekBytes 零拷贝查看指定偏移和长度的数据
// ⚠️ 如果数据跨越边界，会分配新内存拷贝
// 建议：优先使用 PeekTwoSlices 避免拷贝
func (rb *RingBuffer) PeekBytes(offset, length int) ([]byte, error) {
	if rb.Len() < offset+length {
		return nil, ErrInsufficientData
	}

	pos := (rb.read + uint64(offset)) & rb.mask
	endPos := pos + uint64(length)

	if endPos <= rb.size {
		// 不跨越边界，零拷贝返回
		return rb.buf[pos:endPos], nil
	}

	// 跨越边界，需要拷贝
	result := make([]byte, length)
	firstPart := int(rb.size - pos)
	copy(result[:firstPart], rb.buf[pos:rb.size])
	copy(result[firstPart:], rb.buf[:length-firstPart])
	return result, nil
}

// ================================================================
// 统计信息
// ================================================================

// Stats 返回统计信息
func (rb *RingBuffer) Stats() (totalRead, totalWrite int64) {
	return int64(atomic.LoadUint64(&rb.totalRead)), int64(atomic.LoadUint64(&rb.totalWrite))
}

// ================================================================
// 接口断言
// ================================================================

var (
	_ io.Reader     = (*RingBuffer)(nil)
	_ io.Writer     = (*RingBuffer)(nil)
	_ io.ByteReader = (*RingBuffer)(nil)
)
