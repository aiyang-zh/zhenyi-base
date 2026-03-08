package ziface

import (
	"crypto/tls"
	"github.com/tjfoc/gmsm/gmtls"
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

	// GMConfig 为 GM-TLS 配置（Mode=TLSModeGM 时使用）。
	GMConfig *gmtls.Config
}

// WrapListener 将 net.Listener 包装为 TLS listener。
// 如果 cfg 为 nil 或 Mode 为 TLSModeNone，直接返回原 listener。
func (cfg *TLSConfig) WrapListener(ln net.Listener) net.Listener {
	if cfg == nil || cfg.Mode == TLSModeNone {
		return ln
	}
	switch cfg.Mode {
	case TLSModeStandard:
		return tls.NewListener(ln, cfg.StdConfig)
	case TLSModeGM:
		return gmtls.NewListener(ln, cfg.GMConfig)
	default:
		return ln
	}
}
