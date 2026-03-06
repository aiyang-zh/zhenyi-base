package zserialize

import (
	"bytes"
	"sync"

	"github.com/vmihailenco/msgpack"
)

var encoderPool = sync.Pool{
	New: func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 2048)) // 预分配缓冲区
		enc := msgpack.NewEncoder(buf)
		return &EncoderBuffer{
			buf: buf,
			enc: enc,
		}
	},
}
var decoderPool = sync.Pool{
	New: func() interface{} {
		// 创建一个 Decoder，并未绑定具体 Reader，或者绑定一个空的
		dec := msgpack.NewDecoder(nil)
		return dec
	},
}

type EncoderBuffer struct {
	buf *bytes.Buffer
	enc *msgpack.Encoder
}

func MarshalMsgPack(obj interface{}) ([]byte, error) {
	item := encoderPool.Get().(*EncoderBuffer)
	defer encoderPool.Put(item)

	item.buf.Reset()

	// 复用 Encoder 进行编码
	if err := item.enc.Encode(obj); err != nil {
		return nil, err
	}

	// 必须深拷贝，否则归还 pool 后数据会被覆盖
	b := make([]byte, item.buf.Len())
	copy(b, item.buf.Bytes())
	return b, nil
}

func UnmarshalMsgPack(body []byte, obj interface{}) error {
	dec := decoderPool.Get().(*msgpack.Decoder)
	defer decoderPool.Put(dec)

	err := dec.Reset(bytes.NewReader(body))
	if err != nil {
		return err
	}

	return dec.Decode(obj)
}
