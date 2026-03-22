package ztimer

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"time"
)

type TimerPool struct {
	pool *zpool.Pool[*time.Timer]
}

func NewTimerPool(name string, observer ziface.IPoolObserver) *TimerPool {
	opts := []zpool.Option{
		zpool.WithName(name),
	}
	if observer != nil {
		opts = append(opts, zpool.WithObserver(observer))
	}
	pool := zpool.NewPoolWithOptions(func() *time.Timer {
		t := time.NewTimer(time.Hour)
		// 确保 Timer 处于停止状态且 channel 已清空
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		return t
	}, opts...)
	return &TimerPool{pool: pool}
}
func (tp *TimerPool) Get(d time.Duration) *time.Timer {
	t := tp.pool.Get()
	// 确保 Timer 已停止且 channel 已排空，然后重置
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
	return t
}

func (tp *TimerPool) Put(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	tp.pool.Put(t)
}
