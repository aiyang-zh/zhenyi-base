package ziface

// ConnMode 连接模式：Client 与 Server 统一使用。
// - ModeSync: 默认，Request/ReplyImmediate 同步收发
// - ModeAsync: 可选，Read/队列 流式收发
type ConnMode bool

const (
	ModeSync  ConnMode = false
	ModeAsync ConnMode = true
)
