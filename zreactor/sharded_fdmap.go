//go:build linux || darwin

package zreactor

import "sync"

const numShards = 256

// shardedFDMap 按 fd 分片的连接表，降低高连接数下的锁竞争。
type shardedFDMap struct {
	shards [numShards]struct {
		mu      sync.Mutex
		entries map[int]*connEntry
	}
	// heartbeatSnap、heartbeatExpired 仅供 reactor 主循环 goroutine 复用，避免周期性扫描分配。
	heartbeatSnap    []fdEntryPair
	heartbeatExpired []heartbeatExpireEntry
}

type heartbeatExpireEntry struct {
	fd    int
	entry *connEntry
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

// forEachEntry 遍历 fdMap 中所有连接；fn 返回 false 时提前结束。
func (m *shardedFDMap) forEachEntry(fn func(fd int, entry *connEntry) bool) {
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		for fd, entry := range s.entries {
			if !fn(fd, entry) {
				s.mu.Unlock()
				return
			}
		}
		s.mu.Unlock()
	}
}

type fdEntryPair struct {
	fd    int
	entry *connEntry
}

// eachShardEntries 按分片拷贝连接快照后在锁外回调，避免持锁执行 Check/Close 等慢路径。
// 复用 heartbeatSnap，稳态下无堆分配（仅连接数创新高时扩容）。
func (m *shardedFDMap) eachShardEntries(fn func(fd int, entry *connEntry)) {
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		n := len(s.entries)
		if n == 0 {
			s.mu.Unlock()
			continue
		}
		if cap(m.heartbeatSnap) < n {
			m.heartbeatSnap = make([]fdEntryPair, n, n*2)
		}
		snap := m.heartbeatSnap[:n]
		j := 0
		for fd, entry := range s.entries {
			snap[j].fd = fd
			snap[j].entry = entry
			j++
		}
		s.mu.Unlock()
		for j := 0; j < n; j++ {
			fn(snap[j].fd, snap[j].entry)
		}
	}
}
