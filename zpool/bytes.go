package zpool

import (
	"math/bits"
	"sync/atomic"
)

// Buffer 封装 []byte 切片，用指针存入 sync.Pool 避免 interface{} 装箱分配。
type Buffer struct {
	B []byte
}

// Reset 重置有效长度但保留底层容量。
func (b *Buffer) Reset() {
	b.B = b.B[:0]
}

// Len 返回有效数据长度。
func (b *Buffer) Len() int {
	return len(b.B)
}

// Write 追加数据（实现 io.Writer 接口语义）。
func (b *Buffer) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	return len(p), nil
}

// Release 将缓冲区归还到分级池。
func (b *Buffer) Release() {
	PutBytesBuffer(b)
}

const (
	minShift  = 6
	maxShift  = 16
	maxSize   = 1 << maxShift
	poolCount = maxShift - minShift + 1
)

var (
	getRequests      [poolCount]atomic.Uint64
	putReturns       [poolCount]atomic.Uint64
	directAllocCount atomic.Uint64
	pools            [poolCount]*Pool[*Buffer] // *Buffer 指针存入 sync.Pool，Put 时零分配
)

func init() {
	for i := 0; i < poolCount; i++ {
		size := 1 << (i + minShift)
		pools[i] = NewPool(func() *Buffer {
			return &Buffer{
				B: make([]byte, size),
			}
		})
	}
}

// GetBytesBuffer 从分级池获取 *Buffer，buf.B 长度设为 size。
// size 超过 maxSize（64KB）时会直接分配，不回收到池中。
func GetBytesBuffer(size int) *Buffer {
	if size > maxSize {
		directAllocCount.Add(1)
		return &Buffer{B: make([]byte, size)}
	}
	if size <= 0 {
		panic("netlib: GetBuffer size must be positive")
	}

	idx := getIndex(size)
	getRequests[idx].Add(1)

	buf := pools[idx].Get()
	buf.B = buf.B[:size]
	return buf
}

// GetBytesBufferZero 获取 *Buffer 并将内容清零。
func GetBytesBufferZero(size int) *Buffer {
	b := GetBytesBuffer(size)
	clear(b.B)
	return b
}

// PutBytesBuffer 归还 *Buffer 到对应的分级池。
// 对于容量过小或超大的缓冲区将直接丢弃，由 GC 回收。
func PutBytesBuffer(b *Buffer) {
	if b == nil {
		return
	}

	capSize := cap(b.B)
	if capSize < (1<<minShift) || capSize > maxSize {
		return // 超大 buffer 不回池，GC 回收
	}

	idx := getIndex(capSize)
	if capSize != (1 << (idx + minShift)) {
		return
	}

	putReturns[idx].Add(1)
	b.B = b.B[:cap(b.B)] // 恢复到满容量
	pools[idx].Put(b)    // *Buffer 指针 → interface{} 零分配
}

func getIndex(n int) int {
	if n <= (1 << minShift) {
		return 0
	}
	return bits.Len32(uint32(n-1)) - minShift
}

// GetStats 获取全局字节缓冲池的统计信息（供监控使用）。
func GetStats() BufferPoolStats {
	stats := BufferPoolStats{
		DirectAllocs: directAllocCount.Load(),
	}

	for i := 0; i < poolCount; i++ {
		getReq := getRequests[i].Load()
		putRet := putReturns[i].Load()

		stats.BucketStats[i] = BucketStat{
			BucketSize:  1 << (i + minShift),
			GetRequests: getReq,
			PutReturns:  putRet,
			ReuseRate:   calcRate(putRet, getReq),
		}
	}

	return stats
}

// BufferPoolStats 为字节缓冲池的统计快照（各档位 Get/Put 与直接分配次数）。
type BufferPoolStats struct {
	BucketStats  [poolCount]BucketStat
	DirectAllocs uint64
}

// BucketStat 表示某一档位缓冲区的统计项。
type BucketStat struct {
	BucketSize  int     // 该档位缓冲区容量（字节）
	GetRequests uint64  // 该档位 Get 请求次数
	PutReturns  uint64  // 该档位 Put 归还次数
	ReuseRate   float64 // 复用率（Put/Get 百分比）
}

func calcRate(put, get uint64) float64 {
	if get == 0 {
		return 0
	}
	return float64(put) / float64(get) * 100
}
