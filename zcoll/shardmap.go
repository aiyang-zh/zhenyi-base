package zcoll

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"github.com/aiyang-zh/zhenyi-util/ztime"
	"hash/maphash"
	"sync"
	"time"
)

const LimitDelete = 100
const LimitCheck = 1000

// 全局 maphash.Hash 对象池（所有 Map 实例共享）
var globalHasherPool = zpool.NewPool(func() *maphash.Hash {
	return &maphash.Hash{}
})

type hashFunc[K any] func(key K) uint64

type Map[K comparable, V any] struct {
	seed       maphash.Seed
	shardCount uint64
	shards     []map[K]Item[V]
	locks      []sync.RWMutex
	hasher     hashFunc[K] // ✅ 构造期确定的 hash 函数
}

type Item[V any] struct {
	value V
	t     int64
}

// Hashable 可用于自定义 Hash 的 Key，避免 fmt.Sprintf 造成的分配
type Hashable interface {
	Hash() uint64
}

func (i *Item[V]) IsExpire() bool {
	if i.t > 0 && ztime.ServerNowUnixMilli() > i.t {
		return true
	}
	return false
}
func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 2
	} // 默认最小容量 2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
func NewMap[K comparable, V any](shardCount int) *Map[K, V] {
	shardCount = nextPowerOfTwo(shardCount)
	m := &Map[K, V]{
		shards:     make([]map[K]Item[V], shardCount),
		locks:      make([]sync.RWMutex, shardCount),
		shardCount: uint64(shardCount),
		seed:       maphash.MakeSeed(),
	}
	for i := 0; i < shardCount; i++ {
		m.shards[i] = make(map[K]Item[V])
	}

	// ✅ 构造期判断类型，设置最优 hasher（零逃逸、零分配）
	var k K
	switch any(k).(type) {
	case uint64:
		m.hasher = func(key K) uint64 {
			return any(key).(uint64)
		}

	case int64:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(int64))
		}

	case uint32:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(uint32))
		}

	case int32:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(int32))
		}

	case uint:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(uint))
		}

	case int:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(int))
		}

	case uintptr:
		m.hasher = func(key K) uint64 {
			return uint64(any(key).(uintptr))
		}

	case string:
		// string 使用 maphash（全局池）
		m.hasher = func(key K) uint64 {
			s := any(key).(string)
			h := globalHasherPool.Get()
			h.SetSeed(m.seed)
			_, _ = h.WriteString(s)
			hash := h.Sum64()
			h.Reset()
			globalHasherPool.Put(h)
			return hash
		}

	case Hashable:
		// 实现了 Hash() 接口的类型
		m.hasher = func(key K) uint64 {
			return any(key).(Hashable).Hash()
		}

	default:
		// 兜底：使用 fmt.Sprintf（会逃逸，但只在未知类型时触发）
		m.hasher = func(key K) uint64 {
			s := fmt.Sprintf("%v", key)
			h := globalHasherPool.Get()
			h.SetSeed(m.seed)
			_, _ = h.WriteString(s)
			hash := h.Sum64()
			h.Reset()
			globalHasherPool.Put(h)
			return hash
		}
	}

	return m
}

func (m *Map[K, V]) getShard(key K) int {
	// ✅ 使用构造期确定的 hasher（零逃逸、零分配）
	idx := m.hasher(key)
	// 二次散列，让分布更均匀
	idx ^= idx >> 16
	return int(idx & (m.shardCount - 1))
}

func (m *Map[K, V]) Load(key K) (V, bool) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].RLock()
	val, ok := m.shards[shardIndex][key]
	m.locks[shardIndex].RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	if val.IsExpire() {
		m.delExpire(key, shardIndex)
		var zero V
		return zero, false
	} else {
		return val.value, true
	}
}

func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].Lock()
	val, ok := m.shards[shardIndex][key]
	if !ok {
		m.locks[shardIndex].Unlock()
		var zero V
		return zero, false
	}
	delete(m.shards[shardIndex], key)
	m.locks[shardIndex].Unlock()
	if val.IsExpire() {
		var zero V
		return zero, false
	} else {
		return val.value, true
	}
}
func (m *Map[K, V]) delExpire(key K, shardIndex int) {
	m.locks[shardIndex].Lock()
	// 双重检查，防止重复删除或误删新数据
	if item, ok := m.shards[shardIndex][key]; ok && item.IsExpire() {
		delete(m.shards[shardIndex], key)
	}
	m.locks[shardIndex].Unlock()
}
func (m *Map[K, V]) Store(key K, value V) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].Lock()
	m.shards[shardIndex][key] = Item[V]{
		t:     -1,
		value: value,
	}
	m.locks[shardIndex].Unlock()
}

func (m *Map[K, V]) SetExpire(key K, value V, expire time.Duration) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].Lock()
	m.shards[shardIndex][key] = Item[V]{
		t:     ztime.ServerNowUnixMilli() + expire.Milliseconds(),
		value: value,
	}
	m.locks[shardIndex].Unlock()
}

func (m *Map[K, V]) Expire(key K, expire time.Duration) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].Lock()
	item, ok := m.shards[shardIndex][key]
	if ok {
		item.t = ztime.ServerNowUnixMilli() + expire.Milliseconds()
		m.shards[shardIndex][key] = item
	}
	m.locks[shardIndex].Unlock()
}

func (m *Map[K, V]) Delete(key K) {
	shardIndex := m.getShard(key)
	m.locks[shardIndex].Lock()
	delete(m.shards[shardIndex], key)
	m.locks[shardIndex].Unlock()
}

func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	for i := 0; i < int(m.shardCount); i++ {
		if !m.rangeShard(i, f) {
			break
		}
	}
}
func (m *Map[K, V]) rangeShard(shardIdx int, f func(key K, value V) bool) bool {
	m.locks[shardIdx].RLock()
	// 注意：Range 期间持有读锁，不能 delete，不能 modify
	// 如果 callback f 执行时间过长，会阻塞这个分片的写操作
	defer m.locks[shardIdx].RUnlock()

	for k, v := range m.shards[shardIdx] {
		if v.IsExpire() {
			continue
		}
		if !f(k, v.value) {
			return false
		}
	}
	return true
}
func (m *Map[K, V]) Count() int {
	n := 0
	for i, shard := range m.shards {
		m.locks[i].RLock()
		n += len(shard)
		m.locks[i].RUnlock()
	}
	return n
}

func (m *Map[K, V]) ClearTimer() {
	deleted := 0
	checked := 0
	for i := 0; i < int(m.shardCount); i++ {
		if deleted >= LimitDelete || checked > LimitCheck {
			return
		}
		m.locks[i].Lock()
		for k, v := range m.shards[i] {
			if v.IsExpire() {
				delete(m.shards[i], k)
				deleted++
			}
			checked++
			if deleted >= LimitDelete || checked >= LimitCheck {
				break
			}
		}
		m.locks[i].Unlock()

	}
}
