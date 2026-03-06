package zmodel

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"
)

// ============================================================
// CloneVT tests
// ============================================================

func TestCloneVT_NilMessage(t *testing.T) {
	var m *Message
	cloned := m.CloneVT()
	if cloned != nil {
		t.Errorf("CloneVT(nil) should return nil, got %v", cloned)
	}
}

func TestCloneVT_EmptyMessage(t *testing.T) {
	m := &Message{}
	cloned := m.CloneVT()
	if cloned == nil {
		t.Fatal("CloneVT(empty) should not return nil")
	}
	if cloned == m {
		t.Error("CloneVT should return a new instance, not same pointer")
	}
	if !m.EqualVT(cloned) {
		t.Error("Cloned empty message should EqualVT original")
	}
}

func TestCloneVT_FullMessage(t *testing.T) {
	m := &Message{
		AuthId:     100,
		SessionId:  200,
		RpcId:      300,
		TraceIdHi:  400,
		TraceIdLo:  500,
		SpanId:     600,
		MsgId:      7,
		SrcActor:   8,
		TarActor:   9,
		RefCount:   10,
		SeqId:      11,
		ToClient:   true,
		FromClient: true,
		IsResponse: true,
		Data:       []byte("hello world"),
		AuthIds:    []int64{1, 2, 3, 4, 5},
	}
	cloned := m.CloneVT()
	if cloned == nil {
		t.Fatal("CloneVT should not return nil")
	}
	if cloned == m {
		t.Error("CloneVT should return a new instance")
	}
	if !m.EqualVT(cloned) {
		t.Error("Cloned message should EqualVT original")
	}
	if !bytes.Equal(m.Data, cloned.Data) {
		t.Error("Cloned Data should equal original")
	}
	if len(m.AuthIds) != len(cloned.AuthIds) {
		t.Errorf("AuthIds len mismatch: %d vs %d", len(m.AuthIds), len(cloned.AuthIds))
	}
	for i, v := range m.AuthIds {
		if cloned.AuthIds[i] != v {
			t.Errorf("AuthIds[%d] mismatch: %d vs %d", i, v, cloned.AuthIds[i])
		}
	}
}

func TestCloneVT_DeepCopyModifyOriginal(t *testing.T) {
	m := &Message{
		Data:    []byte("original"),
		AuthIds: []int64{1, 2, 3},
	}
	cloned := m.CloneVT()
	if cloned == nil {
		t.Fatal("CloneVT should not return nil")
	}

	// Modify original
	m.Data[0] = 'X'
	m.AuthIds[0] = 999
	m.MsgId = 42

	// Clone should be unaffected
	if cloned.MsgId == 42 {
		t.Error("Clone should not be affected by original MsgId change")
	}
	if bytes.Equal(cloned.Data, m.Data) {
		t.Error("Clone Data should be independent; modifying original should not affect clone")
	}
	if cloned.AuthIds[0] == 999 {
		t.Error("Clone AuthIds should be independent")
	}
	if string(cloned.Data) != "original" {
		t.Errorf("Clone Data should still be 'original', got %q", string(cloned.Data))
	}
	if cloned.AuthIds[0] != 1 {
		t.Errorf("Clone AuthIds[0] should be 1, got %d", cloned.AuthIds[0])
	}
}

func TestCloneMessageVT(t *testing.T) {
	m := &Message{MsgId: 42, AuthId: 123}
	cloned := m.CloneMessageVT()
	if cloned == nil {
		t.Fatal("CloneMessageVT should not return nil")
	}
	msg, ok := cloned.(*Message)
	if !ok {
		t.Fatalf("CloneMessageVT should return *Message, got %T", cloned)
	}
	if !m.EqualVT(msg) {
		t.Error("CloneMessageVT result should EqualVT original")
	}
}

// ============================================================
// EqualVT tests
// ============================================================

func TestEqualVT_Self(t *testing.T) {
	m := &Message{MsgId: 1, Data: []byte("x")}
	if !m.EqualVT(m) {
		t.Error("message should EqualVT itself")
	}
}

func TestEqualVT_SameValues(t *testing.T) {
	a := &Message{MsgId: 1, AuthId: 100, Data: []byte("data")}
	b := &Message{MsgId: 1, AuthId: 100, Data: []byte("data")}
	if !a.EqualVT(b) {
		t.Error("messages with same values should EqualVT")
	}
}

func TestEqualVT_DifferentAuthId(t *testing.T) {
	a := &Message{AuthId: 1}
	b := &Message{AuthId: 2}
	if a.EqualVT(b) {
		t.Error("different AuthId should not EqualVT")
	}
}

func TestEqualVT_DifferentMsgId(t *testing.T) {
	a := &Message{MsgId: 1}
	b := &Message{MsgId: 2}
	if a.EqualVT(b) {
		t.Error("different MsgId should not EqualVT")
	}
}

func TestEqualVT_DifferentData(t *testing.T) {
	a := &Message{Data: []byte("a")}
	b := &Message{Data: []byte("b")}
	if a.EqualVT(b) {
		t.Error("different Data should not EqualVT")
	}
}

func TestEqualVT_DifferentAuthIds(t *testing.T) {
	a := &Message{AuthIds: []int64{1, 2}}
	b := &Message{AuthIds: []int64{1, 3}}
	if a.EqualVT(b) {
		t.Error("different AuthIds should not EqualVT")
	}
}

func TestEqualVT_AuthIdsLengthMismatch(t *testing.T) {
	a := &Message{AuthIds: []int64{1, 2}}
	b := &Message{AuthIds: []int64{1, 2, 3}}
	if a.EqualVT(b) {
		t.Error("different AuthIds length should not EqualVT")
	}
}

func TestEqualVT_NilReceiver(t *testing.T) {
	var m *Message
	other := &Message{MsgId: 1}
	if m.EqualVT(other) {
		t.Error("nil receiver EqualVT non-nil should be false")
	}
}

func TestEqualVT_NilArgument(t *testing.T) {
	m := &Message{MsgId: 1}
	if m.EqualVT(nil) {
		t.Error(" EqualVT(nil) should be false")
	}
}

func TestEqualVT_BothNil(t *testing.T) {
	var a, b *Message
	// When both are nil, this == that in EqualVT, so it returns true
	if !a.EqualVT(b) {
		t.Error("nil EqualVT nil returns true (this == that)")
	}
}

func TestEqualMessageVT_SameType(t *testing.T) {
	a := &Message{MsgId: 1}
	b := &Message{MsgId: 1}
	if !a.EqualMessageVT(b) {
		t.Error("EqualMessageVT with same type and values should be true")
	}
}

func TestEqualMessageVT_DifferentType(t *testing.T) {
	m := &Message{MsgId: 1}
	// Pass a different proto.Message - use nil which fails type assertion
	if m.EqualMessageVT(nil) {
		t.Error("EqualMessageVT(nil) should be false")
	}
}

// ============================================================
// MarshalVT / UnmarshalVT roundtrip tests
// ============================================================

func TestMarshalVT_EmptyRoundtrip(t *testing.T) {
	orig := &Message{}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Error("roundtrip empty message: decoded should EqualVT original")
	}
}

func TestMarshalVT_NilMessage(t *testing.T) {
	var m *Message
	buf, err := m.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT(nil) should not error: %v", err)
	}
	if buf != nil {
		t.Error("MarshalVT(nil) should return nil buffer")
	}
}

func TestMarshalVT_FullRoundtrip(t *testing.T) {
	orig := &Message{
		AuthId:     111,
		SessionId:  222,
		RpcId:      333,
		TraceIdHi:  444,
		TraceIdLo:  555,
		SpanId:     666,
		MsgId:      42,
		SrcActor:   1,
		TarActor:   2,
		RefCount:   3,
		SeqId:      4,
		ToClient:   true,
		FromClient: false,
		IsResponse: true,
		Data:       []byte("payload data"),
		AuthIds:    []int64{10, 20, 30},
	}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("roundtrip mismatch: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestMarshalVT_OnlyData(t *testing.T) {
	orig := &Message{Data: []byte("only data field")}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !bytes.Equal(decoded.Data, orig.Data) {
		t.Errorf("Data mismatch: got %q", decoded.Data)
	}
}

func TestMarshalVT_OnlyAuthIds(t *testing.T) {
	orig := &Message{AuthIds: []int64{100, 200, 300}}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if len(decoded.AuthIds) != len(orig.AuthIds) {
		t.Errorf("AuthIds len: got %d want %d", len(decoded.AuthIds), len(orig.AuthIds))
	}
	for i, v := range orig.AuthIds {
		if decoded.AuthIds[i] != v {
			t.Errorf("AuthIds[%d]: got %d want %d", i, decoded.AuthIds[i], v)
		}
	}
}

// ============================================================
// SizeVT tests
// ============================================================

func TestSizeVT_EmptyMessage(t *testing.T) {
	m := &Message{}
	n := m.SizeVT()
	if n < 0 {
		t.Error("SizeVT should be non-negative")
	}
	if n != 0 {
		t.Logf("empty SizeVT=%d (proto may use minimal encoding)", n)
	}
}

func TestSizeVT_WithData(t *testing.T) {
	m := &Message{Data: []byte("hello")}
	n := m.SizeVT()
	if n <= 0 {
		t.Errorf("SizeVT with data should be positive, got %d", n)
	}
	// Size should account for data: tag + length + 5 bytes
	if n < 7 {
		t.Errorf("SizeVT too small for 5-byte data: %d", n)
	}
}

func TestSizeVT_NilMessage(t *testing.T) {
	var m *Message
	n := m.SizeVT()
	if n != 0 {
		t.Errorf("SizeVT(nil) should be 0, got %d", n)
	}
}

func TestSizeVT_ConsistentWithMarshal(t *testing.T) {
	m := &Message{MsgId: 1, Data: []byte("test"), AuthIds: []int64{1, 2}}
	expected := m.SizeVT()
	buf, err := m.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	if len(buf) != expected {
		t.Errorf("MarshalVT length %d != SizeVT %d", len(buf), expected)
	}
}

// ============================================================
// MarshalToVT / MarshalToSizedBufferVT tests
// ============================================================

func TestMarshalToVT_PreallocatedBuffer(t *testing.T) {
	m := &Message{MsgId: 42, AuthId: 123}
	size := m.SizeVT()
	buf := make([]byte, size+10) // extra space
	n, err := m.MarshalToVT(buf)
	if err != nil {
		t.Fatalf("MarshalToVT: %v", err)
	}
	if n != size {
		t.Errorf("MarshalToVT wrote %d bytes, SizeVT is %d", n, size)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf[:n]); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !m.EqualVT(decoded) {
		t.Error("MarshalToVT roundtrip mismatch")
	}
}

func TestMarshalToSizedBufferVT(t *testing.T) {
	m := &Message{MsgId: 1, Data: []byte("x")}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(buf)
	if err != nil {
		t.Fatalf("MarshalToSizedBufferVT: %v", err)
	}
	if n != size {
		t.Errorf("MarshalToSizedBufferVT wrote %d, expected %d", n, size)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !m.EqualVT(decoded) {
		t.Error("MarshalToSizedBufferVT roundtrip mismatch")
	}
}

func TestMarshalToVT_NilMessage(t *testing.T) {
	var m *Message
	buf := make([]byte, 64)
	n, err := m.MarshalToVT(buf)
	if err != nil {
		t.Fatalf("MarshalToVT(nil) should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("MarshalToVT(nil) should write 0 bytes, got %d", n)
	}
}

// ============================================================
// UnmarshalVT error cases
// ============================================================

func TestUnmarshalVT_TruncatedData(t *testing.T) {
	valid := &Message{MsgId: 1, Data: []byte("hello")}
	buf, _ := valid.MarshalVT()
	// Truncate in the middle
	truncated := buf[:len(buf)/2]
	m := &Message{}
	err := m.UnmarshalVT(truncated)
	if err == nil {
		t.Error("UnmarshalVT truncated data should error")
	}
}

func TestUnmarshalVT_EmptyBuffer(t *testing.T) {
	m := &Message{}
	err := m.UnmarshalVT(nil)
	if err != nil {
		t.Logf("UnmarshalVT(nil) returned: %v (may be acceptable)", err)
	}
	err = m.UnmarshalVT([]byte{})
	if err != nil {
		t.Logf("UnmarshalVT([]byte{}) returned: %v (may be acceptable)", err)
	}
}

func TestUnmarshalVT_InvalidVarint(t *testing.T) {
	// Truncated varint: tag 1 (AuthId), then 0xFF 0xFF - incomplete, will trigger ErrUnexpectedEOF
	invalid := []byte{0x08, 0xFF, 0xFF}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT truncated varint should error")
	}
}

func TestUnmarshalVT_InvalidFieldTag(t *testing.T) {
	// Wire format: fieldNum=0 is illegal
	invalid := []byte{0x00, 0x00}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT illegal tag should error")
	}
}

func TestUnmarshalVT_WrongWireType(t *testing.T) {
	// Field 1 (AuthId) expects wireType 0 (varint), send wireType 2 (length-delimited)
	// Tag: (1<<3)|2 = 10
	invalid := []byte{0x0a, 0x00}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT wrong wire type should error")
	}
}

func TestUnmarshalVT_WireTypeEndGroup(t *testing.T) {
	// Wire type 4 (end group) is invalid for non-group messages
	// Tag (1<<3)|4 = 12, varint = 0x0c
	invalid := []byte{0x0c}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT wire type 4 (end group) should error")
	}
	if err != nil && err.Error() != "proto: Message: wiretype end group for non-group" {
		t.Logf("got error: %v", err)
	}
}

func TestUnmarshalVT_IntOverflow(t *testing.T) {
	// Tag 1 (AuthId), then 11 bytes of 0xFF - varint overflow (shift >= 64)
	invalid := []byte{0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT oversized varint should error with ErrIntOverflow")
	}
}

// ============================================================
// msg.pb.go getter tests
// ============================================================

func TestGetAuthId_Nil(t *testing.T) {
	var m *Message
	if v := m.GetAuthId(); v != 0 {
		t.Errorf("GetAuthId(nil) should be 0, got %d", v)
	}
}

func TestGetAuthId_Populated(t *testing.T) {
	m := &Message{AuthId: 12345}
	if v := m.GetAuthId(); v != 12345 {
		t.Errorf("GetAuthId = %d, want 12345", v)
	}
}

func TestGetSessionId_NilAndPopulated(t *testing.T) {
	var nilMsg *Message
	if v := nilMsg.GetSessionId(); v != 0 {
		t.Errorf("GetSessionId(nil) = %d, want 0", v)
	}
	m := &Message{SessionId: 999}
	if v := m.GetSessionId(); v != 999 {
		t.Errorf("GetSessionId = %d, want 999", v)
	}
}

func TestGetRpcId_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetRpcId(); v != 0 {
		t.Errorf("GetRpcId(nil) = %d, want 0", v)
	}
	m = &Message{RpcId: 0x12345678}
	if v := m.GetRpcId(); v != 0x12345678 {
		t.Errorf("GetRpcId = %d, want 0x12345678", v)
	}
}

func TestGetTraceIdHi_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetTraceIdHi(); v != 0 {
		t.Errorf("GetTraceIdHi(nil) = %d", v)
	}
	m = &Message{TraceIdHi: 1}
	if v := m.GetTraceIdHi(); v != 1 {
		t.Errorf("GetTraceIdHi = %d, want 1", v)
	}
}

func TestGetTraceIdLo_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetTraceIdLo(); v != 0 {
		t.Errorf("GetTraceIdLo(nil) = %d", v)
	}
	m = &Message{TraceIdLo: 2}
	if v := m.GetTraceIdLo(); v != 2 {
		t.Errorf("GetTraceIdLo = %d, want 2", v)
	}
}

func TestGetSpanId_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetSpanId(); v != 0 {
		t.Errorf("GetSpanId(nil) = %d", v)
	}
	m = &Message{SpanId: 3}
	if v := m.GetSpanId(); v != 3 {
		t.Errorf("GetSpanId = %d, want 3", v)
	}
}

func TestGetData_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetData(); v != nil {
		t.Errorf("GetData(nil) = %v, want nil", v)
	}
	m = &Message{Data: []byte("x")}
	if v := m.GetData(); !bytes.Equal(v, []byte("x")) {
		t.Errorf("GetData = %v, want [x]", v)
	}
}

func TestGetAuthIds_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetAuthIds(); v != nil {
		t.Errorf("GetAuthIds(nil) = %v, want nil", v)
	}
	m = &Message{AuthIds: []int64{1, 2}}
	if v := m.GetAuthIds(); len(v) != 2 || v[0] != 1 || v[1] != 2 {
		t.Errorf("GetAuthIds = %v, want [1,2]", v)
	}
}

func TestGetMsgId_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetMsgId(); v != 0 {
		t.Errorf("GetMsgId(nil) = %d", v)
	}
	m = &Message{MsgId: 7}
	if v := m.GetMsgId(); v != 7 {
		t.Errorf("GetMsgId = %d, want 7", v)
	}
}

func TestGetSrcActor_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetSrcActor(); v != 0 {
		t.Errorf("GetSrcActor(nil) = %d", v)
	}
	m = &Message{SrcActor: 8}
	if v := m.GetSrcActor(); v != 8 {
		t.Errorf("GetSrcActor = %d, want 8", v)
	}
}

func TestGetTarActor_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetTarActor(); v != 0 {
		t.Errorf("GetTarActor(nil) = %d", v)
	}
	m = &Message{TarActor: 9}
	if v := m.GetTarActor(); v != 9 {
		t.Errorf("GetTarActor = %d, want 9", v)
	}
}

func TestGetRefCount_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetRefCount(); v != 0 {
		t.Errorf("GetRefCount(nil) = %d", v)
	}
	m = &Message{RefCount: 10}
	if v := m.GetRefCount(); v != 10 {
		t.Errorf("GetRefCount = %d, want 10", v)
	}
}

func TestGetSeqId_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetSeqId(); v != 0 {
		t.Errorf("GetSeqId(nil) = %d", v)
	}
	m = &Message{SeqId: 11}
	if v := m.GetSeqId(); v != 11 {
		t.Errorf("GetSeqId = %d, want 11", v)
	}
}

func TestGetToClient_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetToClient(); v != false {
		t.Errorf("GetToClient(nil) = %v, want false", v)
	}
	m = &Message{ToClient: true}
	if v := m.GetToClient(); v != true {
		t.Errorf("GetToClient = %v, want true", v)
	}
}

func TestGetFromClient_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetFromClient(); v != false {
		t.Errorf("GetFromClient(nil) = %v, want false", v)
	}
	m = &Message{FromClient: true}
	if v := m.GetFromClient(); v != true {
		t.Errorf("GetFromClient = %v, want true", v)
	}
}

func TestGetIsResponse_NilAndPopulated(t *testing.T) {
	var m *Message
	if v := m.GetIsResponse(); v != false {
		t.Errorf("GetIsResponse(nil) = %v, want false", v)
	}
	m = &Message{IsResponse: true}
	if v := m.GetIsResponse(); v != true {
		t.Errorf("GetIsResponse = %v, want true", v)
	}
}

// TestEqualMessageVT_WithProtoInterface verifies EqualMessageVT works with proto.Message interface
func TestEqualMessageVT_WithProtoInterface(t *testing.T) {
	m := &Message{MsgId: 1}
	var iface proto.Message = m
	if !m.EqualMessageVT(iface) {
		t.Error("EqualMessageVT(proto.Message(*Message)) should be true when equal")
	}
}

// ============================================================
// MarshalVTStrict / MarshalToVTStrict / MarshalToSizedBufferVTStrict
// ============================================================

func TestMarshalVTStrict_Roundtrip(t *testing.T) {
	orig := &Message{MsgId: 42, AuthId: 123, Data: []byte("strict")}
	buf, err := orig.MarshalVTStrict()
	if err != nil {
		t.Fatalf("MarshalVTStrict: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Error("MarshalVTStrict roundtrip mismatch")
	}
}

func TestMarshalVTStrict_Nil(t *testing.T) {
	var m *Message
	buf, err := m.MarshalVTStrict()
	if err != nil {
		t.Fatalf("MarshalVTStrict(nil): %v", err)
	}
	if buf != nil {
		t.Error("MarshalVTStrict(nil) should return nil")
	}
}

func TestMarshalToVTStrict_Preallocated(t *testing.T) {
	m := &Message{MsgId: 1, TraceIdLo: 100, SpanId: 200}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToVTStrict(buf)
	if err != nil {
		t.Fatalf("MarshalToVTStrict: %v", err)
	}
	if n != size {
		t.Errorf("MarshalToVTStrict wrote %d, SizeVT=%d", n, size)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf[:n]); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !m.EqualVT(decoded) {
		t.Error("MarshalToVTStrict roundtrip mismatch")
	}
}

func TestMarshalToSizedBufferVTStrict(t *testing.T) {
	m := &Message{ToClient: true, FromClient: true, IsResponse: true}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBufferVTStrict(buf)
	if err != nil {
		t.Fatalf("MarshalToSizedBufferVTStrict: %v", err)
	}
	if n != size {
		t.Errorf("MarshalToSizedBufferVTStrict wrote %d, SizeVT=%d", n, size)
	}
}

// TestMarshalToSizedBufferVTStrict_FullMessage hits all field branches
func TestMarshalToSizedBufferVTStrict_FullMessage(t *testing.T) {
	m := &Message{
		AuthId: 1, SessionId: 2, RpcId: 3, TraceIdHi: 4, TraceIdLo: 5, SpanId: 6,
		MsgId: 7, SrcActor: 8, TarActor: 9, RefCount: 10, SeqId: 11,
		ToClient: true, FromClient: true, IsResponse: true,
		Data: []byte("data"), AuthIds: []int64{1, 2, 3},
	}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBufferVTStrict(buf)
	if err != nil {
		t.Fatalf("MarshalToSizedBufferVTStrict: %v", err)
	}
	if n != size {
		t.Errorf("wrote %d, SizeVT=%d", n, size)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !m.EqualVT(decoded) {
		t.Error("roundtrip mismatch")
	}
}

// ============================================================
// UnmarshalVTUnsafe
// ============================================================

func TestUnmarshalVTUnsafe_Roundtrip(t *testing.T) {
	orig := &Message{MsgId: 1, Data: []byte("unsafe")}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if decoded.MsgId != orig.MsgId {
		t.Errorf("MsgId: got %d want %d", decoded.MsgId, orig.MsgId)
	}
	if !bytes.Equal(decoded.Data, orig.Data) {
		t.Errorf("Data: got %q want %q", decoded.Data, orig.Data)
	}
}

func TestUnmarshalVTUnsafe_AllFields(t *testing.T) {
	orig := &Message{
		AuthId: 1, SessionId: 2, RpcId: 3, TraceIdHi: 4, TraceIdLo: 5, SpanId: 6,
		MsgId: 7, SrcActor: 8, TarActor: 9, RefCount: 10, SeqId: 11,
		ToClient: true, FromClient: true, IsResponse: true,
		Data: []byte("x"), AuthIds: []int64{1, 2},
	}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("UnmarshalVTUnsafe roundtrip mismatch")
	}
}

// ============================================================
// Unknown fields (exercise default/skippy path in UnmarshalVT)
// ============================================================

func TestUnmarshalVT_UnknownFields(t *testing.T) {
	// Valid message: AuthId=1 (tag 1, varint 1) = 0x08 0x01
	// Append unknown field: tag 50, wire type 2 (length-delimited), length 0
	// Tag 50: (50<<3)|2 = 402. Varint 402 = 0x92 0x03
	// Length 0 = 0x00
	withUnknown := []byte{0x08, 0x01, 0x92, 0x03, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(withUnknown); err != nil {
		t.Fatalf("UnmarshalVT with unknown fields: %v", err)
	}
	if m.AuthId != 1 {
		t.Errorf("AuthId: got %d want 1", m.AuthId)
	}
	// unknownFields should be populated (contains the skipped bytes)
	// Re-marshal and verify we can roundtrip (unknown fields preserved)
	buf2, err := m.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	m2 := &Message{}
	if err := m2.UnmarshalVT(buf2); err != nil {
		t.Fatalf("UnmarshalVT roundtrip: %v", err)
	}
	if !m.EqualVT(m2) {
		t.Error("Roundtrip with unknown fields should preserve equality")
	}
}

// ============================================================
// CloneVT with unknownFields
// ============================================================

func TestCloneVT_WithUnknownFields(t *testing.T) {
	withUnknown := []byte{0x08, 0x01, 0x92, 0x03, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(withUnknown); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	cloned := m.CloneVT()
	if cloned == nil {
		t.Fatal("CloneVT should not return nil")
	}
	if !m.EqualVT(cloned) {
		t.Error("Clone with unknown fields should EqualVT original")
	}
}

// ============================================================
// EqualVT with unknownFields (different unknownFields => not equal)
// ============================================================

func TestEqualVT_DifferentUnknownFields(t *testing.T) {
	// Two messages with same AuthId but different unknown field content
	a := &Message{AuthId: 1}
	b := &Message{AuthId: 1}
	if !a.EqualVT(b) {
		t.Error("Same values, no unknownFields: should be equal")
	}
	// Different unknown fields via UnmarshalVT - same known fields, different wire data
	wireA := []byte{0x08, 0x01, 0x92, 0x03, 0x00} // AuthId=1, unknown field 50
	wireB := []byte{0x08, 0x01, 0x98, 0x06, 0x00} // AuthId=1, unknown field 99
	a = &Message{}
	b = &Message{}
	if err := a.UnmarshalVT(wireA); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if err := b.UnmarshalVT(wireB); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if a.EqualVT(b) {
		t.Error("Same known fields but different unknownFields should not EqualVT")
	}
}

// ============================================================
// msg.pb.go Reset, ProtoReflect (indirect via Marshal)
// ============================================================

func TestMessage_Reset(t *testing.T) {
	m := &Message{MsgId: 42, AuthId: 123, Data: []byte("x")}
	m.Reset()
	if m.MsgId != 0 || m.AuthId != 0 || len(m.Data) != 0 {
		t.Errorf("Reset should zero fields: %+v", m)
	}
}

// TestMarshalToSizedBufferVTStrict_WithUnknownFields exercises unknownFields branch
func TestMarshalToSizedBufferVTStrict_WithUnknownFields(t *testing.T) {
	withUnknown := []byte{0x08, 0x01, 0x92, 0x03, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(withUnknown); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBufferVTStrict(buf)
	if err != nil {
		t.Fatalf("MarshalToSizedBufferVTStrict: %v", err)
	}
	if n != size {
		t.Errorf("wrote %d, SizeVT=%d", n, size)
	}
}

// TestUnmarshalVT_AuthIdsUnpacked exercises unpacked repeated encoding (wire type 0)
func TestUnmarshalVT_AuthIdsUnpacked(t *testing.T) {
	// AuthId=1 (tag 1), AuthIds=1 and 2 as unpacked (tag 6, wire 0, each value separate)
	// 0x08 0x01 = AuthId 1
	// 0x30 0x01 = tag 6 varint 1, 0x30 0x02 = tag 6 varint 2
	unpacked := []byte{0x08, 0x01, 0x30, 0x01, 0x30, 0x02}
	m := &Message{}
	if err := m.UnmarshalVT(unpacked); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if m.AuthId != 1 {
		t.Errorf("AuthId: got %d want 1", m.AuthId)
	}
	if len(m.AuthIds) != 2 || m.AuthIds[0] != 1 || m.AuthIds[1] != 2 {
		t.Errorf("AuthIds: got %v want [1,2]", m.AuthIds)
	}
}

// TestUnmarshalVTUnsafe_PackedAuthIds exercises packed AuthIds path
func TestUnmarshalVTUnsafe_PackedAuthIds(t *testing.T) {
	orig := &Message{AuthIds: []int64{100, 200, 300}}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if len(decoded.AuthIds) != 3 || decoded.AuthIds[0] != 100 || decoded.AuthIds[1] != 200 || decoded.AuthIds[2] != 300 {
		t.Errorf("AuthIds: got %v", decoded.AuthIds)
	}
}

// TestMessage_String exercises String() method
func TestMessage_String(t *testing.T) {
	m := &Message{MsgId: 1, AuthId: 123}
	s := m.String()
	if s == "" || len(s) < 2 {
		t.Errorf("String() should return non-empty: %q", s)
	}
}

// TestMessage_ProtoReflect exercises ProtoReflect
func TestMessage_ProtoReflect(t *testing.T) {
	m := &Message{MsgId: 1}
	r := m.ProtoReflect()
	if r == nil {
		t.Error("ProtoReflect should not return nil")
	}
}

// TestMessage_ProtoMessage exercises ProtoMessage (no-op marker method)
func TestMessage_ProtoMessage(t *testing.T) {
	m := &Message{MsgId: 1}
	m.ProtoMessage() // no-op, for coverage
}

// TestMessage_Descriptor exercises Descriptor (deprecated but in coverage)
func TestMessage_Descriptor(t *testing.T) {
	m := &Message{}
	b, idx := m.Descriptor()
	if len(b) == 0 || len(idx) == 0 {
		t.Error("Descriptor should return non-empty")
	}
	if idx[0] != 0 {
		t.Errorf("Descriptor index: got %v", idx)
	}
}

// ============================================================
// EqualVT - each field differs individually (push 70.7% → 90%+)
// ============================================================

func TestEqualVT_DifferentSessionId(t *testing.T) {
	a := &Message{SessionId: 1}
	b := &Message{SessionId: 2}
	if a.EqualVT(b) {
		t.Error("different SessionId should not EqualVT")
	}
}

func TestEqualVT_DifferentRpcId(t *testing.T) {
	a := &Message{RpcId: 1}
	b := &Message{RpcId: 2}
	if a.EqualVT(b) {
		t.Error("different RpcId should not EqualVT")
	}
}

func TestEqualVT_DifferentTraceIdHi(t *testing.T) {
	a := &Message{TraceIdHi: 1}
	b := &Message{TraceIdHi: 2}
	if a.EqualVT(b) {
		t.Error("different TraceIdHi should not EqualVT")
	}
}

func TestEqualVT_DifferentTraceIdLo(t *testing.T) {
	a := &Message{TraceIdLo: 1}
	b := &Message{TraceIdLo: 2}
	if a.EqualVT(b) {
		t.Error("different TraceIdLo should not EqualVT")
	}
}

func TestEqualVT_DifferentSpanId(t *testing.T) {
	a := &Message{SpanId: 1}
	b := &Message{SpanId: 2}
	if a.EqualVT(b) {
		t.Error("different SpanId should not EqualVT")
	}
}

func TestEqualVT_DifferentSrcActor(t *testing.T) {
	a := &Message{SrcActor: 1}
	b := &Message{SrcActor: 2}
	if a.EqualVT(b) {
		t.Error("different SrcActor should not EqualVT")
	}
}

func TestEqualVT_DifferentTarActor(t *testing.T) {
	a := &Message{TarActor: 1}
	b := &Message{TarActor: 2}
	if a.EqualVT(b) {
		t.Error("different TarActor should not EqualVT")
	}
}

func TestEqualVT_DifferentRefCount(t *testing.T) {
	a := &Message{RefCount: 1}
	b := &Message{RefCount: 2}
	if a.EqualVT(b) {
		t.Error("different RefCount should not EqualVT")
	}
}

func TestEqualVT_DifferentSeqId(t *testing.T) {
	a := &Message{SeqId: 1}
	b := &Message{SeqId: 2}
	if a.EqualVT(b) {
		t.Error("different SeqId should not EqualVT")
	}
}

func TestEqualVT_DifferentToClient(t *testing.T) {
	a := &Message{ToClient: false}
	b := &Message{ToClient: true}
	if a.EqualVT(b) {
		t.Error("different ToClient should not EqualVT")
	}
}

func TestEqualVT_DifferentFromClient(t *testing.T) {
	a := &Message{FromClient: false}
	b := &Message{FromClient: true}
	if a.EqualVT(b) {
		t.Error("different FromClient should not EqualVT")
	}
}

func TestEqualVT_DifferentIsResponse(t *testing.T) {
	a := &Message{IsResponse: false}
	b := &Message{IsResponse: true}
	if a.EqualVT(b) {
		t.Error("different IsResponse should not EqualVT")
	}
}

func TestEqualVT_AuthIdsNilVsNonNil(t *testing.T) {
	// a has nil/empty AuthIds, b has elements -> length differs
	a := &Message{}
	b := &Message{AuthIds: []int64{1}}
	if a.EqualVT(b) {
		t.Error("nil AuthIds vs [1] should not EqualVT (length differs)")
	}
	// Reverse: a has AuthIds, b has nil/empty
	a = &Message{AuthIds: []int64{1}}
	b = &Message{}
	if a.EqualVT(b) {
		t.Error("[1] AuthIds vs nil should not EqualVT")
	}
}

// ============================================================
// UnmarshalVT - field-type specific roundtrips
// ============================================================

func TestUnmarshalVT_Int64FieldsOnly(t *testing.T) {
	orig := &Message{AuthId: 123, SessionId: 456}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("Int64 roundtrip: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVT_Uint64FieldsOnly(t *testing.T) {
	orig := &Message{RpcId: 100, TraceIdHi: 200, TraceIdLo: 300, SpanId: 400}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("Uint64 roundtrip: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVT_Int32FieldsOnly(t *testing.T) {
	orig := &Message{MsgId: 10, SrcActor: 20, TarActor: 30, RefCount: 40}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("Int32 roundtrip: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVT_Uint32SeqIdOnly(t *testing.T) {
	orig := &Message{SeqId: 99}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if decoded.SeqId != 99 {
		t.Errorf("SeqId: got %d want 99", decoded.SeqId)
	}
}

func TestUnmarshalVT_BoolFieldsOnly(t *testing.T) {
	orig := &Message{ToClient: true, FromClient: true, IsResponse: false}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("Bool roundtrip: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVT_DataOnly(t *testing.T) {
	orig := &Message{Data: []byte("binary\x00\xff")}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !bytes.Equal(decoded.Data, orig.Data) {
		t.Errorf("Data: got %x want %x", decoded.Data, orig.Data)
	}
}

func TestUnmarshalVT_DataEmptyLength(t *testing.T) {
	// Data field (tag 5) with length 0: 0x2a 0x00
	wire := []byte{0x2a, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(wire); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if len(m.Data) != 0 {
		t.Errorf("Data len: got %d want 0", len(m.Data))
	}
}

func TestUnmarshalVT_EveryFieldSet(t *testing.T) {
	orig := &Message{
		AuthId: 1, SessionId: 2, RpcId: 3, TraceIdHi: 4, TraceIdLo: 5, SpanId: 6,
		MsgId: 7, SrcActor: 8, TarActor: 9, RefCount: 10, SeqId: 11,
		ToClient: true, FromClient: false, IsResponse: true,
		Data: []byte("payload"), AuthIds: []int64{1, 2, 3, 4, 5},
	}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVT(buf); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("Full roundtrip: orig=%+v decoded=%+v", orig, decoded)
	}
}

// ============================================================
// UnmarshalVTUnsafe - same field-type roundtrips
// ============================================================

func TestUnmarshalVTUnsafe_Int64FieldsOnly(t *testing.T) {
	orig := &Message{AuthId: 111, SessionId: 222}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("UnmarshalVTUnsafe Int64: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVTUnsafe_Uint64FieldsOnly(t *testing.T) {
	orig := &Message{RpcId: 1, TraceIdHi: 2, TraceIdLo: 3, SpanId: 4}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("UnmarshalVTUnsafe Uint64: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVTUnsafe_Int32FieldsOnly(t *testing.T) {
	orig := &Message{MsgId: 1, SrcActor: 2, TarActor: 3, RefCount: 4}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("UnmarshalVTUnsafe Int32: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVTUnsafe_BoolFieldsOnly(t *testing.T) {
	orig := &Message{ToClient: false, FromClient: true, IsResponse: true}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !orig.EqualVT(decoded) {
		t.Errorf("UnmarshalVTUnsafe Bool: orig=%+v decoded=%+v", orig, decoded)
	}
}

func TestUnmarshalVTUnsafe_DataOnly(t *testing.T) {
	orig := &Message{Data: []byte("unsafe data")}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !bytes.Equal(decoded.Data, orig.Data) {
		t.Errorf("UnmarshalVTUnsafe Data: got %q want %q", decoded.Data, orig.Data)
	}
}

func TestUnmarshalVTUnsafe_EmptyBuffer(t *testing.T) {
	decoded := &Message{}
	err := decoded.UnmarshalVTUnsafe(nil)
	if err != nil {
		t.Logf("UnmarshalVTUnsafe(nil): %v", err)
	}
	err = decoded.UnmarshalVTUnsafe([]byte{})
	if err != nil {
		t.Logf("UnmarshalVTUnsafe([]byte{}): %v", err)
	}
}

func TestUnmarshalVTUnsafe_UnknownFields(t *testing.T) {
	withUnknown := []byte{0x08, 0x01, 0x92, 0x03, 0x00}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(withUnknown); err != nil {
		t.Fatalf("UnmarshalVTUnsafe with unknown: %v", err)
	}
	if m.AuthId != 1 {
		t.Errorf("AuthId: got %d want 1", m.AuthId)
	}
}

func TestUnmarshalVTUnsafe_TruncatedData(t *testing.T) {
	valid := &Message{MsgId: 1, Data: []byte("x")}
	buf, _ := valid.MarshalVT()
	truncated := buf[:len(buf)/2]
	m := &Message{}
	err := m.UnmarshalVTUnsafe(truncated)
	if err == nil {
		t.Error("UnmarshalVTUnsafe truncated should error")
	}
}

func TestUnmarshalVTUnsafe_WrongWireType(t *testing.T) {
	invalid := []byte{0x0a, 0x00}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(invalid)
	if err == nil {
		t.Error("UnmarshalVTUnsafe wrong wire type should error")
	}
}

func TestUnmarshalVTUnsafe_AuthIdsUnpacked(t *testing.T) {
	unpacked := []byte{0x08, 0x01, 0x30, 0x01, 0x30, 0x02}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(unpacked); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if len(m.AuthIds) != 2 || m.AuthIds[0] != 1 || m.AuthIds[1] != 2 {
		t.Errorf("AuthIds: got %v want [1,2]", m.AuthIds)
	}
}

func TestUnmarshalVTUnsafe_IntOverflow(t *testing.T) {
	invalid := []byte{0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(invalid)
	if err == nil {
		t.Error("UnmarshalVTUnsafe oversized varint should error")
	}
}

func TestUnmarshalVT_IllegalTagZero(t *testing.T) {
	invalid := []byte{0x00, 0x00}
	m := &Message{}
	err := m.UnmarshalVT(invalid)
	if err == nil {
		t.Error("UnmarshalVT tag 0 should error")
	}
}

func TestUnmarshalVTUnsafe_IllegalTagZero(t *testing.T) {
	invalid := []byte{0x00, 0x00}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(invalid)
	if err == nil {
		t.Error("UnmarshalVTUnsafe tag 0 should error")
	}
}

func TestEqualVT_SameFullMessage(t *testing.T) {
	base := &Message{
		AuthId: 1, SessionId: 2, RpcId: 3, TraceIdHi: 4, TraceIdLo: 5, SpanId: 6,
		MsgId: 7, SrcActor: 8, TarActor: 9, RefCount: 10, SeqId: 11,
		ToClient: true, FromClient: false, IsResponse: true,
		Data: []byte("same"), AuthIds: []int64{1, 2, 3},
	}
	other := base.CloneVT()
	if !base.EqualVT(other) {
		t.Error("cloned full message should EqualVT original")
	}
}

func TestMarshalToSizedBufferVT_WithUnknownFields(t *testing.T) {
	withUnknown := []byte{0x08, 0x01, 0x92, 0x03, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(withUnknown); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	size := m.SizeVT()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(buf)
	if err != nil {
		t.Fatalf("MarshalToSizedBufferVT: %v", err)
	}
	if n != size {
		t.Errorf("wrote %d, SizeVT=%d", n, size)
	}
}

// ============================================================
// Targeted coverage: Release double-release (non-DEBUG path)
// ============================================================

func TestRelease_DoubleRelease_TriggersDoubleReleasePath(t *testing.T) {
	msg := GetMessage()
	if msg == nil {
		t.Fatal("GetMessage should not return nil")
	}
	msg.Release()
	// Second Release: refCount goes 1->0 on first, then we call again. refCount was 0,
	// AddInt32(-1) yields -1, so newRef < 0 → double-release path (logger.Error, Store 0).
	msg.Release() // should not panic; triggers double-release branch
}

// ============================================================
// Targeted coverage: Release with large Data nil'd on recycle
// ============================================================

func TestRelease_LargeData_NilOnRecycle(t *testing.T) {
	msg := GetMessage()
	if msg == nil {
		t.Fatal("GetMessage should not return nil")
	}
	// Pool gives Data with cap 256. Replace with buffer cap > 4096 so Release nil's it.
	msg.Data = make([]byte, 0, 5000)
	msg.Release()

	// Get another message; pool may return the same recycled instance
	msg2 := GetMessage()
	defer msg2.Release()
	if msg == msg2 {
		// Same instance recycled: Data must have been nil'd
		if msg2.Data != nil {
			t.Errorf("recycled message with large Data cap should have Data=nil, got cap=%d", cap(msg2.Data))
		}
	}
	// If different instance, we can't assert; test still exercises Release large-Data path
}

func TestRelease_LargeAuthIds_NilOnRecycle(t *testing.T) {
	msg := GetMessage()
	if msg == nil {
		t.Fatal("GetMessage should not return nil")
	}
	// Pool gives AuthIds with cap 4. Replace with cap > 64 so Release nil's it.
	msg.AuthIds = make([]int64, 0, 128)
	msg.Release()

	msg2 := GetMessage()
	defer msg2.Release()
	if msg == msg2 {
		if msg2.AuthIds != nil {
			t.Errorf("recycled message with large AuthIds cap should have AuthIds=nil, got cap=%d", cap(msg2.AuthIds))
		}
	}
}

// ============================================================
// Targeted coverage: EqualVT AuthIds element-by-element loop
// ============================================================

func TestEqualVT_AuthIdsSameLengthDifferentElementAtIndex(t *testing.T) {
	// Same length, first two elements match, third differs → hits loop iterations 0,1,2
	a := &Message{AuthIds: []int64{1, 2, 3}}
	b := &Message{AuthIds: []int64{1, 2, 99}}
	if a.EqualVT(b) {
		t.Error("AuthIds [1,2,3] vs [1,2,99] should not EqualVT")
	}
	// Mismatch at index 0
	a = &Message{AuthIds: []int64{1, 2}}
	b = &Message{AuthIds: []int64{99, 2}}
	if a.EqualVT(b) {
		t.Error("AuthIds [1,2] vs [99,2] should not EqualVT")
	}
}

// ============================================================
// Targeted coverage: UnmarshalVTUnsafe branches
// ============================================================

func TestUnmarshalVTUnsafe_OnlyBoolFields(t *testing.T) {
	orig := &Message{ToClient: true, FromClient: false, IsResponse: true}
	buf, err := orig.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if decoded.ToClient != true || decoded.FromClient != false || decoded.IsResponse != true {
		t.Errorf("bool fields: got ToClient=%v FromClient=%v IsResponse=%v",
			decoded.ToClient, decoded.FromClient, decoded.IsResponse)
	}
}

func TestUnmarshalVTUnsafe_OnlyAuthIdsPacked(t *testing.T) {
	orig := &Message{AuthIds: []int64{11, 22, 33, 44}}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if len(decoded.AuthIds) != 4 || decoded.AuthIds[0] != 11 || decoded.AuthIds[3] != 44 {
		t.Errorf("AuthIds: got %v want [11,22,33,44]", decoded.AuthIds)
	}
}

func TestUnmarshalVTUnsafe_DataNonZeroLength(t *testing.T) {
	orig := &Message{Data: []byte("payload")}
	buf, _ := orig.MarshalVT()
	decoded := &Message{}
	if err := decoded.UnmarshalVTUnsafe(buf); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if !bytes.Equal(decoded.Data, []byte("payload")) {
		t.Errorf("Data: got %q want 'payload'", decoded.Data)
	}
}

// ============================================================
// UnmarshalVT - unknown fields with different wire types (skip paths)
// ============================================================

func TestUnmarshalVT_UnknownField_Varint(t *testing.T) {
	// Field 99 with varint (wire type 0): tag = (99<<3)|0 = 792 = 0x98 0x06, value 42 = 0x2A
	data := []byte{0x98, 0x06, 0x2A}
	m := &Message{}
	if err := m.UnmarshalVT(data); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
}

func TestUnmarshalVT_UnknownField_64bit(t *testing.T) {
	// Field 99 with 64-bit (wire type 1): tag = (99<<3)|1 = 793 = 0x99 0x06
	data := []byte{0x99, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(data); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
}

func TestUnmarshalVT_UnknownField_LengthDelimited(t *testing.T) {
	// Field 99 with length-delimited (wire type 2): tag = (99<<3)|2 = 794 = 0x9A 0x06
	// Length 3, data "abc"
	data := []byte{0x9A, 0x06, 0x03, 0x61, 0x62, 0x63}
	m := &Message{}
	if err := m.UnmarshalVT(data); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
}

func TestUnmarshalVT_UnknownField_32bit(t *testing.T) {
	// Field 99 with 32-bit (wire type 5): tag = (99<<3)|5 = 797 = 0x9D 0x06
	data := []byte{0x9D, 0x06, 0x00, 0x00, 0x00, 0x00}
	m := &Message{}
	if err := m.UnmarshalVT(data); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
}

func TestUnmarshalVT_WrongWireType_AllFields(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"field1_wrong_wt", []byte{0x0A, 0x01, 0x00}},                                     // AuthId expects varint, got length-delimited
		{"field5_wrong_wt", []byte{0x28, 0x00}},                                           // Data expects length-delimited, got varint
		{"field7_wrong_wt", []byte{0x39, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}, // MsgId expects varint, got 64-bit
		{"field13_wrong_wt", []byte{0x6A, 0x01, 0x00}},                                    // ToClient expects varint, got length-delimited
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{}
			err := m.UnmarshalVT(tt.data)
			if err == nil {
				t.Fatal("expected error for wrong wire type")
			}
		})
	}
}

// ============================================================
// UnmarshalVTUnsafe - same unknown/wrong wire type coverage
// ============================================================

func TestUnmarshalVTUnsafe_UnknownField_Varint(t *testing.T) {
	data := []byte{0x98, 0x06, 0x2A}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(data); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
}

func TestUnmarshalVTUnsafe_UnknownField_64bit(t *testing.T) {
	data := []byte{0x99, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(data); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
}

func TestUnmarshalVTUnsafe_UnknownField_LengthDelimited(t *testing.T) {
	data := []byte{0x9A, 0x06, 0x03, 0x61, 0x62, 0x63}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(data); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
}

func TestUnmarshalVTUnsafe_UnknownField_32bit(t *testing.T) {
	data := []byte{0x9D, 0x06, 0x00, 0x00, 0x00, 0x00}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(data); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
}

func TestUnmarshalVTUnsafe_WrongWireType_AllFields(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"field1_wrong_wt", []byte{0x0A, 0x01, 0x00}},
		{"field5_wrong_wt", []byte{0x28, 0x00}},
		{"field7_wrong_wt", []byte{0x39, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"field13_wrong_wt", []byte{0x6A, 0x01, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{}
			err := m.UnmarshalVTUnsafe(tt.data)
			if err == nil {
				t.Fatal("expected error for wrong wire type")
			}
		})
	}
}

// ============================================================
// Wrong wire type for more fields (SessionId, RpcId, Data, AuthIds, etc.)
// ============================================================

func TestUnmarshalVT_WrongWireType_MoreFields(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"field2_wrong_wt", []byte{0x12, 0x01, 0x00}},                                     // SessionId: varint expected, got length-delimited
		{"field3_wrong_wt", []byte{0x1A, 0x01, 0x00}},                                     // RpcId
		{"field4_wrong_wt", []byte{0x22, 0x01, 0x00}},                                     // TraceIdHi
		{"field6_wrong_wt", []byte{0x31, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}, // AuthIds: wire 0 or 2 only, wire 1 (64-bit) invalid
		{"field8_wrong_wt", []byte{0x42, 0x01, 0x00}},                                     // SrcActor: varint expected, got length-delimited
		{"field9_wrong_wt", []byte{0x4A, 0x01, 0x00}},                                     // TarActor
		{"field11_wrong_wt", []byte{0x5A, 0x01, 0x00}},                                    // RefCount
		{"field12_wrong_wt", []byte{0x62, 0x01, 0x00}},                                    // SeqId
		{"field14_wrong_wt", []byte{0x72, 0x01, 0x00}},                                    // FromClient
		{"field15_wrong_wt", []byte{0x7A, 0x01, 0x00}},                                    // IsResponse
		{"field16_wrong_wt", []byte{0x82, 0x01, 0x01, 0x00}},                              // TraceIdLo (tag 0x80 0x02 for field 16)
		{"field17_wrong_wt", []byte{0x8A, 0x01, 0x01, 0x00}},                              // SpanId
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{}
			err := m.UnmarshalVT(tt.data)
			if err == nil {
				t.Fatalf("expected error for wrong wire type: %s", tt.name)
			}
		})
	}
}

func TestUnmarshalVTUnsafe_WrongWireType_MoreFields(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"field2_wrong_wt", []byte{0x12, 0x01, 0x00}},
		{"field3_wrong_wt", []byte{0x1A, 0x01, 0x00}},
		{"field4_wrong_wt", []byte{0x22, 0x01, 0x00}},
		{"field6_wrong_wt", []byte{0x31, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"field8_wrong_wt", []byte{0x42, 0x01, 0x00}},
		{"field9_wrong_wt", []byte{0x4A, 0x01, 0x00}},
		{"field11_wrong_wt", []byte{0x5A, 0x01, 0x00}},
		{"field12_wrong_wt", []byte{0x62, 0x01, 0x00}},
		{"field14_wrong_wt", []byte{0x72, 0x01, 0x00}},
		{"field15_wrong_wt", []byte{0x7A, 0x01, 0x00}},
		{"field16_wrong_wt", []byte{0x82, 0x01, 0x01, 0x00}},
		{"field17_wrong_wt", []byte{0x8A, 0x01, 0x01, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{}
			err := m.UnmarshalVTUnsafe(tt.data)
			if err == nil {
				t.Fatalf("expected error for wrong wire type: %s", tt.name)
			}
		})
	}
}

// ============================================================
// UnmarshalVT - additional edge cases (ErrUnexpectedEOF, tag overflow)
// ============================================================

func TestUnmarshalVT_DataLengthExceedsBuffer(t *testing.T) {
	// Field 5 (Data), wire 2: tag 0x2A, length 1000 (0xE8 0x07), but buffer ends - triggers ErrUnexpectedEOF
	data := []byte{0x2A, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVT(data)
	if err == nil {
		t.Error("UnmarshalVT data length exceeding buffer should error")
	}
}

func TestUnmarshalVTUnsafe_DataLengthExceedsBuffer(t *testing.T) {
	data := []byte{0x2A, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(data)
	if err == nil {
		t.Error("UnmarshalVTUnsafe data length exceeding buffer should error")
	}
}

func TestUnmarshalVT_TagVarintOverflow(t *testing.T) {
	// 10 bytes of 0xFF as tag - varint overflow (shift >= 64)
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	m := &Message{}
	err := m.UnmarshalVT(data)
	if err == nil {
		t.Error("UnmarshalVT tag varint overflow should error")
	}
}

func TestUnmarshalVTUnsafe_TagVarintOverflow(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(data)
	if err == nil {
		t.Error("UnmarshalVTUnsafe tag varint overflow should error")
	}
}

func TestUnmarshalVT_PackedAuthIdsLengthExceedsBuffer(t *testing.T) {
	// Field 6 (AuthIds) packed: tag 0x32, length 1000, but only 3 bytes total
	data := []byte{0x32, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVT(data)
	if err == nil {
		t.Error("UnmarshalVT packed AuthIds length exceeding buffer should error")
	}
}

func TestUnmarshalVTUnsafe_PackedAuthIdsLengthExceedsBuffer(t *testing.T) {
	data := []byte{0x32, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(data)
	if err == nil {
		t.Error("UnmarshalVTUnsafe packed AuthIds length exceeding buffer should error")
	}
}

// ============================================================
// UnmarshalVT - known + unknown field mix (skip then parse)
// ============================================================

func TestUnmarshalVT_KnownThenUnknownThenKnown(t *testing.T) {
	// AuthId=1, unknown field 99 (varint 42), MsgId=7
	data := []byte{0x08, 0x01, 0x98, 0x06, 0x2A, 0x38, 0x07}
	m := &Message{}
	if err := m.UnmarshalVT(data); err != nil {
		t.Fatalf("UnmarshalVT: %v", err)
	}
	if m.AuthId != 1 || m.MsgId != 7 {
		t.Errorf("got AuthId=%d MsgId=%d want 1, 7", m.AuthId, m.MsgId)
	}
}

func TestUnmarshalVTUnsafe_KnownThenUnknownThenKnown(t *testing.T) {
	data := []byte{0x08, 0x01, 0x98, 0x06, 0x2A, 0x38, 0x07}
	m := &Message{}
	if err := m.UnmarshalVTUnsafe(data); err != nil {
		t.Fatalf("UnmarshalVTUnsafe: %v", err)
	}
	if m.AuthId != 1 || m.MsgId != 7 {
		t.Errorf("got AuthId=%d MsgId=%d want 1, 7", m.AuthId, m.MsgId)
	}
}

// Unknown field with length-delimited value where length exceeds buffer - triggers Skip error
func TestUnmarshalVT_UnknownFieldLengthExceedsBuffer(t *testing.T) {
	// Field 99, wire 2, length 1000, but no data bytes
	data := []byte{0x9A, 0x06, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVT(data)
	if err == nil {
		t.Error("UnmarshalVT unknown field with length exceeding buffer should error")
	}
}

func TestUnmarshalVTUnsafe_UnknownFieldLengthExceedsBuffer(t *testing.T) {
	data := []byte{0x9A, 0x06, 0xE8, 0x07}
	m := &Message{}
	err := m.UnmarshalVTUnsafe(data)
	if err == nil {
		t.Error("UnmarshalVTUnsafe unknown field with length exceeding buffer should error")
	}
}
