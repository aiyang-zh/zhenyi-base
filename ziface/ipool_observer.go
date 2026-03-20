package ziface

// IPoolObserver 是对象池观测接口（可选）。
//
// 设计目标：
// - 不依赖任何具体指标/日志库（由业务侧实现）
// - 不改变对象池的语义与行为（仅旁路观测）
type IPoolObserver interface {
	// OnPoolCreate 在创建池时调用（name 可能为空）。
	OnPoolCreate(name string)
	// OnNew 在池为空触发分配新对象时调用（name 可能为空）。
	OnNew(name string)
	// OnGet 在 Get 被调用时触发（name 可能为空）。
	OnGet(name string)
	// OnPut 在 Put 被调用时触发（name 可能为空）。
	OnPut(name string)
	// OnPutNil 在 Put(nil)（可判定时）触发。注意：为了兼容性，不会阻止 Put。
	OnPutNil(name string)
}
