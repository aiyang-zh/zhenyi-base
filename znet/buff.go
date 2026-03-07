package znet

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"io"
	"time"
)

const (
	checkIntervalNormal = 10 * time.Second
	checkIntervalNone   = 2 * time.Second
	idleTimeout         = 30 * time.Second

	freqHigh   = 500 // > 50 pkt/s
	freqMedium = 100 // > 10 pkt/s
	freqLow    = 20  // > 2 pkt/s
)

// AdaptiveWriter 自适应写缓冲器
// ⚠️ 非线程安全：必须由外部保证串行访问或加锁保护
type AdaptiveWriter struct {
	wr      io.Writer
	poolBuf *zpool.Buffer // 从池获取的缓冲
	n       int
	err     error

	tier       ziface.BufferTier
	writeCount int64
	lastCheck  time.Time
	lastWrite  time.Time
}

func NewAdaptiveWriter(w io.Writer) *AdaptiveWriter {
	return &AdaptiveWriter{
		wr:        w,
		tier:      ziface.TierNone,
		lastCheck: time.Now(),
		lastWrite: time.Now(),
	}
}

func (aw *AdaptiveWriter) Write(p []byte) (nn int, err error) {
	aw.writeCount++
	aw.lastWrite = time.Now()
	aw.tryAdapt()

	if aw.err != nil {
		return 0, aw.err
	}

	// TierNone: 直写
	if aw.poolBuf == nil {
		return aw.wr.Write(p)
	}

	buf := aw.poolBuf.B

	// 有缓冲：标准写入逻辑
	for len(p) > 0 {
		if aw.err != nil {
			return nn, aw.err
		}

		available := len(buf) - aw.n
		if available == 0 {
			if err := aw.flushInternal(); err != nil {
				return nn, err
			}
			buf = aw.poolBuf.B // flush 后 buf 引用可能未变，但保持一致
			continue
		}

		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}

		copy(buf[aw.n:], p[:toCopy])
		aw.n += toCopy
		nn += toCopy
		p = p[toCopy:]
	}

	return nn, nil
}

func (aw *AdaptiveWriter) Flush() error {
	return aw.flushInternal()
}

func (aw *AdaptiveWriter) tryAdapt() {
	now := time.Now()

	interval := checkIntervalNormal
	if aw.tier == ziface.TierNone {
		interval = checkIntervalNone
	}

	if now.Sub(aw.lastCheck) < interval {
		return
	}

	targetTier := ziface.TierNone
	if aw.writeCount >= freqHigh {
		targetTier = ziface.TierLarge
	} else if aw.writeCount >= freqMedium {
		targetTier = ziface.TierMedium
	} else if aw.writeCount >= freqLow {
		targetTier = ziface.TierSmall
	}

	// 升级：立即执行
	if aw.tier < targetTier {
		aw.resizeTier(targetTier)
	}

	// 降级：渐进式
	if aw.tier > targetTier {
		idleDuration := now.Sub(aw.lastWrite)
		if idleDuration > idleTimeout && aw.n == 0 && aw.tier > ziface.TierNone {
			aw.resizeTier(aw.tier - 1)
		}
	}

	aw.writeCount = 0
	aw.lastCheck = now
}

func (aw *AdaptiveWriter) resizeTier(newTier ziface.BufferTier) {
	if aw.tier == newTier {
		return
	}

	if aw.n > 0 {
		if err := aw.flushInternal(); err != nil {
			return
		}
	}

	if aw.poolBuf != nil {
		aw.poolBuf.Release()
		aw.poolBuf = nil
	}

	aw.tier = newTier
	switch newTier {
	case ziface.TierNone:
		// poolBuf = nil
	case ziface.TierSmall:
		aw.poolBuf = zpool.GetBytesBuffer(2048)
	case ziface.TierMedium:
		aw.poolBuf = zpool.GetBytesBuffer(8192)
	case ziface.TierLarge:
		aw.poolBuf = zpool.GetBytesBuffer(16384)
	}

	aw.n = 0
}

func (aw *AdaptiveWriter) flushInternal() error {
	if aw.err != nil {
		return aw.err
	}
	if aw.n == 0 {
		return nil
	}

	buf := aw.poolBuf.B
	written := 0
	for written < aw.n {
		n, err := aw.wr.Write(buf[written:aw.n])
		written += n

		// 防御性检查：防止死循环
		if n == 0 && err == nil {
			aw.err = io.ErrShortWrite
			return aw.err
		}

		if err != nil {
			if written < aw.n {
				copy(buf, buf[written:aw.n])
				aw.n = aw.n - written
			} else {
				aw.n = 0
			}
			aw.err = err
			return err
		}
	}

	aw.n = 0
	return nil
}

func (aw *AdaptiveWriter) Available() int {
	if aw.poolBuf == nil {
		return 0
	}
	return len(aw.poolBuf.B) - aw.n
}

func (aw *AdaptiveWriter) Buffered() int {
	return aw.n
}

func (aw *AdaptiveWriter) Close() error {
	if aw.wr == nil {
		return nil // 已关闭
	}

	err := aw.flushInternal()

	if aw.poolBuf != nil {
		aw.poolBuf.Release()
		aw.poolBuf = nil
	}

	aw.tier = ziface.TierNone
	aw.n = 0
	aw.wr = nil // 标记已关闭

	return err
}

func (aw *AdaptiveWriter) Reset(w io.Writer) {
	if aw.poolBuf != nil {
		aw.poolBuf.Release()
		aw.poolBuf = nil
	}

	aw.wr = w
	aw.tier = ziface.TierNone
	aw.n = 0
	aw.err = nil
	aw.writeCount = 0
	aw.lastCheck = time.Now()
	aw.lastWrite = time.Now()
}

func (aw *AdaptiveWriter) GetTier() ziface.BufferTier {
	return aw.tier
}
