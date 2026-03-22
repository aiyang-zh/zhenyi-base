package zpool

import (
	"reflect"
	"sync"

	"github.com/aiyang-zh/zhenyi-base/ziface"
)

// Pool 是基于 sync.Pool 的泛型对象池。
type Pool[T any] struct {
	name             string
	observer         ziface.IPoolObserver
	pool             sync.Pool
	discardTypedZero bool // T 为指针类型时，typed nil 不入池（避免污染 sync.Pool）
}

// NewPool 创建泛型对象池。
func NewPool[T any](f func() T) *Pool[T] {
	return NewPoolWithOptions(f)
}

// NewPoolWithOptions 创建可配置的泛型对象池（推荐）。
func NewPoolWithOptions[T any](f func() T, opts ...Option) *Pool[T] {
	cfg := defaultPoolOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	name := cfg.name
	obs := cfg.observer

	p := &Pool[T]{
		name:             name,
		observer:         obs,
		discardTypedZero: reflect.TypeFor[T]().Kind() == reflect.Ptr,
	}
	p.pool = sync.Pool{
		New: func() any {
			if p.observer != nil {
				p.observer.OnNew(name)
			}
			return f()
		},
	}
	if p.observer != nil {
		p.observer.OnPoolCreate(name)
	}
	return p
}

// NewNamedPool 创建带名称的对象池。
func NewNamedPool[T any](name string, f func() T) *Pool[T] {
	return NewPoolWithOptions(f, WithName(name))
}

// Name 返回池名称（可能为空）。
func (p *Pool[T]) Name() string { return p.name }

// Get 从池中获取一个对象；若池为空则调用 New。
func (p *Pool[T]) Get() T {
	if p.observer != nil {
		p.observer.OnGet(p.name)
	}
	v := p.pool.Get()
	if v != nil {
		return v.(T)
	}
	var zero T
	return zero
}

// Put 将对象放回池中。当 T 为指针类型时，typed nil 丢弃不入池。
func (p *Pool[T]) Put(obj T) {
	if p.discardTypedZero {
		var z T
		if any(obj) == any(z) {
			if p.observer != nil {
				p.observer.OnPutNil(p.name)
			}
			return
		}
	}
	if p.observer != nil {
		p.observer.OnPut(p.name)
	}
	p.pool.Put(obj)
}
