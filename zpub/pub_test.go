package zpub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type MockObserver struct {
	ID        int
	Count     int32 // 使用 atomic 保证并发测试准确
	LastTopic string
}

// OnChange 必须使用指针接收者，否则无法通过 Subscribe 的检查
func (m *MockObserver) OnChange(e *Event) {
	atomic.AddInt32(&m.Count, 1)
	m.LastTopic = e.Topic
}

// PanicObserver 用于测试 Panic 恢复
type PanicObserver struct{}

func (p *PanicObserver) OnChange(e *Event) {
	panic("故意触发的 panic")
}

// UnsubObserver 用于测试在回调中取消订阅（防止死锁）
type UnsubObserver struct {
	Bus *EventBus
	ID  int
}

func (u *UnsubObserver) OnChange(e *Event) {
	// 在回调中调用 UnSubscribe，如果没有 Copy-On-Read 机制，这里会死锁
	u.Bus.UnSubscribe(e.Topic, u)
}

// ----------------------------------------------------------------
// 2. 逻辑功能测试
// ----------------------------------------------------------------

func TestEventBus_BasicFlow(t *testing.T) {
	bus := NewEventBus()
	obs := &MockObserver{ID: 1}
	topic := "login"

	// 1. 订阅
	bus.Subscribe(topic, obs)

	// 2. 发布
	bus.Publish(&Event{Topic: topic, Val: "user_1"})

	if atomic.LoadInt32(&obs.Count) != 1 {
		t.Errorf("期望收到 1 次消息，实际收到 %d", obs.Count)
	}

	// 3. 取消订阅
	bus.UnSubscribe(topic, obs)

	// 4. 再次发布
	bus.Publish(&Event{Topic: topic, Val: "user_2"})

	if atomic.LoadInt32(&obs.Count) != 1 {
		t.Errorf("取消订阅后不应再收到消息，当前计数 %d", obs.Count)
	}
}

func TestEventBus_DeadlockPrevention(t *testing.T) {
	// 测试：在 OnChange 回调中进行 UnSubscribe 操作
	// 预期：不会发生死锁
	bus := NewEventBus()
	topic := "recursion"

	obs := &UnsubObserver{Bus: bus, ID: 1}
	bus.Subscribe(topic, obs)

	done := make(chan bool)
	go func() {
		// 触发回调，回调内部会尝试获取写锁
		bus.Publish(&Event{Topic: topic})
		done <- true
	}()

	select {
	case <-done:
		// 成功完成
	case <-time.After(time.Second * 1):
		t.Fatal("测试超时，可能发生了死锁！(Publish 持有读锁时，回调试图获取写锁)")
	}
}

func TestEventBus_PanicRecovery(t *testing.T) {
	// 测试：某个订阅者 Panic 是否会影响其他订阅者
	bus := NewEventBus()
	topic := "panic_test"

	obs1 := &MockObserver{ID: 1}
	obsPanic := &PanicObserver{}
	obs2 := &MockObserver{ID: 2}

	bus.Subscribe(topic, obs1)
	bus.Subscribe(topic, obsPanic)
	bus.Subscribe(topic, obs2)

	// 捕获标准输出防止 panic 打印干扰测试视线（可选，此处直接运行）
	bus.Publish(&Event{Topic: topic})

	if atomic.LoadInt32(&obs1.Count) != 1 {
		t.Error("观察者 1 应该收到消息")
	}
	if atomic.LoadInt32(&obs2.Count) != 1 {
		t.Error("观察者 2 应该收到消息，不应被 panic 中断")
	}
}

type BadStruct struct{}

func (p BadStruct) OnChange(e *Event) {
	panic("故意触发的 panic")
}
func TestEventBus_PointerEnforcement(t *testing.T) {
	// 测试：是否强制要求传入指针
	bus := NewEventBus()
	topic := "bad_struct"

	// 定义一个非指针的结构体

	defer func() {
		if r := recover(); r == nil {
			t.Error("期望 Subscribe 抛出 panic (非指针检查)，但未发生")
		} else {
			t.Logf("成功捕获预期的 panic: %v", r)
		}
	}()

	// 传入值而非指针，应该触发 Panic
	bus.Subscribe(topic, BadStruct{})
}

type AsyncObserver struct {
	Wg *sync.WaitGroup
}

// 指针接收者实现接口
func (a *AsyncObserver) OnChange(e *Event) {
	// 模拟一点点耗时，确保如果是同步执行会很慢，异步执行会很快
	time.Sleep(10 * time.Millisecond)

	// 完成信号
	if a.Wg != nil {
		a.Wg.Done()
	}
}
func TestEventBus_AsyncPublish(t *testing.T) {
	bus := NewEventBus()
	topic := "async_test"

	count := 50 // 模拟 50 个订阅者
	var wg sync.WaitGroup
	wg.Add(count)

	// 1. 订阅 50 个观察者
	for i := 0; i < count; i++ {
		// 每个观察者都持有同一个 wg 的指针
		obs := &AsyncObserver{Wg: &wg}
		bus.Subscribe(topic, obs)
	}

	// 2. 开始计时
	start := time.Now()

	// 3. 异步发布
	// 如果是同步的，耗时应该是 50 * 10ms = 500ms
	// 如果是异步的，耗时应该是 启动协程的时间 (接近 0ms) + 所有协程并发执行最慢的那个 (约 10ms)
	bus.AsyncPublish(&Event{Topic: topic})

	// 验证发布函数本身是否立即返回（非阻塞）
	publishDuration := time.Since(start)
	if publishDuration > 20*time.Millisecond {
		t.Errorf("AsyncPublish 阻塞了太久: %v，期望它是立即返回的", publishDuration)
	}

	// 4. 等待所有协程执行完毕
	// 如果 AsyncPublish 内部逻辑有误（没启动协程），这里会死锁或超时，因为 Done 永远不会被调够次数
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// 成功：所有任务都执行了
		totalDuration := time.Since(start)
		t.Logf("所有异步任务完成，总耗时: %v", totalDuration)
	case <-time.After(1 * time.Second):
		t.Fatal("测试超时：AsyncPublish 没有正确执行完所有回调")
	}
}

// ----------------------------------------------------------------
// 补充单元测试
// ----------------------------------------------------------------

func TestEventBus_MultipleTopics(t *testing.T) {
	bus := NewEventBus()
	obs1 := &MockObserver{ID: 1}
	obs2 := &MockObserver{ID: 2}

	bus.Subscribe("topic_a", obs1)
	bus.Subscribe("topic_b", obs2)

	bus.Publish(&Event{Topic: "topic_a"})

	if atomic.LoadInt32(&obs1.Count) != 1 {
		t.Error("obs1 should receive 1 message from topic_a")
	}
	if atomic.LoadInt32(&obs2.Count) != 0 {
		t.Error("obs2 should not receive messages from topic_a")
	}

	bus.Publish(&Event{Topic: "topic_b"})

	if atomic.LoadInt32(&obs2.Count) != 1 {
		t.Error("obs2 should receive 1 message from topic_b")
	}
}

func TestEventBus_PublishNoSubscribers(t *testing.T) {
	bus := NewEventBus()
	// 不应 panic
	bus.Publish(&Event{Topic: "no_one_listens"})
}

func TestEventBus_UnSubscribeNonExistent(t *testing.T) {
	bus := NewEventBus()
	obs := &MockObserver{ID: 1}
	// 取消未订阅的 topic 不应 panic
	bus.UnSubscribe("nonexistent", obs)
}

func TestEventBus_SubscribeSameObserverTwice(t *testing.T) {
	bus := NewEventBus()
	obs := &MockObserver{ID: 1}
	topic := "duplicate"

	bus.Subscribe(topic, obs)
	bus.Subscribe(topic, obs)

	bus.Publish(&Event{Topic: topic})

	// map[ObServer]struct{} 去重，同一指针只存一份，应收到 1 次
	count := atomic.LoadInt32(&obs.Count)
	if count != 1 {
		t.Errorf("same pointer subscribed twice, map dedup expects 1 call, got %d", count)
	}
}

func TestEventBus_MultipleObserversSameTopic(t *testing.T) {
	bus := NewEventBus()
	topic := "multi"

	observers := make([]*MockObserver, 10)
	for i := range observers {
		observers[i] = &MockObserver{ID: i}
		bus.Subscribe(topic, observers[i])
	}

	bus.Publish(&Event{Topic: topic, Val: "data"})

	for i, obs := range observers {
		if atomic.LoadInt32(&obs.Count) != 1 {
			t.Errorf("observer %d should receive 1 message, got %d", i, obs.Count)
		}
	}
}

func TestEventBus_UnSubscribeMiddle(t *testing.T) {
	bus := NewEventBus()
	topic := "unsub_middle"

	obs1 := &MockObserver{ID: 1}
	obs2 := &MockObserver{ID: 2}
	obs3 := &MockObserver{ID: 3}

	bus.Subscribe(topic, obs1)
	bus.Subscribe(topic, obs2)
	bus.Subscribe(topic, obs3)

	// 取消中间的
	bus.UnSubscribe(topic, obs2)

	bus.Publish(&Event{Topic: topic})

	if atomic.LoadInt32(&obs1.Count) != 1 {
		t.Error("obs1 should receive message")
	}
	if atomic.LoadInt32(&obs2.Count) != 0 {
		t.Error("obs2 should not receive message after unsubscribe")
	}
	if atomic.LoadInt32(&obs3.Count) != 1 {
		t.Error("obs3 should receive message")
	}
}

// AtomicObserver 仅使用原子操作，适合并发测试
type AtomicObserver struct {
	Count int32
}

func (a *AtomicObserver) OnChange(e *Event) {
	atomic.AddInt32(&a.Count, 1)
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := NewEventBus()
	topic := "concurrent_pub"
	obs := &AtomicObserver{}
	bus.Subscribe(topic, obs)

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			bus.Publish(&Event{Topic: topic})
		}()
	}

	wg.Wait()

	if got := atomic.LoadInt32(&obs.Count); got != int32(count) {
		t.Errorf("expected %d messages, got %d", count, got)
	}
}

func TestEventBus_EventData(t *testing.T) {
	bus := NewEventBus()
	topic := "data_check"

	var receivedVal interface{}
	obs := &MockObserver{ID: 1}
	bus.Subscribe(topic, obs)

	// 发布带数据的事件
	bus.Publish(&Event{Topic: topic, Val: map[string]int{"score": 100}})

	// MockObserver 不保存 Val，这里只验证不 panic
	_ = receivedVal
	if atomic.LoadInt32(&obs.Count) != 1 {
		t.Error("should receive event with data")
	}
}

// ----------------------------------------------------------------
// 3. 性能基准测试
// ----------------------------------------------------------------

func benchmarkPublish(b *testing.B, subscriberCount int) {
	bus := NewEventBus()
	topic := "bench"

	// 预先填充订阅者
	for i := 0; i < subscriberCount; i++ {
		bus.Subscribe(topic, &MockObserver{ID: i})
	}

	event := &Event{Topic: topic, Val: "bench_data"}

	b.ResetTimer() // 重置计时器，排除初始化时间
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}

// 10 个订阅者的情况
func BenchmarkPublish_Sub10(b *testing.B) {
	benchmarkPublish(b, 10)
}

// 100 个订阅者的情况
func BenchmarkPublish_Sub100(b *testing.B) {
	benchmarkPublish(b, 100)
}

// 1000 个订阅者的情况
func BenchmarkPublish_Sub1000(b *testing.B) {
	benchmarkPublish(b, 1000)
}

// 10000 个订阅者的情况 (高并发模拟)
func BenchmarkPublish_Sub10000(b *testing.B) {
	benchmarkPublish(b, 10000)
}
