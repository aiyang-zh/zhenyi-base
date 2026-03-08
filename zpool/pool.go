package zpool

import (
	"sync"
)

// Pool 是一个简单的泛型对象池，基于 sync.Pool 封装。
//
// 适合缓存短生命周期、重复创建的对象，降低 GC 压力。
type Pool[T any] struct {
	pool sync.Pool
}

// NewPool 创建泛型对象池。
// f 用于在池为空时创建新的对象实例。
func NewPool[T any](f func() T) *Pool[T] {
	return &Pool[T]{pool: sync.Pool{
		New: func() any {
			return f()
		},
	}}
}

// Get 从池中获取一个对象；若池为空则返回类型零值。
// 通常配合 Put 一起使用。
func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	if v != nil {
		return v.(T)
	}
	var zero T
	return zero
}

// Put 将对象放回池中，供后续复用。
// 禁止 Put(nil)，否则会污染池导致后续 Get 返回 nil；指针类型需调用方保证非 nil。
func (p *Pool[T]) Put(obj T) {
	p.pool.Put(obj)
}
