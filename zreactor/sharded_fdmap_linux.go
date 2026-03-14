//go:build linux

package zreactor

import "sync"

const numShards = 256

// shardedFDMap 按 fd 分片的 map，降低高连接数下的锁竞争（目标 10w+ 连接时可选用）。
type shardedFDMap struct {
	shards [numShards]struct {
		mu      sync.Mutex
		entries map[int]*connEntry
	}
}

func newShardedFDMap() *shardedFDMap {
	m := &shardedFDMap{}
	for i := range m.shards {
		m.shards[i].entries = make(map[int]*connEntry)
	}
	return m
}

// shardIndex 用乘法哈希分散连续 fd，减轻 fd 连续分配时的分片负载不均。
func (m *shardedFDMap) shardIndex(fd int) int {
	// 0x9e3779b1 为 2^32/phi；无符号乘法后取模，保证 0..numShards-1。
	return int(uint32(fd) * 0x9e3779b1 % uint32(numShards))
}

func (m *shardedFDMap) shard(fd int) *struct {
	mu      sync.Mutex
	entries map[int]*connEntry
} {
	return &m.shards[m.shardIndex(fd)]
}

func (m *shardedFDMap) Get(fd int) (*connEntry, bool) {
	s := m.shard(fd)
	s.mu.Lock()
	entry, ok := s.entries[fd]
	s.mu.Unlock()
	return entry, ok
}

func (m *shardedFDMap) Set(fd int, entry *connEntry) {
	s := m.shard(fd)
	s.mu.Lock()
	s.entries[fd] = entry
	s.mu.Unlock()
}

func (m *shardedFDMap) Delete(fd int) {
	s := m.shard(fd)
	s.mu.Lock()
	delete(s.entries, fd)
	s.mu.Unlock()
}

// Len 返回当前连接数（仅统计 fdMap 中的 conn，不含 listener/wakeFd）。
func (m *shardedFDMap) Len() int {
	n := 0
	for i := range m.shards {
		m.shards[i].mu.Lock()
		n += len(m.shards[i].entries)
		m.shards[i].mu.Unlock()
	}
	return n
}
