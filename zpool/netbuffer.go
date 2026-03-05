package zpool

import (
	"net"
)

var netBuffPool = NewPool(func() *net.Buffers {
	b := make(net.Buffers, 0, 512)
	return &b
})

func GetNetBuffer() *net.Buffers {
	return netBuffPool.Get()
}

func PutNetBuffer(buf *net.Buffers) {
	*buf = (*buf)[:0]
	netBuffPool.Put(buf)
}
