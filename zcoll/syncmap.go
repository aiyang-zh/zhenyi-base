package zcoll

import "sync"

// SyncMap 是对 [sync.Map] 的泛型封装，在并发读多、键空间分片不固定时可直接使用标准库实现。
type SyncMap[K comparable, V any] struct {
	m *sync.Map
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{
		m: new(sync.Map),
	}
}

func (m *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (m *SyncMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

func (m *SyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (m *SyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

func (m *SyncMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	p, loaded := m.m.Swap(key, value)
	if !loaded {
		var zero V
		return zero, false
	}
	return p.(V), true
}

func (m *SyncMap[K, V]) CompareAndSwap(key K, old, new V) bool {
	return m.m.CompareAndSwap(key, old, new)
}

func (m *SyncMap[K, V]) CompareAndDelete(key K, old V) bool {
	return m.m.CompareAndDelete(key, old)
}

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

// Clear 清空 map（Go 1.23+ [sync.Map.Clear]）。
func (m *SyncMap[K, V]) Clear() {
	m.m.Clear()
}
