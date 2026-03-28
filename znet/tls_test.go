package znet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateSelfSignedCert 生成自签名 RSA/ECDSA 证书用于测试
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

func writeTempFiles(t *testing.T, certPEM, keyPEM []byte) (certFile, keyFile string) {
	t.Helper()
	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	os.WriteFile(certFile, certPEM, 0600)
	os.WriteFile(keyFile, keyPEM, 0600)
	return
}

func TestTLSConfig_Nil_NoWrap(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var cfg *ziface.TLSConfig
	wrapped := cfg.WrapListener(ln)
	if wrapped != ln {
		t.Fatal("nil config should return original listener")
	}
}

func TestTLSConfig_ModeNone_NoWrap(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfg := &ziface.TLSConfig{Mode: ziface.TLSModeNone}
	wrapped := cfg.WrapListener(ln)
	if wrapped != ln {
		t.Fatal("TLSModeNone should return original listener")
	}
}

func TestNewStandardTLSConfig_FromPEM(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)
	cfg, err := NewStandardTLSConfigFromPEM(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("NewStandardTLSConfigFromPEM failed: %v", err)
	}
	if cfg.Mode != ziface.TLSModeStandard {
		t.Fatalf("expected TLSModeStandard, got %d", cfg.Mode)
	}
	if cfg.StdConfig == nil {
		t.Fatal("StdConfig should not be nil")
	}
}

func TestNewStandardTLSConfig_FromFile(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)
	certFile, keyFile := writeTempFiles(t, certPEM, keyPEM)

	cfg, err := NewStandardTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("NewStandardTLSConfig failed: %v", err)
	}
	if cfg.Mode != ziface.TLSModeStandard {
		t.Fatalf("expected TLSModeStandard, got %d", cfg.Mode)
	}
}

func TestNewStandardTLSConfig_InvalidFile(t *testing.T) {
	_, err := NewStandardTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
}

func TestNewStandardTLSConfigFromPEM_Invalid(t *testing.T) {
	_, err := NewStandardTLSConfigFromPEM([]byte("bad"), []byte("bad"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestDialTLS_Nil_PlainTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	conn, err := DialTLS("tcp", ln.Addr().String(), nil)
	if err != nil {
		t.Fatalf("DialTLS with nil config failed: %v", err)
	}
	conn.Close()
}

func TestDialTLS_ModeNone_PlainTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	conn, err := DialTLS("tcp", ln.Addr().String(), &ziface.TLSConfig{Mode: ziface.TLSModeNone})
	if err != nil {
		t.Fatalf("DialTLS with ModeNone failed: %v", err)
	}
	conn.Close()
}

func TestStandardTLS_EndToEnd(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)
	serverCfg, err := NewStandardTLSConfigFromPEM(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	tlsLn := serverCfg.WrapListener(ln)

	done := make(chan []byte, 1)
	go func() {
		conn, err := tlsLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		done <- buf[:n]
	}()

	// 测试中客户端显式信任自签名证书，而不是跳过验证。
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to append server cert to RootCAs")
	}
	clientCfg := &ziface.TLSConfig{
		Mode: ziface.TLSModeStandard,
		StdConfig: &tls.Config{
			RootCAs: rootCAs,
		},
	}
	conn, err := DialTLS("tcp", ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("DialTLS failed: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello tls")
	_, err = conn.Write(msg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	select {
	case received := <-done:
		if string(received) != string(msg) {
			t.Fatalf("got %q, want %q", received, msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server to receive data")
	}
}

func TestClientTLSConfig_DefaultIsGM(t *testing.T) {
	cfg := NewClientTLSConfig()
	if cfg.Mode != ziface.TLSModeGM {
		t.Fatalf("expected TLSModeGM (信创默认), got %d", cfg.Mode)
	}
	if cfg.GMConfig == nil {
		t.Fatal("GMConfig should not be nil")
	}
	if cfg.GMConfig.IsInsecureSkipVerify() {
		t.Fatal("client config should not skip verify by default")
	}
}

func TestClientStandardTLSConfig(t *testing.T) {
	cfg := NewClientStandardTLSConfig()
	if cfg.Mode != ziface.TLSModeStandard {
		t.Fatalf("expected TLSModeStandard, got %d", cfg.Mode)
	}
	if cfg.StdConfig == nil {
		t.Fatal("StdConfig should not be nil")
	}
	if cfg.StdConfig.InsecureSkipVerify {
		t.Fatal("client config should not skip verify by default")
	}
}
