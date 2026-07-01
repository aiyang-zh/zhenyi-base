//go:build linux || darwin

// sharded_fdmap 与 checkHeartbeats 的分配行为测试。
package zreactor

import "testing"

func TestEachShardEntries_SteadyStateNoAlloc(t *testing.T) {
	m := newShardedFDMap()
	for i := range m.shards {
		for j := 0; j < 8; j++ {
			fd := i*1000 + j
			m.shards[i].entries[fd] = &connEntry{}
		}
	}
	m.eachShardEntries(func(int, *connEntry) {})

	allocs := testing.AllocsPerRun(100, func() {
		m.eachShardEntries(func(int, *connEntry) {})
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocs per eachShardEntries tick, got %v", allocs)
	}
}

type okHeartbeatChannel struct {
	heartbeatTestChannel
}

func (c *okHeartbeatChannel) Check() bool { return true }

func TestCheckHeartbeats_SteadyStateNoAlloc(t *testing.T) {
	m := newShardedFDMap()
	ch := &okHeartbeatChannel{heartbeatTestChannel: heartbeatTestChannel{id: 1}}
	m.shards[0].entries[1] = &connEntry{ch: ch}
	checkHeartbeats(nil, m, nil, defaultServeConfig())

	allocs := testing.AllocsPerRun(100, func() {
		checkHeartbeats(nil, m, nil, defaultServeConfig())
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocs per checkHeartbeats tick (no expirations), got %v", allocs)
	}
}
