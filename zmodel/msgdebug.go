package zmodel

import (
	"context"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-core/zlog"

	"go.uber.org/zap"
)

const DEBUG_LIFECYCLE = false // 生产环境关闭

// PoolStats 消息池运行时统计（全 atomic，线程安全，零锁）
type PoolStats struct {
	Outstanding   atomic.Int64 // 当前在外未归还的消息数
	TotalGet      atomic.Int64 // 累计 GetMessage 次数
	TotalRelease  atomic.Int64 // 累计归池次数
	DoubleRelease atomic.Int64 // double-release 检测次数
}

// Snapshot 统计快照（无锁读取，各字段可能不完全一致但足够监控用）
type PoolStatsSnapshot struct {
	Outstanding   int64
	TotalGet      int64
	TotalRelease  int64
	DoubleRelease int64
}

func (s *PoolStats) Snapshot() PoolStatsSnapshot {
	return PoolStatsSnapshot{
		Outstanding:   s.Outstanding.Load(),
		TotalGet:      s.TotalGet.Load(),
		TotalRelease:  s.TotalRelease.Load(),
		DoubleRelease: s.DoubleRelease.Load(),
	}
}

var poolStats PoolStats

// GetPoolStats 获取消息池统计（MIT 层 API，上层可注册到 monitor.Manager）
func GetPoolStats() *PoolStats {
	return &poolStats
}

// PoolMonitorConfig 池监控配置
type PoolMonitorConfig struct {
	CheckInterval time.Duration // 检查间隔（默认 30s）
	LeakThreshold int64         // outstanding 超过此值触发采样（默认 10000）
	SampleWindow  time.Duration // 采样窗口时长（默认 10s）
	MessageMaxAge time.Duration // 消息超龄阈值（默认 10s）
}

func defaultPoolMonitorConfig() PoolMonitorConfig {
	return PoolMonitorConfig{
		CheckInterval: 30 * time.Second,
		LeakThreshold: 10000,
		SampleWindow:  10 * time.Second,
		MessageMaxAge: 10 * time.Second,
	}
}

var (
	samplingEnabled    atomic.Bool
	samplingInProgress atomic.Bool
	sampledMessages    sync.Map // *Message -> *SampleInfo
)

// SampleInfo 采样期间捕获的消息分配信息
type SampleInfo struct {
	Stack      string
	CreateTime time.Time
}

func sampleTrack(m *Message) {
	if !samplingEnabled.Load() {
		return
	}
	sampledMessages.Store(m, &SampleInfo{
		Stack:      string(debug.Stack()),
		CreateTime: time.Now(),
	})
}

func sampleUntrack(m *Message) {
	if !samplingEnabled.Load() {
		return
	}
	sampledMessages.Delete(m)
}

// StartPoolMonitor 启动池监控 goroutine（MIT 层，生产环境可用）
//
// 正常路径：仅读取 atomic 计数器，零开销
// 异常路径：开启采样窗口，捕获堆栈用于定位泄漏
func StartPoolMonitor(ctx context.Context, cfgs ...PoolMonitorConfig) {
	cfg := defaultPoolMonitorConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
		if cfg.CheckInterval <= 0 {
			cfg.CheckInterval = 30 * time.Second
		}
		if cfg.LeakThreshold <= 0 {
			cfg.LeakThreshold = 10000
		}
		if cfg.SampleWindow <= 0 {
			cfg.SampleWindow = 10 * time.Second
		}
		if cfg.MessageMaxAge <= 0 {
			cfg.MessageMaxAge = 10 * time.Second
		}
	}

	ticker := time.NewTicker(cfg.CheckInterval)
	var prevOutstanding int64

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap := poolStats.Snapshot()

				if snap.DoubleRelease > 0 {
					zlog.Error("msgpool double-release detected",
						zap.Int64("count", snap.DoubleRelease),
						zap.Int64("outstanding", snap.Outstanding))
				}

				growthRate := snap.Outstanding - prevOutstanding
				prevOutstanding = snap.Outstanding

				if snap.Outstanding > cfg.LeakThreshold || growthRate > cfg.LeakThreshold/2 {
					if !samplingInProgress.CompareAndSwap(false, true) {
						continue
					}
					zlog.Warn("msgpool potential leak detected, starting sample window",
						zap.Int64("outstanding", snap.Outstanding),
						zap.Int64("growth", growthRate),
						zap.Int64("threshold", cfg.LeakThreshold))

					samplingEnabled.Store(true)

					time.AfterFunc(cfg.SampleWindow, func() {
						samplingEnabled.Store(false)

						leaked := 0
						now := time.Now()
						sampledMessages.Range(func(key, value interface{}) bool {
							info := value.(*SampleInfo)
							age := now.Sub(info.CreateTime)
							if age > cfg.MessageMaxAge {
								leaked++
								msg := key.(*Message)
								zlog.Error("msgpool leaked message found",
									zap.Int32("refCount", msg.LoadRefCount()),
									zap.Duration("age", age),
									zap.String("allocStack", info.Stack))
							}
							sampledMessages.Delete(key)
							return true
						})

						if leaked > 0 {
							zlog.Error("msgpool leak sample result",
								zap.Int("leakedCount", leaked),
								zap.Int64("outstanding", poolStats.Outstanding.Load()))
						} else {
							zlog.Info("msgpool sample window complete, no leaks found",
								zap.Int64("outstanding", poolStats.Outstanding.Load()))
						}
						samplingInProgress.Store(false)
					})
				}

				if snap.Outstanding < 0 {
					zlog.Error("msgpool outstanding negative (indicates bug)",
						zap.Int64("outstanding", snap.Outstanding))
				}
			}
		}
	}()
}

// ============ DEBUG_LIFECYCLE 模式（编译期裁剪） ============

var (
	liveMessages sync.Map // *Message -> *AllocInfo
	nextMsgID    atomic.Uint64
)

type AllocInfo struct {
	ID         uint64
	Stack      string
	CreateTime time.Time
}

func trackMessage(m *Message) {
	if !DEBUG_LIFECYCLE {
		return
	}

	id := nextMsgID.Add(1)
	liveMessages.Store(m, &AllocInfo{
		ID:         id,
		Stack:      string(debug.Stack()),
		CreateTime: time.Now(),
	})

	log.Printf("MSG#%d created (refCount=%d)", id, m.GetRefCount())
}

func untrackMessage(m *Message) {
	if !DEBUG_LIFECYCLE {
		return
	}

	if info, ok := liveMessages.LoadAndDelete(m); ok {
		allocInfo := info.(*AllocInfo)
		lifetime := time.Since(allocInfo.CreateTime)
		log.Printf("MSG#%d released (lifetime=%v)", allocInfo.ID, lifetime)
	}
}

// StartLeakDetector DEBUG 模式专用，生产环境 DEBUG_LIFECYCLE=false 时直接返回
func StartLeakDetector(ctx context.Context) {
	if !DEBUG_LIFECYCLE {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				leaked := 0
				now := time.Now()

				liveMessages.Range(func(key, value interface{}) bool {
					msg := key.(*Message)
					info := value.(*AllocInfo)

					if now.Sub(info.CreateTime) > 10*time.Second {
						leaked++
						log.Printf("MSG#%d LEAKED (refCount=%d, age=%v)\nAlloc at:\n%s",
							info.ID, msg.LoadRefCount(), now.Sub(info.CreateTime), info.Stack)
					}
					return true
				})

				if leaked > 0 {
					log.Printf("Total leaked messages: %d", leaked)
				}
			}
		}
	}()
}

// ForceGCAndCheck 用于测试：强制 GC 后检查 outstanding
func ForceGCAndCheck() PoolStatsSnapshot {
	runtime.GC()
	return poolStats.Snapshot()
}
