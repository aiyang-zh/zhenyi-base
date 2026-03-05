package zpool

import (
	"sync"
)

// Pool 对象池
type Pool[T any] struct {
	pool sync.Pool
}

func NewPool[T any](f func() T) *Pool[T] {
	return &Pool[T]{pool: sync.Pool{
		New: func() any {
			return f()
		},
	}}
}

func (p *Pool[T]) Get() T {
	v := p.pool.Get()
	if v != nil {
		return v.(T)
	}
	var zero T
	return zero
}

func (p *Pool[T]) Put(obj T) {
	p.pool.Put(obj)
}
