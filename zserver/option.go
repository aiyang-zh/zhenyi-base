package zserver

import (
	"github.com/aiyang-zh/zhenyi-core/ziface"
	"github.com/aiyang-zh/zhenyi-core/znet"
)

type Option func(*Server)

func WithAddr(addr string) Option {
	return func(s *Server) { s.addr = addr }
}

func WithProtocol(p znet.ConnProtocol) Option {
	return func(s *Server) { s.protocol = p }
}

func WithWorkers(n int) Option {
	return func(s *Server) { s.workerSize = n }
}

func WithMaxConnections(n int64) Option {
	return func(s *Server) { s.maxConn = n }
}

// WithName 设置服务器名称（用于日志）
func WithName(name string) Option {
	return func(s *Server) { s.name = name }
}

// WithDirectDispatch 直连模式：handler 在读 goroutine 内直接执行，
// 不经过 worker pool。适用于 handler 极轻量且无阻塞的场景（如 Echo）。
func WithDirectDispatch() Option {
	return func(s *Server) { s.directDispatch = true }
}

// WithTLS 配置 GM-TLS（SM2 双证书，信创默认）。
//
//	server.New(server.WithTLS("sm2_sign.crt", "sm2_sign.key", "sm2_enc.crt", "sm2_enc.key"))
func WithTLS(signCertFile, signKeyFile, encCertFile, encKeyFile string) Option {
	return func(s *Server) {
		cfg, err := znet.NewGMTLSConfig(signCertFile, signKeyFile, encCertFile, encKeyFile)
		if err != nil {
			panic("server: GM-TLS config failed: " + err.Error())
		}
		s.tlsConfig = cfg
	}
}

// WithTLSSingle 配置 GM-TLS（SM2 单证书，签名和加密共用）。
//
//	server.New(server.WithTLSSingle("sm2.crt", "sm2.key"))
func WithTLSSingle(certFile, keyFile string) Option {
	return func(s *Server) {
		cfg, err := znet.NewGMTLSConfigSingle(certFile, keyFile)
		if err != nil {
			panic("server: GM-TLS config failed: " + err.Error())
		}
		s.tlsConfig = cfg
	}
}

// WithStandardTLS 配置标准 TLS（RSA/ECDSA 证书，非信创场景）。
func WithStandardTLS(certFile, keyFile string) Option {
	return func(s *Server) {
		cfg, err := znet.NewStandardTLSConfig(certFile, keyFile)
		if err != nil {
			panic("server: TLS config failed: " + err.Error())
		}
		s.tlsConfig = cfg
	}
}

// WithTLSConfig 直接传入 TLS 配置（适用于需要自定义高级配置的场景）。
func WithTLSConfig(cfg *ziface.TLSConfig) Option {
	return func(s *Server) { s.tlsConfig = cfg }
}
