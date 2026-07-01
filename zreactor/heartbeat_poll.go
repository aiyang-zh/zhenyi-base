//go:build linux || darwin

package zreactor

// heartbeatPollMs 解析 ServeConfig.HeartbeatPollMs，返回 epoll/kqueue 等待超时（毫秒）；-1 表示无限等待。
func heartbeatPollMs(cfg *ServeConfig) int {
	if cfg == nil {
		return defaultHeartbeatPollMs
	}
	if cfg.HeartbeatPollMs < 0 {
		return -1
	}
	if cfg.HeartbeatPollMs == 0 {
		return defaultHeartbeatPollMs
	}
	return cfg.HeartbeatPollMs
}
