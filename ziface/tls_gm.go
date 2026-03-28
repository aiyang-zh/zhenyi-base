package ziface

import (
	"errors"
	"net"

	gmx509 "github.com/emmansun/gmsm/smx509"

	"github.com/aiyang-zh/zhenyi-base/zgmtls"
)

// GMTLSConfig 封装国密 GM-TLS 底层配置，避免对外暴露 *gmtls.Config，便于将来替换实现而不破坏调用方。
type GMTLSConfig struct {
	inner *gmtls.Config
}

// Dial 使用本配置建立 GM-TLS 客户端连接。
func (g *GMTLSConfig) Dial(network, addr string) (net.Conn, error) {
	if g == nil || g.inner == nil {
		return nil, errors.New("ziface: nil GMTLSConfig")
	}
	return gmtls.Dial(network, addr, g.inner)
}

func (g *GMTLSConfig) wrapListener(ln net.Listener) net.Listener {
	if g == nil || g.inner == nil {
		return ln
	}
	return gmtls.NewListener(ln, g.inner)
}

// SetInsecureSkipVerify 跳过服务端证书与主机名校验（仅测试/自签场景；生产请配置可信根）。
func (g *GMTLSConfig) SetInsecureSkipVerify(skip bool) {
	if g == nil || g.inner == nil {
		return
	}
	g.inner.InsecureSkipVerify = skip
}

// SetRootCAsPEM 从 PEM 设置国密客户端信任根（可含多段证书）。
func (g *GMTLSConfig) SetRootCAsPEM(pem []byte) error {
	if g == nil || g.inner == nil {
		return errors.New("ziface: nil GMTLSConfig")
	}
	pool := gmx509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return errors.New("ziface: no certificates in PEM")
	}
	g.inner.RootCAs = pool
	return nil
}

// IsInsecureSkipVerify 返回是否跳过证书校验（便于测试与诊断）。
func (g *GMTLSConfig) IsInsecureSkipVerify() bool {
	if g == nil || g.inner == nil {
		return false
	}
	return g.inner.InsecureSkipVerify
}

func newGMTLSConfig(inner *gmtls.Config) *GMTLSConfig {
	if inner == nil {
		return nil
	}
	return &GMTLSConfig{inner: inner}
}

// dupGMTLSCertificate 复制证书链与相关字节切片，使多条 gmtls.Certificate 不共享 [][]byte 底层，
// 降低握手路径若原地修改 DER 时互相影响的风险。PrivateKey、Leaf 仍与传入值一致（同一密钥与叶子证书解析结果）。
func dupGMTLSCertificate(c gmtls.Certificate) gmtls.Certificate {
	out := c
	if c.Certificate != nil {
		out.Certificate = make([][]byte, len(c.Certificate))
		for i, der := range c.Certificate {
			cp := make([]byte, len(der))
			copy(cp, der)
			out.Certificate[i] = cp
		}
	}
	if len(c.OCSPStaple) > 0 {
		cp := make([]byte, len(c.OCSPStaple))
		copy(cp, c.OCSPStaple)
		out.OCSPStaple = cp
	}
	if c.SignedCertificateTimestamps != nil {
		out.SignedCertificateTimestamps = make([][]byte, len(c.SignedCertificateTimestamps))
		for i, s := range c.SignedCertificateTimestamps {
			cp := make([]byte, len(s))
			copy(cp, s)
			out.SignedCertificateTimestamps[i] = cp
		}
	}
	return out
}

// NewGMTLSServerTLSFromFiles 从 SM2 双证书文件构建服务端 GM-TLS（Mode=TLSModeGM）。
// 签名与加密各需一对证书/私钥，底层握手要求 Certificates 至少两条。
func NewGMTLSServerTLSFromFiles(signCertFile, signKeyFile, encCertFile, encKeyFile string) (*TLSConfig, error) {
	signCert, err := gmtls.LoadGMX509KeyPair(signCertFile, signKeyFile)
	if err != nil {
		return nil, errors.New("gmtls: failed to load GM sign certificate: " + err.Error())
	}
	encCert, err := gmtls.LoadGMX509KeyPair(encCertFile, encKeyFile)
	if err != nil {
		return nil, errors.New("gmtls: failed to load GM enc certificate: " + err.Error())
	}
	return &TLSConfig{
		Mode: TLSModeGM,
		GMConfig: newGMTLSConfig(&gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{signCert, encCert},
		}),
	}, nil
}

// NewGMTLSServerTLSFromSingleFile 从单个 SM2 证书文件构建服务端 GM-TLS（同一证书对复制为签名与加密各一条）。
func NewGMTLSServerTLSFromSingleFile(certFile, keyFile string) (*TLSConfig, error) {
	cert, err := gmtls.LoadGMX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.New("gmtls: failed to load GM certificate: " + err.Error())
	}
	return &TLSConfig{
		Mode: TLSModeGM,
		GMConfig: newGMTLSConfig(&gmtls.Config{
			GMSupport: gmtls.NewGMSupport(),
			// 单证书双槽：底层需两条 Certificate；各槽使用独立 DER 副本，见 dupGMTLSCertificate。
			Certificates: []gmtls.Certificate{dupGMTLSCertificate(cert), dupGMTLSCertificate(cert)},
		}),
	}, nil
}

// NewGMTLSServerTLSFromPEM 从 PEM 字节构建双证书服务端 GM-TLS。
func NewGMTLSServerTLSFromPEM(signCertPEM, signKeyPEM, encCertPEM, encKeyPEM []byte) (*TLSConfig, error) {
	signCert, err := gmtls.GMX509KeyPairsSingle(signCertPEM, signKeyPEM)
	if err != nil {
		return nil, errors.New("gmtls: failed to parse GM sign certificate: " + err.Error())
	}
	encCert, err := gmtls.GMX509KeyPairsSingle(encCertPEM, encKeyPEM)
	if err != nil {
		return nil, errors.New("gmtls: failed to parse GM enc certificate: " + err.Error())
	}
	return &TLSConfig{
		Mode: TLSModeGM,
		GMConfig: newGMTLSConfig(&gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{signCert, encCert},
		}),
	}, nil
}

// NewGMTLSServerTLSFromPEMSingle 从 PEM 单证书构建服务端 GM-TLS。
func NewGMTLSServerTLSFromPEMSingle(certPEM, keyPEM []byte) (*TLSConfig, error) {
	cert, err := gmtls.GMX509KeyPairsSingle(certPEM, keyPEM)
	if err != nil {
		return nil, errors.New("gmtls: failed to parse GM certificate: " + err.Error())
	}
	return &TLSConfig{
		Mode: TLSModeGM,
		GMConfig: newGMTLSConfig(&gmtls.Config{
			GMSupport:    gmtls.NewGMSupport(),
			Certificates: []gmtls.Certificate{dupGMTLSCertificate(cert), dupGMTLSCertificate(cert)},
		}),
	}, nil
}

// NewClientGMTLSTLS 创建客户端 GM-TLS 配置（信创默认，启用证书验证）。
func NewClientGMTLSTLS() *TLSConfig {
	return &TLSConfig{
		Mode: TLSModeGM,
		GMConfig: newGMTLSConfig(&gmtls.Config{
			GMSupport: gmtls.NewGMSupport(),
		}),
	}
}
