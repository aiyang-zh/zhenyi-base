package zmodel

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-core/zlog"
)

func init() {
	zlog.NewDefaultLogger()
}

// ============================================================
// Message Pool — GetMessage / Retain / Release
// ============================================================

func TestGetMessage_InitialState(t *testing.T) {
	msg := GetMessage()
	defer msg.Release()

	if msg == nil {
		t.Fatal("GetMessage returned nil")
	}
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1, got %d", rc)
	}
	if msg.MsgId != 0 {
		t.Fatalf("expected MsgId=0 after pool reset, got %d", msg.MsgId)
	}
	if len(msg.Data) != 0 {
		t.Fatalf("expected empty Data, got len=%d", len(msg.Data))
	}
	if len(msg.AuthIds) != 0 {
		t.Fatalf("expected empty AuthIds, got len=%d", len(msg.AuthIds))
	}
}

func TestRetain_Nil(t *testing.T) {
	var m *Message
	got := m.Retain()
	if got != nil {
		t.Fatal("Retain(nil) should return nil")
	}
}

func TestRetain_Normal(t *testing.T) {
	msg := GetMessage()
	defer msg.Release()

	ret := msg.Retain()
	if ret != msg {
		t.Fatal("Retain should return same pointer")
	}
	if rc := msg.LoadRefCount(); rc != 2 {
		t.Fatalf("expected RefCount=2 after Retain, got %d", rc)
	}
	msg.Release() // balance the extra Retain
}

func TestRetain_Multiple(t *testing.T) {
	msg := GetMessage()
	msg.Retain()
	msg.Retain()
	if rc := msg.LoadRefCount(); rc != 3 {
		t.Fatalf("expected RefCount=3, got %d", rc)
	}
	msg.Release()
	msg.Release()
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1, got %d", rc)
	}
	msg.Release()
}

func TestRelease_Nil(t *testing.T) {
	var m *Message
	m.Release() // should not panic
}

func TestRelease_BackToPool(t *testing.T) {
	msg := GetMessage()
	msg.MsgId = 42
	msg.Release()

	msg2 := GetMessage()
	defer msg2.Release()
	if msg2.MsgId != 0 {
		t.Fatal("pooled message should have MsgId=0 after PoolReset")
	}
}

func TestRelease_LargeBufferCleanup(t *testing.T) {
	msg := GetMessage()
	msg.Data = make([]byte, 5000)
	msg.AuthIds = make([]int64, 100)
	msg.Release()

	msg2 := GetMessage()
	defer msg2.Release()
	// After release of large buffers, they should be nil'd and re-allocated from pool
	// The pool creates with cap=256 / cap=4 by default
	if cap(msg2.Data) > 5000 {
		t.Fatal("expected large Data buffer to be released")
	}
}

func TestRelease_DoubleRelease_NosPanic(t *testing.T) {
	msg := GetMessage()
	msg.Release()
	// Second release should not panic in production mode (DEBUG_LIFECYCLE=false)
	msg.Release()
}

func TestMustRelease_Nil(t *testing.T) {
	var m *Message
	m.MustRelease() // should not panic
}

func TestMustRelease_Normal(t *testing.T) {
	msg := GetMessage()
	msg.MustRelease()
	// should behave same as Release
}

func TestLoadRefCount_Nil(t *testing.T) {
	var m *Message
	if rc := m.LoadRefCount(); rc != 0 {
		t.Fatalf("expected LoadRefCount(nil)=0, got %d", rc)
	}
}

func TestLoadRefCount_AfterRetainRelease(t *testing.T) {
	msg := GetMessage()
	if msg.LoadRefCount() != 1 {
		t.Fatal("expected RefCount=1")
	}
	msg.Retain()
	if msg.LoadRefCount() != 2 {
		t.Fatal("expected RefCount=2")
	}
	msg.Release()
	if msg.LoadRefCount() != 1 {
		t.Fatal("expected RefCount=1")
	}
	msg.Release()
}

// ============================================================
// Message Pool — Concurrent Safety
// ============================================================

func TestMessagePool_ConcurrentGetRelease(t *testing.T) {
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				msg := GetMessage()
				msg.MsgId = int32(i)
				msg.Data = append(msg.Data, byte(i))
				msg.Release()
			}
		}()
	}
	wg.Wait()
}

func TestMessagePool_RetainReleaseConcurrent(t *testing.T) {
	msg := GetMessage()

	const retainers = 20
	var wg sync.WaitGroup
	wg.Add(retainers)
	for i := 0; i < retainers; i++ {
		msg.Retain()
	}
	for i := 0; i < retainers; i++ {
		go func() {
			defer wg.Done()
			msg.Release()
		}()
	}
	wg.Wait()
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1 after concurrent releases, got %d", rc)
	}
	msg.Release()
}

// ============================================================
// ActorConfig
// ============================================================

func TestActorConfig_GetTopic(t *testing.T) {
	cfg := ActorConfig{Id: 100, ActorType: 2, Index: 3}
	want := "topic_2_3_100"
	if got := cfg.GetTopic(); got != want {
		t.Fatalf("GetTopic()=%q, want %q", got, want)
	}
}

func TestActorConfig_GetNameTopic(t *testing.T) {
	cfg := ActorConfig{ActorType: 5}
	want := "topic_name_5"
	if got := cfg.GetNameTopic(); got != want {
		t.Fatalf("GetNameTopic()=%q, want %q", got, want)
	}
}

func TestActorConfig_GetActorId(t *testing.T) {
	cfg := ActorConfig{Id: 42}
	if cfg.GetActorId() != 42 {
		t.Fatalf("expected 42, got %d", cfg.GetActorId())
	}
}

func TestActorConfig_GetActorType(t *testing.T) {
	cfg := ActorConfig{ActorType: 7}
	if cfg.GetActorType() != 7 {
		t.Fatalf("expected 7, got %d", cfg.GetActorType())
	}
}

// ============================================================
// ActorModeConfig
// ============================================================

func TestActorModeConfig_IsSequential(t *testing.T) {
	tests := []struct {
		mode int
		want bool
	}{
		{0, true}, {1, false}, {2, false},
	}
	for _, tt := range tests {
		c := ActorModeConfig{Mode: tt.mode}
		if got := c.IsSequential(); got != tt.want {
			t.Errorf("Mode=%d: IsSequential()=%v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestActorModeConfig_IsConcurrent(t *testing.T) {
	tests := []struct {
		mode int
		want bool
	}{
		{0, false}, {1, true}, {2, false},
	}
	for _, tt := range tests {
		c := ActorModeConfig{Mode: tt.mode}
		if got := c.IsConcurrent(); got != tt.want {
			t.Errorf("Mode=%d: IsConcurrent()=%v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestActorModeConfig_GetPoolSize(t *testing.T) {
	if (ActorModeConfig{}).GetPoolSize() != 100 {
		t.Fatal("default pool size should be 100")
	}
	if (ActorModeConfig{ConcurrentPoolSize: -1}).GetPoolSize() != 100 {
		t.Fatal("negative pool size should default to 100")
	}
	if (ActorModeConfig{ConcurrentPoolSize: 200}).GetPoolSize() != 200 {
		t.Fatal("explicit pool size should be returned")
	}
}

func TestActorModeConfig_GetMaxBatch(t *testing.T) {
	if (ActorModeConfig{}).GetMaxBatch() != 50 {
		t.Fatal("default max batch should be 50")
	}
	if (ActorModeConfig{ConcurrentMaxBatch: -5}).GetMaxBatch() != 50 {
		t.Fatal("negative max batch should default to 50")
	}
	if (ActorModeConfig{ConcurrentMaxBatch: 300}).GetMaxBatch() != 300 {
		t.Fatal("explicit max batch should be returned")
	}
}

// ============================================================
// ActorCmd Release / Retain
// ============================================================

func TestActorCmd_Release_WithMsg(t *testing.T) {
	msg := GetMessage()
	msg.Retain() // refCount=2
	cmd := ActorCmd{Msg: msg}
	cmd.Release()
	if cmd.Msg != nil {
		t.Fatal("Msg should be nil after Release")
	}
	if msg.LoadRefCount() != 1 {
		t.Fatalf("expected RefCount=1, got %d", msg.LoadRefCount())
	}
	msg.Release()
}

func TestActorCmd_Release_NilMsg(t *testing.T) {
	cmd := ActorCmd{}
	cmd.Release() // should not panic
}

func TestActorCmd_Retain_WithMsg(t *testing.T) {
	msg := GetMessage()
	defer msg.Release()

	cmd := ActorCmd{Msg: msg}
	cmd.Retain()
	if msg.LoadRefCount() != 2 {
		t.Fatalf("expected RefCount=2, got %d", msg.LoadRefCount())
	}
	msg.Release() // balance
}

func TestActorCmd_Retain_NilMsg(t *testing.T) {
	cmd := ActorCmd{}
	cmd.Retain() // should not panic
}

// ============================================================
// SmartReset vs PoolReset
// ============================================================

func TestSmartReset(t *testing.T) {
	msg := GetMessage()
	msg.MsgId = 100
	msg.Data = []byte("hello")
	msg.AuthId = 999
	msg.ToClient = true
	msg.IsResponse = true
	msg.RefCount = 5

	msg.SmartReset()

	if msg.MsgId != 0 || msg.AuthId != 0 || msg.ToClient || msg.IsResponse || msg.RefCount != 0 {
		t.Fatal("SmartReset didn't clear all fields")
	}
	if msg.Data != nil {
		t.Fatal("SmartReset should set Data to nil")
	}
}

func TestPoolReset(t *testing.T) {
	msg := &Message{
		MsgId:    100,
		Data:     make([]byte, 10, 256),
		AuthIds:  make([]int64, 5, 10),
		AuthId:   999,
		ToClient: true,
	}
	origDataCap := cap(msg.Data)
	origAuthCap := cap(msg.AuthIds)

	msg.PoolReset()

	if msg.MsgId != 0 || msg.AuthId != 0 || msg.ToClient {
		t.Fatal("PoolReset didn't clear fields")
	}
	if len(msg.Data) != 0 {
		t.Fatal("PoolReset should truncate Data to len=0")
	}
	if cap(msg.Data) != origDataCap {
		t.Fatal("PoolReset should preserve Data capacity")
	}
	if len(msg.AuthIds) != 0 {
		t.Fatal("PoolReset should truncate AuthIds to len=0")
	}
	if cap(msg.AuthIds) != origAuthCap {
		t.Fatal("PoolReset should preserve AuthIds capacity")
	}
}

// ============================================================
// Marshal / Unmarshal
// ============================================================

func TestMarshalUnmarshal_Roundtrip(t *testing.T) {
	orig := &Message{
		MsgId:      42,
		AuthId:     12345,
		SrcActor:   1,
		TarActor:   2,
		SessionId:  9999,
		RpcId:      8888,
		SeqId:      7,
		TraceIdHi:  5555,
		TraceIdLo:  6666,
		SpanId:     3333,
		ToClient:   true,
		FromClient: false,
		IsResponse: true,
		Data:       []byte("hello world"),
		AuthIds:    []int64{100, 200, 300},
	}

	buf, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	decoded := &Message{}
	if err := decoded.Unmarshal(buf); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.MsgId != orig.MsgId {
		t.Fatalf("MsgId mismatch: %d vs %d", decoded.MsgId, orig.MsgId)
	}
	if decoded.AuthId != orig.AuthId {
		t.Fatalf("AuthId mismatch")
	}
	if decoded.SrcActor != orig.SrcActor || decoded.TarActor != orig.TarActor {
		t.Fatal("Actor ID mismatch")
	}
	if decoded.SessionId != orig.SessionId || decoded.RpcId != orig.RpcId {
		t.Fatal("Session/RPC mismatch")
	}
	if decoded.SeqId != orig.SeqId || decoded.TraceIdHi != orig.TraceIdHi || decoded.TraceIdLo != orig.TraceIdLo || decoded.SpanId != orig.SpanId {
		t.Fatal("Seq/Trace/Span mismatch")
	}
	if !decoded.ToClient || decoded.FromClient || !decoded.IsResponse {
		t.Fatal("flags mismatch")
	}
	if string(decoded.Data) != "hello world" {
		t.Fatalf("Data mismatch: %q", decoded.Data)
	}
	if len(decoded.AuthIds) != 3 || decoded.AuthIds[0] != 100 || decoded.AuthIds[2] != 300 {
		t.Fatalf("AuthIds mismatch: %v", decoded.AuthIds)
	}
}

func TestMarshalUnmarshal_EmptyData(t *testing.T) {
	orig := &Message{MsgId: 1}
	buf, err := orig.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	decoded := &Message{}
	if err := decoded.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}
	if decoded.Data != nil {
		t.Fatal("empty Data should unmarshal as nil")
	}
	if decoded.AuthIds != nil {
		t.Fatal("empty AuthIds should unmarshal as nil")
	}
}

func TestMarshalTo_BufferTooSmall(t *testing.T) {
	msg := &Message{MsgId: 1, Data: []byte("data")}
	buf := make([]byte, 10)
	_, err := msg.MarshalTo(buf)
	if err != ErrBufferTooSmall {
		t.Fatalf("expected ErrBufferTooSmall, got %v", err)
	}
}

func TestUnmarshal_TooShort(t *testing.T) {
	msg := &Message{}
	err := msg.Unmarshal(make([]byte, 10))
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt, got %v", err)
	}
}

func TestUnmarshal_TruncatedData(t *testing.T) {
	orig := &Message{MsgId: 1, Data: []byte("hello")}
	buf, _ := orig.Marshal()
	// Truncate buffer so Data is incomplete
	err := (&Message{}).Unmarshal(buf[:FixedHeaderSize+4+2])
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt for truncated data, got %v", err)
	}
}

func TestUnmarshal_TruncatedAuthIds(t *testing.T) {
	orig := &Message{MsgId: 1, AuthIds: []int64{100, 200}}
	buf, _ := orig.Marshal()
	// Truncate buffer so AuthIds are incomplete
	err := (&Message{}).Unmarshal(buf[:len(buf)-4])
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt for truncated AuthIds, got %v", err)
	}
}

func TestUnmarshal_DataCapReuse(t *testing.T) {
	msg := &Message{Data: make([]byte, 0, 128)}
	orig := &Message{Data: []byte("reuse me")}
	buf, _ := orig.Marshal()
	if err := msg.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}
	if cap(msg.Data) < 128 {
		t.Fatal("Unmarshal should reuse existing Data capacity")
	}
}

func TestMarshalPooled(t *testing.T) {
	msg := &Message{
		MsgId: 42,
		Data:  []byte("pooled"),
	}
	buf, err := msg.MarshalPooled()
	if err != nil {
		t.Fatalf("MarshalPooled failed: %v", err)
	}
	defer buf.Release()

	decoded := &Message{}
	if err := decoded.Unmarshal(buf.B); err != nil {
		t.Fatalf("Unmarshal after MarshalPooled failed: %v", err)
	}
	if decoded.MsgId != 42 || string(decoded.Data) != "pooled" {
		t.Fatal("roundtrip mismatch")
	}
}

func TestSize(t *testing.T) {
	msg := &Message{}
	base := FixedHeaderSize + 4 + 4 // data len + authids count
	if msg.Size() != base {
		t.Fatalf("empty Size()=%d, want %d", msg.Size(), base)
	}

	msg.Data = []byte("test")
	msg.AuthIds = []int64{1, 2}
	want := base + 4 + 16 // 4 bytes data + 2*8 bytes authids
	if msg.Size() != want {
		t.Fatalf("Size()=%d, want %d", msg.Size(), want)
	}
}

// ============================================================
// UpdateFuncItem
// ============================================================

func TestNewUpdateFuncItem(t *testing.T) {
	called := false
	item := NewUpdateFuncItem("test", 1*time.Second, func(ctx context.Context, nowTs int64) {
		called = true
	})
	if item.Name != "test" || item.Interval != 1*time.Second {
		t.Fatal("field mismatch")
	}
	item.Do(context.Background(), 0)
	if !called {
		t.Fatal("Do func was not set")
	}
}

// ============================================================
// StartLeakDetector (DEBUG_LIFECYCLE=false path)
// ============================================================

func TestStartLeakDetector_DisabledMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartLeakDetector(ctx) // should return immediately when DEBUG_LIFECYCLE=false
}

// ============================================================
// PoolStats — 生产级消息池监控
// ============================================================

func TestPoolStats_GetRelease(t *testing.T) {
	before := GetPoolStats().Snapshot()

	msg := GetMessage()
	after1 := GetPoolStats().Snapshot()

	if after1.Outstanding != before.Outstanding+1 {
		t.Fatalf("expected outstanding +1, got %d -> %d", before.Outstanding, after1.Outstanding)
	}
	if after1.TotalGet != before.TotalGet+1 {
		t.Fatalf("expected totalGet +1, got %d -> %d", before.TotalGet, after1.TotalGet)
	}

	msg.Release()
	after2 := GetPoolStats().Snapshot()

	if after2.Outstanding != before.Outstanding {
		t.Fatalf("expected outstanding back to %d, got %d", before.Outstanding, after2.Outstanding)
	}
	if after2.TotalRelease != before.TotalRelease+1 {
		t.Fatalf("expected totalRelease +1, got %d -> %d", before.TotalRelease, after2.TotalRelease)
	}
}

func TestPoolStats_RetainDoesNotAffectOutstanding(t *testing.T) {
	before := GetPoolStats().Snapshot()

	msg := GetMessage()
	msg.Retain()
	afterRetain := GetPoolStats().Snapshot()

	if afterRetain.Outstanding != before.Outstanding+1 {
		t.Fatalf("Retain should not change outstanding: expected %d, got %d",
			before.Outstanding+1, afterRetain.Outstanding)
	}

	msg.Release() // refCount 2->1, not returned to pool
	afterRel1 := GetPoolStats().Snapshot()
	if afterRel1.Outstanding != before.Outstanding+1 {
		t.Fatalf("partial release should not change outstanding: expected %d, got %d",
			before.Outstanding+1, afterRel1.Outstanding)
	}

	msg.Release() // refCount 1->0, returned to pool
	afterRel2 := GetPoolStats().Snapshot()
	if afterRel2.Outstanding != before.Outstanding {
		t.Fatalf("expected outstanding back to %d, got %d", before.Outstanding, afterRel2.Outstanding)
	}
}

func TestPoolStats_DoubleRelease(t *testing.T) {
	before := GetPoolStats().Snapshot()

	msg := GetMessage()
	msg.Release()
	msg.Release() // double release

	after := GetPoolStats().Snapshot()
	if after.DoubleRelease != before.DoubleRelease+1 {
		t.Fatalf("expected doubleRelease +1, got %d -> %d", before.DoubleRelease, after.DoubleRelease)
	}
}

func TestPoolStats_ConcurrentAccuracy(t *testing.T) {
	before := GetPoolStats().Snapshot()

	const goroutines = 20
	const perGoroutine = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				m := GetMessage()
				m.Release()
			}
		}()
	}
	wg.Wait()

	after := GetPoolStats().Snapshot()
	totalOps := int64(goroutines * perGoroutine)

	if after.Outstanding != before.Outstanding {
		t.Fatalf("expected outstanding unchanged, got delta=%d", after.Outstanding-before.Outstanding)
	}
	if after.TotalGet-before.TotalGet != totalOps {
		t.Fatalf("expected totalGet +%d, got +%d", totalOps, after.TotalGet-before.TotalGet)
	}
	if after.TotalRelease-before.TotalRelease != totalOps {
		t.Fatalf("expected totalRelease +%d, got +%d", totalOps, after.TotalRelease-before.TotalRelease)
	}
}

func TestPoolStats_NilRelease(t *testing.T) {
	before := GetPoolStats().Snapshot()
	var m *Message
	m.Release()
	after := GetPoolStats().Snapshot()

	if before != after {
		t.Fatal("nil Release should not affect pool stats")
	}
}

func TestStartPoolMonitor_NormalPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	StartPoolMonitor(ctx, PoolMonitorConfig{
		CheckInterval: 50 * time.Millisecond,
		LeakThreshold: 100000,
		SampleWindow:  100 * time.Millisecond,
		MessageMaxAge: 100 * time.Millisecond,
	})
	time.Sleep(120 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)
}

func TestGetPoolStats_ReturnsSingleton(t *testing.T) {
	s1 := GetPoolStats()
	s2 := GetPoolStats()
	if s1 != s2 {
		t.Fatal("GetPoolStats should return the same instance")
	}
}

func TestSampling_DisabledByDefault(t *testing.T) {
	if samplingEnabled.Load() {
		t.Fatal("sampling should be disabled by default")
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkGetMessageRelease(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := GetMessage()
		m.Release()
	}
}

func BenchmarkRetainRelease(b *testing.B) {
	msg := GetMessage()
	defer msg.Release()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.Retain()
		msg.Release()
	}
}

func BenchmarkMarshalUnmarshal(b *testing.B) {
	msg := &Message{
		MsgId:    42,
		AuthId:   12345,
		SrcActor: 1,
		TarActor: 2,
		Data:     make([]byte, 128),
		AuthIds:  []int64{1, 2, 3},
	}
	buf := make([]byte, msg.Size())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.MarshalTo(buf)
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	orig := &Message{
		MsgId:    42,
		AuthId:   12345,
		SrcActor: 1,
		TarActor: 2,
		Data:     make([]byte, 128),
		AuthIds:  []int64{1, 2, 3},
	}
	buf, _ := orig.Marshal()
	target := &Message{Data: make([]byte, 0, 256)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.Unmarshal(buf)
	}
}

func BenchmarkMarshalPooled(b *testing.B) {
	msg := &Message{
		MsgId: 42,
		Data:  make([]byte, 128),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf, _ := msg.MarshalPooled()
		buf.Release()
	}
}

func BenchmarkMessagePool_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := GetMessage()
			m.MsgId = 1
			m.Data = append(m.Data, 1, 2, 3, 4)
			m.Release()
		}
	})
}

func BenchmarkLoadRefCount(b *testing.B) {
	msg := GetMessage()
	defer msg.Release()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = msg.LoadRefCount()
	}
}

func BenchmarkSize(b *testing.B) {
	msg := &Message{
		MsgId:   42,
		Data:    make([]byte, 256),
		AuthIds: []int64{1, 2, 3, 4, 5},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = msg.Size()
	}
}

func BenchmarkSmartReset(b *testing.B) {
	msg := &Message{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.MsgId = 42
		msg.Data = []byte("test")
		msg.SmartReset()
	}
}

func BenchmarkPoolReset(b *testing.B) {
	msg := &Message{
		Data:    make([]byte, 0, 256),
		AuthIds: make([]int64, 0, 8),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.MsgId = 42
		msg.Data = msg.Data[:10]
		msg.PoolReset()
	}
}

func BenchmarkActorCmd_Release(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := GetMessage()
		cmd := ActorCmd{Msg: msg}
		cmd.Release()
	}
}

func BenchmarkAtomicRefCount(b *testing.B) {
	var rc int32 = 1
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		atomic.AddInt32(&rc, 1)
		atomic.AddInt32(&rc, -1)
	}
}

func BenchmarkGetMessageRelease_WithStats(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := GetMessage()
		m.Release()
	}
}

func BenchmarkGetMessageRelease_WithStats_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := GetMessage()
			m.Release()
		}
	})
}

var benchSnap PoolStatsSnapshot

func BenchmarkPoolStatsSnapshot(b *testing.B) {
	stats := GetPoolStats()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSnap = stats.Snapshot()
	}
}
