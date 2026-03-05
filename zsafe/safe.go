package zsafe

import (
	"runtime"
)

func Recover(msg string) {
	if err := recover(); err != nil {
		// 限制堆栈大小，防止刷屏
		// 只取前 2048 字节，通常足够覆盖核心错误路径
		stackBuf := make([]byte, 2048)
		n := runtime.Stack(stackBuf, false)
		shortStack := string(stackBuf[:n])

		println("Panic:", msg, err, shortStack)
	}
}
