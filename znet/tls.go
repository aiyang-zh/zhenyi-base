// Package network 提供 TLS/GM-TLS 配置支持。
//
// 支持两种 TLS 模式：
//   - 标准 TLS（使用 RSA/ECDSA 证书）
//   - 国密 GM-TLS（使用 SM2 证书，符合 GM/T 0024-2014）
//
// 默认不启用 TLS。通过配置启用后，对上层业务代码完全透明。
//
// 标准 TLS 示例:
//
//	tlsCfg, _ := network.NewStandardTLSConfig("server.crt", "server.key")
//	server := network.NewTServer(addr, handlers)
//	server.SetTLSConfig(tlsCfg)
//
// GM-TLS 示例:
//
//	tlsCfg, _ := network.NewGMTLSConfig("sm2_sign.crt", "sm2_sign.key", "sm2_enc.crt", "sm2_enc.key")
//	server := network.NewTServer(addr, handlers)
//	server.SetTLSConfig(tlsCfg)
//
// 客户端连接:
//
//	conn, _ := network.DialTLS("tcp", addr, tlsCfg)
package znet

import (
	"crypto/tls"
	"errors"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"net"

	"github.com/tjfoc/gmsm/gmtls"
)

// NewStandardTLSConfig 从证书文件创建标准 TLS 配置。
func NewStandardTLSConfig(certFile, keyFile string) (*ziface.TLSConfig, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.New("tls: failed to load certificate: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeStandard,
		StdConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}, nil
}

// NewStandardTLSConfigFromPEM 从 PEM 字节创建标准 TLS 配置。
func NewStandardTLSConfigFromPEM(certPEM, keyPEM []byte) (*ziface.TLSConfig, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.New("tls: failed to parse certificate: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeStandard,
		StdConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}, nil
}

// NewGMTLSConfig 从 SM2 证书文件创建 GM-TLS 配置。
//
// GM-TLS 使用双证书体系：签名证书 + 加密证书。
// signCertFile/signKeyFile: SM2 签名证书和私钥
// encCertFile/encKeyFile:   SM2 加密证书和私钥
func NewGMTLSConfig(signCertFile, signKeyFile, encCertFile, encKeyFile string) (*ziface.TLSConfig, error) {
	sigCert, err := gmtls.LoadGMX509KeyPairs(signCertFile, signKeyFile, encCertFile, encKeyFile)
	if err != nil {
		return nil, errors.New("gmtls: failed to load GM certificate pair: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeGM,
		GMConfig: &gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{sigCert},
		},
	}, nil
}

// NewGMTLSConfigSingle 从单个 SM2 证书创建 GM-TLS 配置（签名和加密使用同一证书）。
func NewGMTLSConfigSingle(certFile, keyFile string) (*ziface.TLSConfig, error) {
	cert, err := gmtls.LoadGMX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.New("gmtls: failed to load GM certificate: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeGM,
		GMConfig: &gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{cert},
		},
	}, nil
}

// NewGMTLSConfigFromPEM 从 PEM 字节创建 GM-TLS 配置（双证书）。
func NewGMTLSConfigFromPEM(signCertPEM, signKeyPEM, encCertPEM, encKeyPEM []byte) (*ziface.TLSConfig, error) {
	sigCert, err := gmtls.GMX509KeyPairs(signCertPEM, signKeyPEM, encCertPEM, encKeyPEM)
	if err != nil {
		return nil, errors.New("gmtls: failed to parse GM certificate pair: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeGM,
		GMConfig: &gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{sigCert},
		},
	}, nil
}

// NewGMTLSConfigFromPEMSingle 从 PEM 字节创建 GM-TLS 配置（单证书）。
func NewGMTLSConfigFromPEMSingle(certPEM, keyPEM []byte) (*ziface.TLSConfig, error) {
	cert, err := gmtls.GMX509KeyPairsSingle(certPEM, keyPEM)
	if err != nil {
		return nil, errors.New("gmtls: failed to parse GM certificate: " + err.Error())
	}
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeGM,
		GMConfig: &gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{cert},
		},
	}, nil
}

// DialTLS 建立 TLS 连接。
// 如果 cfg 为 nil 或 Mode 为 TLSModeNone，使用普通 net.Dial。
func DialTLS(network, addr string, cfg *ziface.TLSConfig) (net.Conn, error) {
	if cfg == nil || cfg.Mode == ziface.TLSModeNone {
		return net.Dial(network, addr)
	}
	switch cfg.Mode {
	case ziface.TLSModeStandard:
		return tls.Dial(network, addr, cfg.StdConfig)
	case ziface.TLSModeGM:
		return gmtls.Dial(network, addr, cfg.GMConfig)
	default:
		return net.Dial(network, addr)
	}
}

// NewClientTLSConfig 创建客户端 GM-TLS 配置（信创默认，启用证书验证）。
func NewClientTLSConfig() *ziface.TLSConfig {
	return &ziface.TLSConfig{
		Mode: ziface.TLSModeGM,
		GMConfig: &gmtls.Config{
			GMSupport: gmtls.NewGMSupport(),
		},
	}
}

// NewClientStandardTLSConfig 创建客户端标准 TLS 配置（非信创场景，启用证书验证）。
func NewClientStandardTLSConfig() *ziface.TLSConfig {
	return &ziface.TLSConfig{
		Mode:      ziface.TLSModeStandard,
		StdConfig: &tls.Config{},
	}
}
