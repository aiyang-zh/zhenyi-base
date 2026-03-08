package ziface

// ILimit 限流器接口。
//
// 用于控制请求通过速率，通常基于令牌桶或漏桶实现。
type ILimit interface {
	// Allow 返回当前请求是否被允许通过。
	Allow() bool
}
