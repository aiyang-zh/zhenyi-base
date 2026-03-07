package ziface

type BufferTier int

const (
	TierNone   BufferTier = 0
	TierSmall  BufferTier = 1 // 2KB
	TierMedium BufferTier = 2 // 8KB
	TierLarge  BufferTier = 3 // 16KB
)
