// Package zpool 提供基于 sync.Pool 的泛型对象池与分级字节缓冲等复用能力。
//
// # Pool[T] 与 Put
//
// Pool[T] 适用于任意 T。构造时根据 reflect.TypeFor[T]().Kind() 是否为 [reflect.Ptr]
// 决定是否启用「零值丢弃」分支：仅当 T 为指针类型时，Put 可能用 any(obj)==any(z)
// 检测 typed nil，避免将 nil 指针放入 sync.Pool；该比较只比较指针本身，可比较且安全。
// 当 T 为 slice、map、struct、标量等非指针类型时，不会执行上述比较，因此不会出现
// 「对不可比较的 slice/map 值做 interface 相等比较」导致的运行时 panic。
//
// 观测：可选 WithName、WithObserver（见 ziface.IPoolObserver），OnPutNil 表示 nil 已丢弃未入池。
package zpool
