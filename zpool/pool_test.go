package zpool

import (
	"bytes"
	"net"
	"runtime"
	"sync"
	"testing"
)

// ============================================================
// pool.go — 泛型 Pool[T]
// ============================================================

func TestPool_GetPut_Pointer(t *testing.T) {
	p := NewPool(func() *int {
		v := 42
		return &v
	})

	obj := p.Get()
	if obj == nil || *obj != 42 {
		t.Fatalf("expected *int(42), got %v", obj)
	}

	*obj = 100
	p.Put(obj)

	// Put 后再 Get 有可能复用（不保证，GC 可能清除）
	obj2 := p.Get()
	if obj2 == nil {
		t.Fatal("expected non-nil from pool")
	}
}

func TestPool_GetPut_Struct(t *testing.T) {
	type testStruct struct {
		Name string
		Val  int
	}

	p := NewPool(func() *testStruct {
		return &testStruct{Name: "init", Val: 0}
	})

	obj := p.Get()
	if obj.Name != "init" {
		t.Fatalf("expected Name='init', got %q", obj.Name)
	}

	obj.Name = "used"
	obj.Val = 99
	p.Put(obj)

	// 注意：Get 到的对象可能是复用的（带脏数据），也可能是新建的
	obj2 := p.Get()
	if obj2 == nil {
		t.Fatal("expected non-nil")
	}
}

func TestPool_Concurrent(t *testing.T) {
	p := NewPool(func() *int {
		v := 0
		return &v
	})

	const goroutines = 100
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				obj := p.Get()
				if obj == nil {
					t.Error("got nil from pool")
					return
				}
				*obj = j
				p.Put(obj)
			}
		}()
	}
	wg.Wait()
}

func TestPool_Get_NilNew(t *testing.T) {
	// 不设置 New 函数时 Get 返回零值
	p := &Pool[*int]{pool: sync.Pool{}}
	obj := p.Get()
	if obj != nil {
		t.Fatalf("expected nil, got %v", obj)
	}
}

func TestPool_WithName(t *testing.T) {
	p := NewPoolWithOptions(func() *int {
		v := 1
		return &v
	}, WithName("test_pool"))
	if p.Name() != "test_pool" {
		t.Fatalf("expected Name=test_pool, got %q", p.Name())
	}
}

type testObserver struct {
	get int64
	put int64
	new int64
}

func (o *testObserver) OnPoolCreate(string) {}
func (o *testObserver) OnNew(string)        { o.new++ }
func (o *testObserver) OnGet(string)        { o.get++ }
func (o *testObserver) OnPut(string)        { o.put++ }
func (o *testObserver) OnPutNil(string)     {}

func TestPool_WithObserver(t *testing.T) {
	local := &testObserver{}
	p := NewPoolWithOptions(func() *int {
		v := 1
		return &v
	}, WithName("obs_pool"), WithObserver(local))

	x := p.Get()
	p.Put(x)

	if local.get != 1 || local.put != 1 {
		t.Fatalf("expected local get/put=1/1, got %d/%d", local.get, local.put)
	}
}

func TestPool_PutNilPointer_Discarded(t *testing.T) {
	p := NewPool(func() *int {
		v := 42
		return &v
	})
	var z *int
	p.Put(z)
	x := p.Get()
	if x == nil {
		t.Fatal("expected non-nil *int from New after Put(nil) discarded")
	}
	if *x != 42 {
		t.Fatalf("want *x=42, got %d", *x)
	}
}

func TestPool_PutNilPointer_Observer(t *testing.T) {
	local := &testObserver{}
	p := NewPoolWithOptions(func() *int {
		v := 1
		return &v
	}, WithName("nil_discard"), WithObserver(local))
	var z *int
	p.Put(z)
	if local.put != 0 {
		t.Fatalf("Put(nil) must not count as OnPut, got put=%d", local.put)
	}
}

type testObserverPutNil struct {
	testObserver
	putNil int
}

func (o *testObserverPutNil) OnPutNil(string) { o.putNil++ }

func TestPool_PutNilPointer_OnPutNil(t *testing.T) {
	local := &testObserverPutNil{}
	p := NewPoolWithOptions(func() *int {
		v := 1
		return &v
	}, WithObserver(local))
	var z *int
	p.Put(z)
	if local.putNil != 1 {
		t.Fatalf("OnPutNil want 1, got %d", local.putNil)
	}
}

func TestPool_PutZeroValueInt_StillStored(t *testing.T) {
	p := NewPool(func() int { return 0 })
	p.Put(0)
	_ = p.Get()
}

// ============================================================
// buff.go — bytes.Buffer 池
// ============================================================

func TestBufferPool_GetPut(t *testing.T) {
	buf := GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer returned nil")
	}

	buf.WriteString("hello world")
	if buf.String() != "hello world" {
		t.Fatalf("unexpected content: %q", buf.String())
	}

	PutBuffer(buf)

	// Reset 后长度应为 0
	if buf.Len() != 0 {
		t.Fatalf("expected buf reset, len=%d", buf.Len())
	}
}

func TestBufferPool_PreserveCapacity(t *testing.T) {
	buf := GetBuffer()
	initialCap := buf.Cap()
	if initialCap < 16*1024 {
		t.Fatalf("expected initial cap >= 16KB, got %d", initialCap)
	}

	// 写入数据再归还
	buf.Write(make([]byte, 8*1024))
	PutBuffer(buf)

	// 再次获取，底层 cap 应保留
	buf2 := GetBuffer()
	if buf2.Cap() < 16*1024 {
		t.Fatalf("expected cap >= 16KB after reuse, got %d", buf2.Cap())
	}
	PutBuffer(buf2)
}

// ============================================================
// bytes.go — 分级 *Buffer 池
// ============================================================

func TestGetBytesBuffer_Sizes(t *testing.T) {
	tests := []struct {
		reqSize     int
		minExpected int
	}{
		{1, 1},       // 最小请求
		{64, 64},     // 最小桶 (1<<6)
		{65, 65},     // 需要上取整到 128 桶
		{128, 128},   // 精确 128
		{256, 256},   // 精确 256
		{1024, 1024}, // 1KB
		{4096, 4096}, // 4KB
	}

	for _, tc := range tests {
		buf := GetBytesBuffer(tc.reqSize)
		if len(buf.B) != tc.reqSize {
			t.Errorf("GetBytesBuffer(%d): len=%d, want %d", tc.reqSize, len(buf.B), tc.reqSize)
		}
		if cap(buf.B) < tc.reqSize {
			t.Errorf("GetBytesBuffer(%d): cap=%d, want >= %d", tc.reqSize, cap(buf.B), tc.reqSize)
		}
		buf.Release()
	}
}

func TestGetBytesBuffer_Oversized(t *testing.T) {
	// 超过 maxSize (64KB) 的直接分配
	size := maxSize + 1
	buf := GetBytesBuffer(size)
	if len(buf.B) != size {
		t.Fatalf("expected len=%d, got %d", size, len(buf.B))
	}

	// 超大 Buffer 归还后不入池（不应 panic）
	buf.Release()

	stats := GetStats()
	if stats.DirectAllocs == 0 {
		t.Fatal("expected DirectAllocs > 0 for oversized buffer")
	}
}

func TestGetBytesBuffer_ZeroPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for size=0")
		}
	}()
	GetBytesBuffer(0)
}

func TestGetBytesBuffer_NegativePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative size")
		}
	}()
	GetBytesBuffer(-1)
}

func TestGetBytesBufferZero(t *testing.T) {
	buf := GetBytesBufferZero(128)
	for i, b := range buf.B {
		if b != 0 {
			t.Fatalf("expected zero at index %d, got %d", i, b)
		}
	}
	buf.Release()
}

func TestBuffer_Reset(t *testing.T) {
	buf := GetBytesBuffer(128)
	buf.B[0] = 0xFF
	origCap := cap(buf.B)

	buf.Reset()
	if len(buf.B) != 0 {
		t.Fatalf("Reset: expected len=0, got %d", len(buf.B))
	}
	if cap(buf.B) != origCap {
		t.Fatalf("Reset: expected cap preserved=%d, got %d", origCap, cap(buf.B))
	}

	buf.Release()
}

func TestBuffer_Write(t *testing.T) {
	buf := &Buffer{B: make([]byte, 0, 64)}
	n, err := buf.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Write returned %d, want 5", n)
	}

	n, err = buf.Write([]byte(" world"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write returned %d, want 6", n)
	}

	if string(buf.B) != "hello world" {
		t.Fatalf("unexpected content: %q", string(buf.B))
	}
}

func TestBuffer_Len(t *testing.T) {
	buf := &Buffer{B: make([]byte, 100)}
	if buf.Len() != 100 {
		t.Fatalf("expected Len=100, got %d", buf.Len())
	}
	buf.Reset()
	if buf.Len() != 0 {
		t.Fatalf("expected Len=0 after Reset, got %d", buf.Len())
	}
}

func TestPutBytesBuffer_Nil(t *testing.T) {
	// 不应 panic
	PutBytesBuffer(nil)
}

func TestPutBytesBuffer_InvalidCap(t *testing.T) {
	// 非 2 的幂次容量，不应入池（也不应 panic）
	b := &Buffer{B: make([]byte, 100)}
	PutBytesBuffer(b)

	// 容量太小
	b2 := &Buffer{B: make([]byte, 1)}
	PutBytesBuffer(b2)
}

func TestPutBytesBuffer_RoundTrip(t *testing.T) {
	// 精确的 2^n 容量应能成功归还并复用
	buf := GetBytesBuffer(64)
	data := []byte("test data here")
	copy(buf.B, data)
	buf.Release()

	// 强制 GC 不回收池内对象（仅在同一轮次）
	buf2 := GetBytesBuffer(64)
	if cap(buf2.B) < 64 {
		t.Fatalf("expected cap >= 64, got %d", cap(buf2.B))
	}
	buf2.Release()
}

func TestGetStats(t *testing.T) {
	// 进行一些操作后检查统计
	for i := 0; i < 10; i++ {
		buf := GetBytesBuffer(64)
		buf.Release()
	}

	stats := GetStats()

	// 第一个桶（64 bytes）应有请求
	bucket := stats.BucketStats[0]
	if bucket.BucketSize != (1 << minShift) {
		t.Fatalf("expected BucketSize=%d, got %d", 1<<minShift, bucket.BucketSize)
	}
	if bucket.GetRequests == 0 {
		t.Fatal("expected GetRequests > 0 for 64-byte bucket")
	}
	if bucket.ReuseRate < 0 || bucket.ReuseRate > 100 {
		t.Fatalf("unexpected ReuseRate: %f", bucket.ReuseRate)
	}
}

func TestGetIndex_BucketMapping(t *testing.T) {
	tests := []struct {
		size int
		idx  int
	}{
		{1, 0},      // <= 64 → 0
		{64, 0},     // == 64 → 0
		{65, 1},     // 65 → 128 桶 → idx 1
		{128, 1},    // == 128 → 1
		{129, 2},    // 129 → 256 桶 → idx 2
		{256, 2},    // == 256 → 2
		{512, 3},    // == 512 → 3
		{1024, 4},   // == 1024 → 4
		{2048, 5},   // == 2048 → 5
		{4096, 6},   // == 4096 → 6
		{8192, 7},   // == 8192 → 7
		{16384, 8},  // == 16384 → 8
		{32768, 9},  // == 32768 → 9
		{65536, 10}, // == 65536 → 10 (maxShift=16)
	}

	for _, tc := range tests {
		got := getIndex(tc.size)
		if got != tc.idx {
			t.Errorf("getIndex(%d) = %d, want %d", tc.size, got, tc.idx)
		}
	}
}

// ============================================================
// netbuffer.go — net.Buffers 池
// ============================================================

func TestNetBufferPool_GetPut(t *testing.T) {
	buf := GetNetBuffer()
	if buf == nil {
		t.Fatal("GetNetBuffer returned nil")
	}

	// 追加一些数据
	*buf = append(*buf, []byte("hello"))
	*buf = append(*buf, []byte("world"))
	if len(*buf) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(*buf))
	}

	PutNetBuffer(buf)

	// 归还后长度应为 0
	if len(*buf) != 0 {
		t.Fatalf("expected 0 entries after Put, got %d", len(*buf))
	}
}

func TestNetBufferPool_PreserveCap(t *testing.T) {
	buf := GetNetBuffer()
	initialCap := cap(*buf)
	if initialCap < 512 {
		t.Fatalf("expected initial cap >= 512, got %d", initialCap)
	}
	PutNetBuffer(buf)
}

func TestNetBufferPool_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 50
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf := GetNetBuffer()
				*buf = append(*buf, []byte("data"))
				PutNetBuffer(buf)
			}
		}()
	}
	wg.Wait()
}

// ============================================================
// 基准测试 (Benchmarks)
// ============================================================

// --- Pool[T] ---

func BenchmarkPool_GetPut(b *testing.B) {
	p := NewPool(func() *int {
		v := 0
		return &v
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		p.Put(obj)
	}
}

func BenchmarkPool_GetPut_Parallel(b *testing.B) {
	p := NewPool(func() *int {
		v := 0
		return &v
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := p.Get()
			p.Put(obj)
		}
	})
}

// --- bytes.Buffer 池 ---

func BenchmarkBufferPool_GetPut(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer()
		buf.WriteString("benchmark test data for buffer pool")
		PutBuffer(buf)
	}
}

// --- *Buffer 分级池 ---

func BenchmarkGetBytesBuffer_64(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBuffer(64)
		buf.Release()
	}
}

func BenchmarkGetBytesBuffer_4096(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBuffer(4096)
		buf.Release()
	}
}

func BenchmarkGetBytesBuffer_65536(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBuffer(65536)
		buf.Release()
	}
}

func BenchmarkGetBytesBuffer_Oversized(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBuffer(maxSize + 1)
		buf.Release()
	}
}

func BenchmarkGetBytesBuffer_Parallel(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := GetBytesBuffer(1024)
			buf.Release()
		}
	})
}

func BenchmarkGetBytesBufferZero_128(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBufferZero(128)
		buf.Release()
	}
}

// --- *Buffer vs 裸 []byte 对比 ---

func BenchmarkBuffer_vs_RawSlice(b *testing.B) {
	b.Run("Buffer_Pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := GetBytesBuffer(1024)
			_ = buf.B
			buf.Release()
		}
	})

	b.Run("Raw_Make", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := make([]byte, 1024)
			_ = buf
		}
	})

	b.Run("Raw_SyncPool", func(b *testing.B) {
		rawPool := sync.Pool{
			New: func() any {
				return make([]byte, 1024)
			},
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := rawPool.Get().([]byte)
			_ = buf
			rawPool.Put(buf) //nolint: staticcheck // 故意测 interface 转换开销
		}
	})
}

// --- net.Buffers 池 ---

func BenchmarkNetBufferPool_GetPut(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetNetBuffer()
		*buf = append(*buf, []byte("data1"), []byte("data2"))
		PutNetBuffer(buf)
	}
}

// --- 内存分配对比：验证 *Buffer 确实是 0 allocs ---

func BenchmarkBytesBuffer_ZeroAlloc_Verify(b *testing.B) {
	// 预热池
	for i := 0; i < 100; i++ {
		buf := GetBytesBuffer(256)
		buf.Release()
	}
	runtime.GC()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBytesBuffer(256)
		buf.B[0] = byte(i)
		buf.Release()
	}
}

// --- bytes.Buffer 池 vs 直接 new ---

func BenchmarkBufferPool_vs_New(b *testing.B) {
	b.Run("Pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := GetBuffer()
			buf.WriteString("test")
			PutBuffer(buf)
		}
	})

	b.Run("New", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(make([]byte, 0, 16*1024))
			buf.WriteString("test")
			_ = buf
		}
	})
}

// --- net.Buffers 池 vs 直接 make ---

func BenchmarkNetBufferPool_vs_Make(b *testing.B) {
	b.Run("Pool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := GetNetBuffer()
			*buf = append(*buf, []byte("x"))
			PutNetBuffer(buf)
		}
	})

	b.Run("Make", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := make(net.Buffers, 0, 512)
			buf = append(buf, []byte("x"))
			_ = buf
		}
	})
}
