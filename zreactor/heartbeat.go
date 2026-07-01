//go:build linux || darwin

package zreactor

// heartbeatChecker 由 channel 可选实现；reactor 周期性调用 Check，返回 false 时关闭连接。
type heartbeatChecker interface {
	Check() bool
}

func checkHeartbeats(poller *Poller, fdMap *shardedFDMap, metrics *Metrics, cfg *ServeConfig) {
	expired := fdMap.heartbeatExpired[:0]
	fdMap.eachShardEntries(func(fd int, entry *connEntry) {
		hc, ok := entry.ch.(heartbeatChecker)
		if ok && !hc.Check() {
			expired = append(expired, heartbeatExpireEntry{fd: fd, entry: entry})
		}
	})
	fdMap.heartbeatExpired = expired
	for i := range fdMap.heartbeatExpired {
		e := &fdMap.heartbeatExpired[i]
		closeConn(poller, e.fd, e.entry, fdMap, metrics, cfg.ReadBufSize)
	}
}
