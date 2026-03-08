package zserver

import (
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
)

// Option 是配置 Server 行为的可选项函数。
type Option func(*Server)

// WithAddr 设置服务器监听地址，例如 ":9001" 或 "127.0.0.1:9001"。
func WithAddr(addr string) Option {
	return func(s *Server) { s.addr = addr }
}

// WithProtocol 设置网络协议（TCP / WebSocket / KCP），默认使用 TCP。
func WithProtocol(p znet.ConnProtocol) Option {
	return func(s *Server) { s.protocol = p }
}

// WithWorkers 设置 worker pool 大小。
// 仅在未启用 WithDirectDispatch 时生效；默认值为 runtime.NumCPU()。
func WithWorkers(n int) Option {
	return func(s *Server) { s.workerSize = n }
}

// WithMaxConnections 设置最大连接数上限；为 0 或负数时不限制连接数。
func WithMaxConnections(n int64) Option {
	return func(s *Server) { s.maxConn = n }
}

// WithName 设置服务器名称（用于日志）
func WithName(name string) Option {
	return func(s *Server) { s.name = name }
}

// WithBanner 是否显示启动标识（默认 true）
func WithBanner(show bool) Option {
	return func(s *Server) { s.showBanner = show }
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
