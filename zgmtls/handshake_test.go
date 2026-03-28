package gmtls

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	x509 "github.com/emmansun/gmsm/smx509"
)

func newTestGMServerCertificates(tb testing.TB) []Certificate {
	tb.Helper()
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:       big.NewInt(1),
		Subject:            pkix.Name{CommonName: "zgmtls-test"},
		NotBefore:          time.Now().Add(-time.Hour),
		NotAfter:           time.Now().Add(24 * time.Hour),
		SignatureAlgorithm: x509.SM2WithSM3,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:           []string{"zgmtls-test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		tb.Fatalf("CreateCertificate: %v", err)
	}
	c := Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}
	// 默认构建为双证书服务端：签名与加密各一条；测试复用同一 SM2 证书与私钥。
	return []Certificate{c, c}
}

func TestGMTLS_Handshake_RoundTrip(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	serverCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	}
	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
	}

	c1, c2 := net.Pipe()
	errc := make(chan error, 2)
	go func() {
		srv := Server(c1, serverCfg)
		errc <- srv.Handshake()
	}()
	go func() {
		cl := Client(c2, clientCfg)
		errc <- cl.Handshake()
	}()
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			t.Fatalf("handshake: %v", err)
		}
	}
}

func TestGMTLS_Handshake_RoundTrip_ECDHEOnly(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	serverCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	}
	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
		CipherSuites:       []uint16{GMTLS_ECDHE_SM2_WITH_SM4_SM3},
	}
	c1, c2 := net.Pipe()
	errc := make(chan error, 2)
	go func() {
		srv := Server(c1, serverCfg)
		errc <- srv.Handshake()
	}()
	go func() {
		cl := Client(c2, clientCfg)
		errc <- cl.Handshake()
	}()
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			t.Fatalf("handshake: %v", err)
		}
	}
}

func TestGMTLS_Handshake_RoundTrip_ECCOnly(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	serverCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
		CipherSuites: []uint16{GMTLS_SM2_WITH_SM4_SM3},
	}
	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
		CipherSuites:       []uint16{GMTLS_SM2_WITH_SM4_SM3},
	}
	c1, c2 := net.Pipe()
	errc := make(chan error, 2)
	go func() {
		srv := Server(c1, serverCfg)
		errc <- srv.Handshake()
	}()
	go func() {
		cl := Client(c2, clientCfg)
		errc <- cl.Handshake()
	}()
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			t.Fatalf("handshake: %v", err)
		}
	}
}

func BenchmarkGMTLS_Handshake_RoundTrip(b *testing.B) {
	certs := newTestGMServerCertificates(b)
	serverCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	}
	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c1, c2 := net.Pipe()
		errc := make(chan error, 2)
		go func() {
			srv := Server(c1, serverCfg)
			errc <- srv.Handshake()
		}()
		go func() {
			cl := Client(c2, clientCfg)
			errc <- cl.Handshake()
		}()
		for j := 0; j < 2; j++ {
			if err := <-errc; err != nil {
				b.Fatalf("handshake: %v", err)
			}
		}
	}
}
