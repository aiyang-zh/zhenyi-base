package zmodel

import (
	"context"
	"fmt"
)

// ActorModeConfig Actor 模式配置
type ActorModeConfig struct {
	Mode               int `json:"mode" mapstructure:"mode"`                                       // 0=顺序 1=并发
	ConcurrentPoolSize int `json:"concurrentPoolSize,omitempty" mapstructure:"concurrentPoolSize"` // 协程池大小
	ConcurrentMaxBatch int `json:"concurrentMaxBatch,omitempty" mapstructure:"concurrentMaxBatch"` // 最大批次
}

// IsSequential 是否为顺序模式
func (c ActorModeConfig) IsSequential() bool {
	return c.Mode == 0
}

// IsConcurrent 是否为并发模式
func (c ActorModeConfig) IsConcurrent() bool {
	return c.Mode == 1
}

// GetPoolSize 获取协程池大小（有默认值）
func (c ActorModeConfig) GetPoolSize() int {
	if c.ConcurrentPoolSize <= 0 {
		return 100
	}
	return c.ConcurrentPoolSize
}

// GetMaxBatch 获取最大批次大小（有默认值）
func (c ActorModeConfig) GetMaxBatch() int {
	if c.ConcurrentMaxBatch <= 0 {
		return 50
	}
	return c.ConcurrentMaxBatch
}

type ActorConfig struct {
	Id            int32           `json:"id"`        // id
	Process       uint            `json:"process"`   // 进程
	Name          string          `json:"name"`      // 名称
	ActorType     int32           `json:"actorType"` // 类型
	Index         int             `json:"index"`     // 索引
	Port          int             `json:"port"`      // 端口
	Host          string          `json:"host"`      // 地址
	Rate          int             `json:"rate"`
	Burst         int             `json:"burst"`
	WorkSize      int             `json:"workSize"`
	MaxRPCPending int             `json:"maxRPCPending"` // RPC 并发槽数（0=默认4096，必须是 2 的幂）
	MaxRestarts   int             `json:"maxRestarts"`   // Actor 崩溃后最大重启次数（0=默认3）
	ModeConfig    ActorModeConfig `json:"modeConfig"`    // 执行模式配置
}

func (a ActorConfig) GetTopic() string {
	return fmt.Sprintf("topic_%d_%d_%d", a.ActorType, a.Index, a.Id)
}

func (a ActorConfig) GetNameTopic() string {
	return fmt.Sprintf("topic_name_%d", a.ActorType)
}
func (a ActorConfig) GetActorId() int32 {
	return a.Id
}

func (a ActorConfig) GetActorType() int32 {
	return a.ActorType
}

type ActorServerRegister struct {
	Key         string      `json:"key"`
	Count       int32       `json:"count"`
	Weight      int32       `json:"weight"`
	ActorConfig ActorConfig `json:"actor"`
}

// CmdType Actor 命令类型
type CmdType = uint8

const (
	CmdTypeMsg            CmdType = 0 // 网络消息
	CmdTypeTick           CmdType = 1 // 定时器 Tick
	CmdTypeSafeFn         CmdType = 2 // 线程安全的内部闭包
	CmdTypeUpdateRegister CmdType = 3 // 批量消息处理
	CmdTypeClient         CmdType = 4 // 客户端消息
)

// ActorCmd 信封结构体（值类型入队，减少 GC 压力）
// UpdateFunc 改为指针：仅 CmdType_UpdateRegister 使用，避免每条消息拷贝 ~64 bytes
type ActorCmd struct {
	TickNow    int64           // CmdType_Tick 时间戳
	Msg        *Message        // CmdTypeMsg / CmdTypeClient
	Ctx        context.Context // 可选：携带 trace/cancel 链，nil 时使用 Actor 默认 ctx
	UpdateFunc *UpdateFuncItem // CmdTypeUpdateRegister（指针，减小 ActorCmd 体积）
	Fn         func()          // CmdType_SafeFn
	Type       uint8           // 消息类型
}

func (c *ActorCmd) Release() {
	if c.Msg != nil {
		c.Msg.Release()
		c.Msg = nil
	}
}

func (c *ActorCmd) Retain() {
	if c.Msg != nil {
		c.Msg.Retain()
	}
}
