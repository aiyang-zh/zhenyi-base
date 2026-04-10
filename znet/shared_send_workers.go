package znet

import (
	"context"
	"runtime"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zqueue"
	"go.uber.org/zap"
)

type sharedSendWorkers struct {
	shards []sharedSendShard
}

type sharedSendShard struct {
	q      *zqueue.UnboundedMPSC[*BaseChannel]
	wakeup chan struct{}
}

func newSharedSendWorkers(ctx context.Context) *sharedSendWorkers {
	n := runtime.GOMAXPROCS(0)
	if n < 1 {
		n = 1
	}

	shards := make([]sharedSendShard, n)
	for i := 0; i < n; i++ {
		shards[i] = sharedSendShard{
			q:      zqueue.NewUnboundedMPSC[*BaseChannel](),
			wakeup: make(chan struct{}, 1),
		}
	}

	w := &sharedSendWorkers{shards: shards}
	for i := range w.shards {
		sh := &w.shards[i]
		go sh.loop(ctx)
	}
	return w
}

func (w *sharedSendWorkers) enqueue(ch *BaseChannel) {
	if ch == nil {
		return
	}
	idx := int(ch.GetChannelId() % uint64(len(w.shards)))
	sh := &w.shards[idx]

	sh.q.TryEnqueue(ch)
	select {
	case sh.wakeup <- struct{}{}:
	default:
	}
}

func (sh *sharedSendShard) loop(ctx context.Context) {
	const maxBatch = MaxBatchLimit
	batch := make([]ziface.IMessage, maxBatch)

	for {
		select {
		case <-ctx.Done():
			sh.drainShardQueue(batch)
			return
		case <-sh.wakeup:
			for {
				ch, ok := sh.q.Dequeue()
				if !ok || ch == nil {
					break
				}
				sh.flushChannel(batch, ch)
			}
		}
	}
}

func (sh *sharedSendShard) drainShardQueue(batch []ziface.IMessage) {
	for {
		ch, ok := sh.q.Dequeue()
		if !ok || ch == nil {
			break
		}
		sh.flushChannel(batch, ch)
	}
}

func (sh *sharedSendShard) flushChannel(batch []ziface.IMessage, ch *BaseChannel) {
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("shared send worker panic",
				zap.Any("panic", r),
				zap.Uint64("channelId", ch.GetChannelId()))
			ch.Close()
		}
	}()

	quota := GetSendLoopTuning().ReactorFlushBatchesPerTurn
	if quota <= 0 {
		quota = 8
	}
	processedBatches := 0

	for {
		n := ch.SharedSendDequeueBatch(batch)
		if n > 0 {
			ch.SharedSendProcessBatch(batch[:n])
			processedBatches++
			if processedBatches >= quota {
				sh.q.TryEnqueue(ch)
				select {
				case sh.wakeup <- struct{}{}:
				default:
				}
				return
			}
			continue
		}

		ch.SharedSendClearPending()
		n2 := ch.SharedSendDequeueBatch(batch[:1])
		if n2 > 0 {
			ch.SharedSendSetPending()
			ch.SharedSendProcessBatch(batch[:n2])
			processedBatches++
			if processedBatches >= quota {
				sh.q.TryEnqueue(ch)
				select {
				case sh.wakeup <- struct{}{}:
				default:
				}
				return
			}
			continue
		}
		return
	}
}
