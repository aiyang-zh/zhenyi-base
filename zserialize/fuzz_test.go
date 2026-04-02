package zserialize

import "testing"

// FuzzUnmarshalJson 对任意字节做 JSON 反序列化，要求不 panic。
func FuzzUnmarshalJson(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":1,"name":"x"}`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var v BenchmarkData
		_ = UnmarshalJson(data, &v)
	})
}

// FuzzMarshalJsonRoundtrip 在「能解出有效结构」时做编解码往返。
func FuzzMarshalJsonRoundtrip(f *testing.F) {
	f.Add([]byte(`{"id":1,"name":"a","email":"e","age":1,"active":true,"score":1.5}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var v BenchmarkData
		if err := UnmarshalJson(data, &v); err != nil {
			return
		}
		b, err := MarshalJson(&v)
		if err != nil {
			t.Fatalf("MarshalJson: %v", err)
		}
		var v2 BenchmarkData
		if err := UnmarshalJson(b, &v2); err != nil {
			t.Fatalf("roundtrip UnmarshalJson: %v", err)
		}
	})
}

// FuzzUnmarshalMsgPack 对任意字节做 MsgPack 反序列化，要求不 panic。
func FuzzUnmarshalMsgPack(f *testing.F) {
	f.Add([]byte{0x80})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var v BenchmarkData
		_ = UnmarshalMsgPack(data, &v)
	})
}
