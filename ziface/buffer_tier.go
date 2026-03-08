package ziface

// BufferTier 表示写缓冲区的档位等级。
//
// 用于根据当前连接写入压力动态调整缓冲区大小。
type BufferTier int

const (
	// TierNone 表示无缓冲。
	TierNone BufferTier = 0

	// TierSmall 表示小缓冲区（约 2KB）。
	TierSmall BufferTier = 1

	// TierMedium 表示中等缓冲区（约 8KB）。
	TierMedium BufferTier = 2

	// TierLarge 表示大缓冲区（约 16KB）。
	TierLarge BufferTier = 3
)
