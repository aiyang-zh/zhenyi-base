package znet

import (
	"sync/atomic"
	"time"
)

// SendLoopTuning controls BaseChannel runSend batching/backoff behavior.
// It is intentionally lock-free for hot paths.
type SendLoopTuning struct {
	// Adaptive batcher params.
	BatchMin        int
	BatchMax        int
	BatchTargetMean time.Duration
	MaxBatchLimit   int

	// Backoff params when queue is empty.
	BackoffFirst  int
	BackoffSecond int
	BackoffSleep  time.Duration

	// Shrink on prolonged idle.
	IdleShrinkAfter int
	IdleShrinkEvery time.Duration

	// Reactor shared-write protection knobs.
	// ReactorMaxQueuedMsgs：单连接发送队列软上限（超过则打 Warn，并可配合 BaseChannel.SetSendQueueOverflowHook 选择断链；0 表示不在 SetSendLoopTuning 中覆盖当前值）。
	ReactorMaxQueuedMsgs int
	// ReactorFlushBatchesPerTurn：共享写 worker 单次轮转内对单连接最多连续 flush 的批次数（公平性）。
	ReactorFlushBatchesPerTurn int
}

var sendLoopTuning atomic.Value // SendLoopTuning

func init() {
	sendLoopTuning.Store(SendLoopTuning{
		BatchMin:        1,
		BatchMax:        200,
		BatchTargetMean: 5 * time.Millisecond,
		MaxBatchLimit:   MaxBatchLimit,

		BackoffFirst:  10,
		BackoffSecond: 30,
		BackoffSleep:  time.Microsecond,

		IdleShrinkAfter: 100,
		IdleShrinkEvery: 30 * time.Second,

		ReactorMaxQueuedMsgs:       8192,
		ReactorFlushBatchesPerTurn: 8,
	})
}

// SetSendLoopTuning updates BaseChannel runSend tuning. Zero values are kept as-is.
// This is intended for experiments/benchmarks; call it once during startup.
func SetSendLoopTuning(t SendLoopTuning) {
	cur := GetSendLoopTuning()
	if t.BatchMin > 0 {
		cur.BatchMin = t.BatchMin
	}
	if t.BatchMax > 0 {
		cur.BatchMax = t.BatchMax
	}
	if t.BatchTargetMean > 0 {
		cur.BatchTargetMean = t.BatchTargetMean
	}
	if t.MaxBatchLimit > 0 {
		cur.MaxBatchLimit = t.MaxBatchLimit
	}
	if t.BackoffFirst > 0 {
		cur.BackoffFirst = t.BackoffFirst
	}
	if t.BackoffSecond > 0 {
		cur.BackoffSecond = t.BackoffSecond
	}
	if t.BackoffSleep > 0 {
		cur.BackoffSleep = t.BackoffSleep
	}
	if t.IdleShrinkAfter > 0 {
		cur.IdleShrinkAfter = t.IdleShrinkAfter
	}
	if t.IdleShrinkEvery > 0 {
		cur.IdleShrinkEvery = t.IdleShrinkEvery
	}
	if t.ReactorMaxQueuedMsgs > 0 {
		cur.ReactorMaxQueuedMsgs = t.ReactorMaxQueuedMsgs
	}
	if t.ReactorFlushBatchesPerTurn > 0 {
		cur.ReactorFlushBatchesPerTurn = t.ReactorFlushBatchesPerTurn
	}
	sendLoopTuning.Store(cur)
}

func GetSendLoopTuning() SendLoopTuning {
	return sendLoopTuning.Load().(SendLoopTuning)
}
