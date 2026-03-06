package zserialize

import (
	"testing"

	"github.com/aiyang-zh/zhenyi-core/zmodel"
)

// ============================================================
// JSON (Sonic)
// ============================================================

func TestMarshalJson_DeSerialize(t *testing.T) {
	data := createTestData()

	buf, err := MarshalJson(data)
	if err != nil {
		t.Fatalf("MarshalJson: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("MarshalJson: empty output")
	}

	var result BenchmarkData
	if err := UnmarshalJson(buf, &result); err != nil {
		t.Fatalf("UnmarshalJson: %v", err)
	}

	if result.Id != data.Id {
		t.Errorf("Id: got %d, want %d", result.Id, data.Id)
	}
	if result.Name != data.Name {
		t.Errorf("Name: got %q, want %q", result.Name, data.Name)
	}
	if result.Email != data.Email {
		t.Errorf("Email: got %q, want %q", result.Email, data.Email)
	}
	if result.Age != data.Age {
		t.Errorf("Age: got %d, want %d", result.Age, data.Age)
	}
	if result.Active != data.Active {
		t.Errorf("Active: got %v, want %v", result.Active, data.Active)
	}
	if result.Score != data.Score {
		t.Errorf("Score: got %f, want %f", result.Score, data.Score)
	}
	if len(result.Tags) != len(data.Tags) {
		t.Fatalf("Tags len: got %d, want %d", len(result.Tags), len(data.Tags))
	}
	for i := range data.Tags {
		if result.Tags[i] != data.Tags[i] {
			t.Errorf("Tags[%d]: got %q, want %q", i, result.Tags[i], data.Tags[i])
		}
	}
	if len(result.Items) != len(data.Items) {
		t.Fatalf("Items len: got %d, want %d", len(result.Items), len(data.Items))
	}
	for i := range data.Items {
		if result.Items[i] != data.Items[i] {
			t.Errorf("Items[%d]: got %+v, want %+v", i, result.Items[i], data.Items[i])
		}
	}
}

func TestMarshalJson_EmptyStruct(t *testing.T) {
	data := &BenchmarkData{}
	buf, err := MarshalJson(data)
	if err != nil {
		t.Fatalf("serialize empty: %v", err)
	}
	var result BenchmarkData
	if err := UnmarshalJson(buf, &result); err != nil {
		t.Fatalf("deserialize empty: %v", err)
	}
	if result.Id != 0 || result.Name != "" {
		t.Error("empty struct should have zero values")
	}
}

func TestUnmarshalJson_InvalidJSON(t *testing.T) {
	err := UnmarshalJson([]byte("{invalid"), &BenchmarkData{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMarshalJson_NilSlice(t *testing.T) {
	data := &BenchmarkData{Id: 1}
	buf, err := MarshalJson(data)
	if err != nil {
		t.Fatalf("serialize nil slice: %v", err)
	}
	var result BenchmarkData
	if err := UnmarshalJson(buf, &result); err != nil {
		t.Fatalf("deserialize nil slice: %v", err)
	}
	if result.Id != 1 {
		t.Errorf("Id: got %d, want 1", result.Id)
	}
}

// ============================================================
// MsgPack
// ============================================================

func TestMarshalMsgPack_DeSerialize(t *testing.T) {
	data := createTestData()

	buf, err := MarshalMsgPack(data)
	if err != nil {
		t.Fatalf("MarshalMsgPack: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("MarshalMsgPack: empty output")
	}

	var result BenchmarkData
	if err := UnmarshalMsgPack(buf, &result); err != nil {
		t.Fatalf("UnmarshalMsgPack: %v", err)
	}

	if result.Id != data.Id {
		t.Errorf("Id: got %d, want %d", result.Id, data.Id)
	}
	if result.Name != data.Name {
		t.Errorf("Name: got %q, want %q", result.Name, data.Name)
	}
	if result.Active != data.Active {
		t.Errorf("Active: got %v, want %v", result.Active, data.Active)
	}
	if len(result.Tags) != len(data.Tags) {
		t.Fatalf("Tags len: got %d, want %d", len(result.Tags), len(data.Tags))
	}
	if len(result.Items) != len(data.Items) {
		t.Fatalf("Items len: got %d, want %d", len(result.Items), len(data.Items))
	}
}

func TestMarshalMsgPack_EmptyStruct(t *testing.T) {
	data := &BenchmarkData{}
	buf, err := MarshalMsgPack(data)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	var result BenchmarkData
	if err := UnmarshalMsgPack(buf, &result); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
}

func TestUnmarshalMsgPack_InvalidData(t *testing.T) {
	err := UnmarshalMsgPack([]byte{0xFF, 0xFE}, &BenchmarkData{})
	if err == nil {
		t.Fatal("expected error for invalid msgpack data")
	}
}

// ============================================================
// JSON vs MsgPack 一致性
// ============================================================

func TestCrossFormat_Consistency(t *testing.T) {
	data := createTestData()

	// 序列化 JSON
	jsonBuf, _ := MarshalJson(data)
	var fromJson BenchmarkData
	_ = UnmarshalJson(jsonBuf, &fromJson)

	// 序列化 MsgPack
	mpBuf, _ := MarshalMsgPack(data)
	var fromMp BenchmarkData
	_ = UnmarshalMsgPack(mpBuf, &fromMp)

	// 两种格式反序列化结果应一致
	if fromJson.Id != fromMp.Id || fromJson.Name != fromMp.Name || fromJson.Score != fromMp.Score {
		t.Errorf("cross-format inconsistency: JSON=%+v, MsgPack=%+v", fromJson, fromMp)
	}
}

// ============================================================
// 并发安全（池化的 encoder/decoder）
// ============================================================

func TestMsgPack_Concurrent(t *testing.T) {
	data := createTestData()
	done := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func() {
			buf, err := MarshalMsgPack(data)
			if err != nil {
				done <- err
				return
			}
			var result BenchmarkData
			done <- UnmarshalMsgPack(buf, &result)
		}()
	}

	for i := 0; i < 100; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent msgpack: %v", err)
		}
	}
}

func TestJson_Concurrent(t *testing.T) {
	data := createTestData()
	done := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func() {
			buf, err := MarshalJson(data)
			if err != nil {
				done <- err
				return
			}
			var result BenchmarkData
			done <- UnmarshalJson(buf, &result)
		}()
	}

	for i := 0; i < 100; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent json: %v", err)
		}
	}
}

// ============================================================
// Protobuf
// ============================================================

func TestMarshalProto_DeSerialize(t *testing.T) {
	msg := &zmodel.Message{
		AuthId:    12345,
		SessionId: 67890,
		MsgId:     100,
		SrcActor:  1,
		TarActor:  2,
		Data:      []byte("hello protobuf"),
	}

	buf, err := MarshalProto(msg)
	if err != nil {
		t.Fatalf("MarshalProto: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("MarshalProto: empty output")
	}

	var result zmodel.Message
	if err := UnmarshalProto(buf, &result); err != nil {
		t.Fatalf("UnmarshalProto: %v", err)
	}

	if result.AuthId != msg.AuthId {
		t.Errorf("AuthId: got %d, want %d", result.AuthId, msg.AuthId)
	}
	if result.SessionId != msg.SessionId {
		t.Errorf("SessionId: got %d, want %d", result.SessionId, msg.SessionId)
	}
	if result.MsgId != msg.MsgId {
		t.Errorf("MsgId: got %d, want %d", result.MsgId, msg.MsgId)
	}
	if string(result.Data) != string(msg.Data) {
		t.Errorf("Data: got %q, want %q", result.Data, msg.Data)
	}
}

func TestMarshalProto_EmptyMessage(t *testing.T) {
	msg := &zmodel.Message{}
	buf, err := MarshalProto(msg)
	if err != nil {
		t.Fatalf("MarshalProto empty: %v", err)
	}

	var result zmodel.Message
	if err := UnmarshalProto(buf, &result); err != nil {
		t.Fatalf("UnmarshalProto empty: %v", err)
	}
	if result.AuthId != 0 || result.MsgId != 0 {
		t.Error("empty message should have zero values")
	}
}

func TestUnmarshalProto_InvalidData(t *testing.T) {
	err := UnmarshalProto([]byte{0xFF, 0xFF, 0xFF}, &zmodel.Message{})
	if err == nil {
		t.Fatal("expected error for invalid protobuf data")
	}
}

func TestMarshalProto_WithAuthIds(t *testing.T) {
	msg := &zmodel.Message{
		AuthId:  1,
		MsgId:   200,
		AuthIds: []int64{100, 200, 300, 400, 500},
		Data:    []byte("broadcast"),
	}

	buf, err := MarshalProto(msg)
	if err != nil {
		t.Fatalf("MarshalProto: %v", err)
	}

	var result zmodel.Message
	if err := UnmarshalProto(buf, &result); err != nil {
		t.Fatalf("UnmarshalProto: %v", err)
	}

	if len(result.AuthIds) != 5 {
		t.Fatalf("AuthIds len: got %d, want 5", len(result.AuthIds))
	}
	for i, id := range result.AuthIds {
		if id != msg.AuthIds[i] {
			t.Errorf("AuthIds[%d]: got %d, want %d", i, id, msg.AuthIds[i])
		}
	}
}

// 测试数据结构
type BenchmarkData struct {
	Id       int64             `json:"id" msgpack:"id"`
	Name     string            `json:"name" msgpack:"name"`
	Email    string            `json:"email" msgpack:"email"`
	Age      int32             `json:"age" msgpack:"age"`
	Active   bool              `json:"active" msgpack:"active"`
	Score    float64           `json:"score" msgpack:"score"`
	Tags     []string          `json:"tags" msgpack:"tags"`
	Metadata map[string]string `json:"metadata" msgpack:"metadata"`
	Items    []Item            `json:"items" msgpack:"items"`
}

type Item struct {
	ItemId   int64   `json:"item_id" msgpack:"item_id"`
	Quantity int32   `json:"quantity" msgpack:"quantity"`
	Price    float64 `json:"price" msgpack:"price"`
}

// 创建测试数据
func createTestData() *BenchmarkData {
	return &BenchmarkData{
		Id:     123456789,
		Name:   "Test User",
		Email:  "test@example.com",
		Age:    25,
		Active: true,
		Score:  98.5,
		Tags:   []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		Items: []Item{
			{ItemId: 1, Quantity: 10, Price: 99.99},
			{ItemId: 2, Quantity: 5, Price: 149.99},
			{ItemId: 3, Quantity: 3, Price: 299.99},
		},
	}
}

// ==================== JSON (Sonic) 性能测试 ====================

func BenchmarkJSON_Serialize(b *testing.B) {
	data := createTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := MarshalJson(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSON_Deserialize(b *testing.B) {
	data := createTestData()
	serialized, _ := MarshalJson(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := &BenchmarkData{}
		err := UnmarshalJson(serialized, result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSON_RoundTrip(b *testing.B) {
	data := createTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		serialized, err := MarshalJson(data)
		if err != nil {
			b.Fatal(err)
		}
		result := &BenchmarkData{}
		err = UnmarshalJson(serialized, result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== MsgPack 性能测试 ====================

func BenchmarkMsgPack_Serialize(b *testing.B) {
	data := createTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := MarshalMsgPack(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMsgPack_Deserialize(b *testing.B) {
	data := createTestData()
	serialized, _ := MarshalMsgPack(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := &BenchmarkData{}
		err := UnmarshalMsgPack(serialized, result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMsgPack_RoundTrip(b *testing.B) {
	data := createTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		serialized, err := MarshalMsgPack(data)
		if err != nil {
			b.Fatal(err)
		}
		result := &BenchmarkData{}
		err = UnmarshalMsgPack(serialized, result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== Protobuf 性能测试 ====================

func createProtoTestMessage() *zmodel.Message {
	return &zmodel.Message{
		AuthId:    123456789,
		SessionId: 987654321,
		RpcId:     1111222233334444,
		MsgId:     500,
		SrcActor:  10,
		TarActor:  20,
		Data:      []byte("benchmark protobuf data payload"),
		AuthIds:   []int64{100, 200, 300, 400, 500},
	}
}

func BenchmarkProto_Serialize(b *testing.B) {
	msg := createProtoTestMessage()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := MarshalProto(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProto_Deserialize(b *testing.B) {
	msg := createProtoTestMessage()
	serialized, _ := MarshalProto(msg)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := &zmodel.Message{}
		if err := UnmarshalProto(serialized, result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProto_RoundTrip(b *testing.B) {
	msg := createProtoTestMessage()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf, err := MarshalProto(msg)
		if err != nil {
			b.Fatal(err)
		}
		result := &zmodel.Message{}
		if err := UnmarshalProto(buf, result); err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== 数据大小对比测试 ====================

func TestSerializationSize(t *testing.T) {
	data := createTestData()

	jsonData, _ := MarshalJson(data)
	msgpackData, _ := MarshalMsgPack(data)

	t.Logf("JSON 大小:    %d bytes", len(jsonData))
	t.Logf("MsgPack 大小: %d bytes", len(msgpackData))
	t.Logf("压缩率: %.2f%%", float64(len(msgpackData))/float64(len(jsonData))*100)

	// Protobuf 大小对比
	protoMsg := createProtoTestMessage()
	protoData, _ := MarshalProto(protoMsg)

	t.Logf("Protobuf 标准序列化大小: %d bytes", len(protoData))
}
