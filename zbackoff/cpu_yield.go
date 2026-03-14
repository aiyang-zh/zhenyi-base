//go:build amd64 || arm64

package zbackoff

// cpuYield 执行指定次数的自旋等待（CPU 暂停指令），用于短时忙等待。
// 参数 cycles 表示重复执行暂停指令的次数，具体时长与 CPU 主频有关。
func cpuYield(cycles uint32)
