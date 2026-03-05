package zpool

import (
	"bytes"
)

var bufferPool = NewPool(func() *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, 16*1024))
	return buf
})

func GetBuffer() *bytes.Buffer {
	return bufferPool.Get()
}

func PutBuffer(buf *bytes.Buffer) {
	buf.Reset() // 保留底层数组，只重置读写位置
	bufferPool.Put(buf)
}
