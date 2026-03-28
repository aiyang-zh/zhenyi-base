package gmtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	cryptoX509 "crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestNewBasicAutoSwitchConfig_GetCertificate(t *testing.T) {
	pair := newTestGMServerCertificates(t)
	sig := &pair[0]
	enc := &pair[1]

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &cryptoX509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "std-tls"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     cryptoX509.KeyUsageDigitalSignature,
	}
	der, err := cryptoX509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	std := &Certificate{Certificate: [][]byte{der}, PrivateKey: priv}

	cfg, err := NewBasicAutoSwitchConfig(sig, enc, std)
	if err != nil {
		t.Fatalf("NewBasicAutoSwitchConfig: %v", err)
	}
	if cfg == nil || cfg.GetCertificate == nil || cfg.GetKECertificate == nil {
		t.Fatal("unexpected nil config fields")
	}

	gm, err := cfg.GetCertificate(&ClientHelloInfo{SupportedVersions: []uint16{VersionGMSSL}})
	if err != nil {
		t.Fatal(err)
	}
	if gm != sig {
		t.Fatal("GM branch should return SM2 sign cert")
	}

	nonGM, err := cfg.GetCertificate(&ClientHelloInfo{SupportedVersions: []uint16{VersionTLS12}})
	if err != nil {
		t.Fatal(err)
	}
	if nonGM != std {
		t.Fatal("non-GM branch should return standard cert")
	}

	ke, err := cfg.GetKECertificate(&ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if ke != enc {
		t.Fatal("GetKECertificate should return enc cert")
	}
}
