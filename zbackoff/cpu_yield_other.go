//go:build !amd64 && !arm64

package zbackoff

import "runtime"

func cpuYield(cycles uint32) {
	// 软回退：让出时间片，避免忙等待（LoongArch 等无 PAUSE/YIELD 指令的架构使用此实现）。
	// 注意：这不是忙等待，如果调用方需要精确的自旋语义，可能需要调整。
	runtime.Gosched()
}
