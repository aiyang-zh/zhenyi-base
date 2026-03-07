package zid

import (
	"fmt"
	"sync/atomic"
	"time"
)

// 配置常量
const (
	NodeBits = 12                  // 机器ID占 12 位 (支持 4096 个节点)
	SeqBits  = 64 - NodeBits       // 剩下 52 位留给序列
	NodeMask = (1 << NodeBits) - 1 // 机器ID掩码
	SeqMask  = (1 << SeqBits) - 1  // 序列掩码
)

var (
	nodeOffset uint64 // 预先计算好的机器ID偏移量 (高位)
	sequence   uint64 // 全局自增序列
)

// InitFast 初始化机器ID (在 main.go 或 startup 时调用一次)
// nodeId: 当前服务器的编号 (例如 1, 2, 100...)
func InitFast(nodeId int) {
	if nodeId > NodeMask {
		panic(fmt.Sprintf("NodeID must be between 0 and %d", NodeMask))
	}
	// 1. 将 MachineID 移到高 52 位
	nodeOffset = uint64(nodeId) << SeqBits
	// 2. 初始化序列
	// 为了防止重启后 ID 重复，我们将当前时间的纳秒数混入序列的初始值
	// 注意：只取低 52 位，避免溢出
	initSeq := uint64(time.Now().UnixNano()) & SeqMask
	atomic.StoreUint64(&sequence, initSeq)
}

// NextFast 生成下一个唯一 ID (极速、无锁)
func NextFast() uint64 {
	// 1. 原子自增 (耗时 ~5ns)
	seq := atomic.AddUint64(&sequence, 1)

	// 2. 拼接：机器ID(高位) | 序列(低位)
	// 使用 & SeqMask 确保序列无限自增也不会溢出污染到高位的 MachineID
	return nodeOffset | (seq & SeqMask)
}
