package zpool

import (
	"sync"

	"github.com/aiyang-zh/zhenyi-base/ziface"
)

// Pool 是一个简单的泛型对象池，基于 sync.Pool 封装。
//
// 适合缓存短生命周期、重复创建的对象，降低 GC 压力。
type Pool[T any] struct {
	name     string
	observer ziface.IPoolObserver
	pool     sync.Pool
}

// NewPool 创建泛型对象池。
// f 用于在池为空时创建新的对象实例。
func NewPool[T any](f func() T) *Pool[T] {
	return NewPoolWithOptions[T](f)
}

// NewPoolWithOptions 创建可配置的泛型对象池（推荐）。
// 兼容性：不影响池的语义与行为；未配置任何 Option 时与 NewPool 等价。
func NewPoolWithOptions[T any](f func() T, opts ...Option) *Pool[T] {
	cfg := defaultPoolOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	name := cfg.name
	obs := cfg.observer

	p := &Pool[T]{name: name, observer: obs}
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
// name 用于观测与指标聚合；为空时表示“未命名”。
// 兼容性：不影响池的语义与行为。
func NewNamedPool[T any](name string, f func() T) *Pool[T] {
	return NewPoolWithOptions[T](f, WithName(name))
}

// Name 返回池名称（可能为空）。
func (p *Pool[T]) Name() string { return p.name }

// Get 从池中获取一个对象；若池为空则返回类型零值。
// 通常配合 Put 一起使用。
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

// Put 将对象放回池中，供后续复用。
// 禁止 Put(nil)，否则会污染池导致后续 Get 返回 nil；指针类型需调用方保证非 nil。
func (p *Pool[T]) Put(obj T) {
	if p.observer != nil {
		p.observer.OnPut(p.name)
		// 不改变既有行为：仅在可判定时记录 Put(nil)。
		if any(obj) == nil {
			p.observer.OnPutNil(p.name)
		}
	}
	p.pool.Put(obj)
}
