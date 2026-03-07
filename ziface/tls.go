package ziface

import (
	"crypto/tls"
	"github.com/tjfoc/gmsm/gmtls"
	"net"
)

// TLSMode 标识 TLS 模式
type TLSMode int

const (
	TLSModeNone     TLSMode = iota // 不启用 TLS
	TLSModeStandard                // 标准 TLS（RSA/ECDSA）
	TLSModeGM                      // 国密 GM-TLS（SM2）
)

// TLSConfig 统一的 TLS 配置（支持标准 TLS 和 GM-TLS）
type TLSConfig struct {
	Mode      TLSMode
	StdConfig *tls.Config   // 标准 TLS 配置（Mode=TLSModeStandard 时使用）
	GMConfig  *gmtls.Config // GM-TLS 配置（Mode=TLSModeGM 时使用）
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
