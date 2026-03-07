package zbackoff

import (
	"runtime"
	"time"
)

func Backoff(k int, first, second int, t time.Duration) {
	if k < first {
		cpuYield(30)
	} else if k < second {
		runtime.Gosched()
	} else {
		time.Sleep(t)
	}
}
