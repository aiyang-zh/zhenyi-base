package zpool

import "github.com/aiyang-zh/zhenyi-base/ziface"

// Option 用于配置 Pool 的可选行为（保持兼容：默认无行为变化）。
type Option func(*poolOptions)

type poolOptions struct {
	name     string
	observer ziface.IPoolObserver
}

func defaultPoolOptions() poolOptions { return poolOptions{} }

// WithName 为对象池指定名称，用于观测/聚合。
// name 为空时表示未命名。
func WithName(name string) Option {
	return func(o *poolOptions) { o.name = name }
}

// WithObserver 为当前池实例注入观测器。
// 传 nil 表示不启用观测（完全关闭）。
func WithObserver(obs ziface.IPoolObserver) Option {
	return func(o *poolOptions) { o.observer = obs }
}
