//go:build linux || darwin

package zreactor

import (
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"go.uber.org/zap"
)

// beginReactorRead 在连接读处理前后标记读栈深度；返回的 defer 函数须在同 goroutine 调用。
func beginReactorRead(ch ReactorChannel) func() {
	lc, ok := ch.(ReactorChannelLifecycle)
	if !ok {
		return func() {}
	}
	lc.BeginReactorRead()
	return lc.EndReactorRead
}

// closeReactorChannel 关闭 reactor 管理的连接；优先 CloseFromReactor 以免阻塞事件循环。
func closeReactorChannel(ch ReactorChannel) {
	if lc, ok := ch.(ReactorChannelLifecycle); ok {
		lc.CloseFromReactor()
		return
	}
	ch.Close()
}

// ReactorReadMetrics 可选：channel 在读缓冲 ingest 失败时递增 ConnErrorsInc（*znet.BaseChannel 实现）。
type ReactorReadMetrics interface {
	RecordReadIngestError()
}

func recordReadIngestError(ch ReactorChannel) {
	if m, ok := ch.(ReactorReadMetrics); ok {
		m.RecordReadIngestError()
	}
}

// ingestConnReadAndDispatch 将 syscall.Read 结果写入读缓冲并解析分发（单次 Parse 路径）。
func ingestConnReadAndDispatch(entry *connEntry, fd int, p []byte) (shouldClose bool) {
	defer func() {
		if r := recover(); r != nil {
			zlog.Recover("zreactor read ingest panic",
				zap.Int("fd", fd),
				zap.Uint64("channelId", entry.ch.GetChannelId()),
				zap.Any("panic", r))
			shouldClose = true
		}
	}()
	if writeConnBytes(entry.ch, p) {
		return true
	}
	return entry.ch.ParseAndDispatch()
}

// writeConnBytes 将读到的字节全部写入 channel 读缓冲；缓冲满时最多 Parse 一次腾挪后重试。
func writeConnBytes(ch ReactorChannel, p []byte) (shouldClose bool) {
	if len(p) == 0 {
		return false
	}
	off := 0
	drained := false
	for off < len(p) {
		n, err := ch.WriteToReadBuffer(p[off:])
		off += n
		if off >= len(p) {
			return false
		}
		if n > 0 {
			drained = false
			continue
		}
		if !drained {
			drained = true
			if ch.ParseAndDispatch() {
				return true
			}
			continue
		}
		recordReadIngestError(ch)
		zlog.Warn("WriteToReadBuffer error",
			zap.Uint64("channelId", ch.GetChannelId()),
			zap.Error(err))
		return true
	}
	return false
}
