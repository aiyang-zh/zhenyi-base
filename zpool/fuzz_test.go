package zpool

import "testing"

// FuzzBytesBufferWrite 对分级 bytes 池 Get/Write/Release 做模糊测试，要求不 panic。
func FuzzBytesBufferWrite(f *testing.F) {
	f.Add(byte(1), []byte("x"))
	f.Add(byte(64), []byte{})

	f.Fuzz(func(t *testing.T, sizeHint byte, chunks []byte) {
		n := int(sizeHint)
		if n <= 0 {
			n = 1
		}
		if n > maxSize {
			n = maxSize
		}
		buf := GetBytesBuffer(n)
		if buf == nil {
			t.Fatal("GetBytesBuffer returned nil")
		}
		defer buf.Release()
		if len(chunks) > 256*1024 {
			chunks = chunks[:256*1024]
		}
		_, _ = buf.Write(chunks)
	})
}
