package zpool

import (
	"net"
)

// netBuffPool 用于缓存 net.Buffers（writev 参数），减少切片分配。
var netBuffPool = NewPool(func() *net.Buffers {
	b := make(net.Buffers, 0, 512)
	return &b
})

// GetNetBuffer 从池中获取 *net.Buffers，初始长度为 0。
func GetNetBuffer() *net.Buffers {
	return netBuffPool.Get()
}

// PutNetBuffer 将 *net.Buffers 归还到池中，并重置长度。
func PutNetBuffer(buf *net.Buffers) {
	*buf = (*buf)[:0]
	netBuffPool.Put(buf)
}
