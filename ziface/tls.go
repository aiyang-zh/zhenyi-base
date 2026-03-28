package ziface

import (
	"crypto/tls"
	"net"
)

// TLSMode 标识 TLS 模式。
type TLSMode int

const (
	// TLSModeNone 表示不启用 TLS。
	TLSModeNone TLSMode = iota

	// TLSModeStandard 表示标准 TLS（RSA/ECDSA）。
	TLSModeStandard

	// TLSModeGM 表示国密 GM-TLS（SM2）。
	TLSModeGM
)

// TLSConfig 统一的 TLS 配置（支持标准 TLS 和 GM-TLS）。
type TLSConfig struct {
	// Mode 指定当前 TLS 模式。
	Mode TLSMode

	// StdConfig 为标准 TLS 配置（Mode=TLSModeStandard 时使用）。
	StdConfig *tls.Config

	// GMConfig 为 GM-TLS 配置（Mode=TLSModeGM 时使用）；对外为不透明包装，勿依赖底层类型。
	GMConfig *GMTLSConfig
}

// WrapListener 将 net.Listener 包装为 TLS listener。
// 如果 cfg 为 nil 或 Mode 为 TLSModeNone，直接返回原 listener。
// TLSModeGM 且 GMConfig 为 nil 时会 panic（配置错误）；请勿在未设置 GMConfig 时使用国密模式。
func (cfg *TLSConfig) WrapListener(ln net.Listener) net.Listener {
	if cfg == nil || cfg.Mode == TLSModeNone {
		return ln
	}
	switch cfg.Mode {
	case TLSModeStandard:
		return tls.NewListener(ln, cfg.StdConfig)
	case TLSModeGM:
		if cfg.GMConfig == nil {
			panic("ziface: TLSModeGM requires non-nil GMConfig")
		}
		return cfg.GMConfig.wrapListener(ln)
	default:
		return ln
	}
}
