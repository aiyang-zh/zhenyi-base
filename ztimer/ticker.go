package ztimer

import (
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"time"
)

var tickerPool = zpool.NewPoolWithOptions(func() *Ticker {
	return &Ticker{}
}, zpool.WithName("ztimer.ticker"))

type Ticker struct {
	ticker *time.Ticker
}

func NewTicker(d time.Duration) *Ticker {
	ticker := tickerPool.Get()
	if ticker.ticker == nil {
		ticker.ticker = time.NewTicker(d)
	} else {
		ticker.ticker.Reset(d)
	}
	return ticker
}

func (t *Ticker) ResetTime(d time.Duration) {
	t.ticker.Reset(d)
}

func (t *Ticker) Stop() {
	t.Reset()
	tickerPool.Put(t)
}

func (t *Ticker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *Ticker) Reset() {
	if t.ticker == nil {
		return
	}
	t.ticker.Stop()
	select {
	case <-t.ticker.C:
	default:
	}
}
