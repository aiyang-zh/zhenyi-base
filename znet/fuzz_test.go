package znet

import (
	"bytes"
	"testing"
)

// FuzzBaseSocketParseFromRingBufferV0 模糊线协议 v0（12 字节头 + body）解析，要求不 panic。
func FuzzBaseSocketParseFromRingBufferV0(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0})
	f.Add([]byte{
		0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 5,
		'h', 'e', 'l', 'l', 'o',
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		socket := NewBaseSocket(SocketConfig{
			MaxHeaderLength: DefaultMaxHeaderLength,
			MaxDataLength:   4096,
			MaxMsgId:        DefaultMaxMsgId,
			ProtocolVersion: 0,
		})
		rb := NewRingBuffer(RingBufferConfig{Size: 65536})
		_, _ = rb.Write(data)

		pd := GetParseData()
		defer PutParseData(pd)

		const maxRounds = 256
		for i := 0; i < maxRounds; i++ {
			parsed, err := socket.ParseFromRingBuffer(rb, pd)
			if err != nil {
				return
			}
			if !parsed {
				break
			}
			pd.ResetForReuse()
		}
	})
}

// FuzzBaseSocketParseFromRingBufferV1 模糊线协议 v1（1 字节 version + 12 字节头 + body），要求不 panic。
func FuzzBaseSocketParseFromRingBufferV1(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		socket := NewBaseSocket(SocketConfig{
			MaxHeaderLength: DefaultMaxHeaderLength,
			MaxDataLength:   4096,
			MaxMsgId:        DefaultMaxMsgId,
			ProtocolVersion: 1,
		})
		rb := NewRingBuffer(RingBufferConfig{Size: 65536})
		_, _ = rb.Write(data)

		pd := GetParseData()
		defer PutParseData(pd)

		const maxRounds = 256
		for i := 0; i < maxRounds; i++ {
			parsed, err := socket.ParseFromRingBuffer(rb, pd)
			if err != nil {
				return
			}
			if !parsed {
				break
			}
			pd.ResetForReuse()
		}
	})
}

// FuzzRingBufferWritePeekRead 对 RingBuffer 写入、Peek、PeekTwoSlices、Discard、Read、WriteFromReader 做模糊测试，要求不 panic。
func FuzzRingBufferWritePeekRead(f *testing.F) {
	f.Add([]byte("ab"), 1, 1, 0, 1)
	f.Add([]byte{}, 0, 0, 0, 0)

	f.Fuzz(func(t *testing.T, data []byte, peekN, discardN, off, length int) {
		rb := NewRingBuffer(RingBufferConfig{Size: 256, MaxSize: 4096})
		if len(data) > 8000 {
			data = data[:8000]
		}
		_, _ = rb.Write(data)

		if rb.Len() > 0 {
			n := peekN
			if n < 0 {
				n = -n
			}
			_, _ = rb.Peek(n % 8192)

			first, second := rb.PeekAll()
			_ = first
			_ = second

			o := off
			if o < 0 {
				o = -o
			}
			ln := length
			if ln < 0 {
				ln = -ln
			}
			f1, f2, _ := rb.PeekTwoSlices(o%8192, ln%8192)
			_ = f1
			_ = f2
		}

		d := discardN
		if d < 0 {
			d = -d
		}
		_ = rb.Discard(d % 8192)

		buf := make([]byte, 256)
		_, _ = rb.Read(buf)

		_, _ = rb.WriteFromReader(bytes.NewReader(data), 512)
	})
}
