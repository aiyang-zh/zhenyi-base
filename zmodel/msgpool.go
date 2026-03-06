package zmodel

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-core/zlog"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"sync/atomic"

	"go.uber.org/zap"
)

var messagePool = zpool.NewPool[*Message](func() *Message {
	return &Message{
		Data:    make([]byte, 0, 256),
		AuthIds: make([]int64, 0, 4),
	}
})

// GetMessage 从池中获取消息（初始引用计数 = 1）
func GetMessage() *Message {
	msg := messagePool.Get()
	msg.PoolReset()
	atomic.StoreInt32(&msg.RefCount, 1)

	poolStats.Outstanding.Add(1)
	poolStats.TotalGet.Add(1)
	sampleTrack(msg)

	if DEBUG_LIFECYCLE {
		trackMessage(msg)
	}
	return msg
}

func (m *Message) Retain() *Message {
	if m == nil {
		return nil
	}
	newRef := atomic.AddInt32(&m.RefCount, 1)
	if DEBUG_LIFECYCLE {
		if info, ok := liveMessages.Load(m); ok {
			zlog.Info("MSG Retain",
				zlog.FastUInt64("id", info.(*AllocInfo).ID), zlog.FastInt32("new", newRef+1), zlog.FastInt32("old", newRef))
		}

		if newRef <= 1 {
			panic("Retain called on released message")
		}
	}
	return m
}

// Release 减少引用计数，降为 0 时自动回收到池
func (m *Message) Release() {
	if m == nil {
		return
	}

	newRef := atomic.AddInt32(&m.RefCount, -1)
	if DEBUG_LIFECYCLE {
		if info, ok := liveMessages.Load(m); ok {
			zlog.Info("MSG Release",
				zlog.FastUInt64("id", info.(*AllocInfo).ID), zlog.FastInt32("new", newRef+1), zlog.FastInt32("old", newRef))
		}
	}
	if newRef < 0 {
		poolStats.DoubleRelease.Add(1)
		if DEBUG_LIFECYCLE {
			panic(fmt.Sprintf("Double release detected! refCount=%d msgId=%d", newRef, m.MsgId))
		}
		zlog.Error("Double release detected", zap.Int32("refCount", newRef), zap.Int32("msgId", m.MsgId))
		atomic.StoreInt32(&m.RefCount, 0)
		return
	}
	if newRef == 0 {
		if DEBUG_LIFECYCLE {
			untrackMessage(m)
		}
		sampleUntrack(m)

		poolStats.Outstanding.Add(-1)
		poolStats.TotalRelease.Add(1)

		atomic.StoreInt32(&m.RefCount, 0)
		if cap(m.Data) > 4096 {
			m.Data = nil
		}
		if cap(m.AuthIds) > 64 {
			m.AuthIds = nil
		}
		messagePool.Put(m)
	}
}

// MustRelease 安全释放（用于 defer）
func (m *Message) MustRelease() {
	if m != nil {
		m.Release()
	}
}
func (m *Message) LoadRefCount() int32 {
	if m == nil {
		return 0
	}
	return atomic.LoadInt32(&m.RefCount)
}
