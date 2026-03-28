package gmtls

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	x509 "github.com/emmansun/gmsm/smx509"
)

func testSM2CertAndKeyPEM(tb testing.TB) (certPEM, keyPEM []byte) {
	tb.Helper()
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:       big.NewInt(1),
		Subject:            pkix.Name{CommonName: "pem-test"},
		NotBefore:          time.Now().Add(-time.Hour),
		NotAfter:           time.Now().Add(24 * time.Hour),
		SignatureAlgorithm: x509.SM2WithSM3,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		tb.Fatal(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		tb.Fatal(err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func TestGMX509KeyPairsSingle_SM2(t *testing.T) {
	certPEM, keyPEM := testSM2CertAndKeyPEM(t)
	c, err := GMX509KeyPairsSingle(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("GMX509KeyPairsSingle: %v", err)
	}
	if len(c.Certificate) != 1 || c.PrivateKey == nil {
		t.Fatal("unexpected certificate content")
	}
}

func TestGMX509KeyPairs_DupEnc(t *testing.T) {
	certPEM, keyPEM := testSM2CertAndKeyPEM(t)
	c, err := GMX509KeyPairs(certPEM, keyPEM, certPEM, keyPEM)
	if err != nil {
		t.Fatalf("GMX509KeyPairs: %v", err)
	}
	if len(c.Certificate) != 2 {
		t.Fatalf("want 2 DER entries, got %d", len(c.Certificate))
	}
}

func TestX509KeyPair_SM2(t *testing.T) {
	certPEM, keyPEM := testSM2CertAndKeyPEM(t)
	c, err := X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	if len(c.Certificate) < 1 {
		t.Fatal("expected certificate chain")
	}
}

func TestLoadGMX509KeyPair(t *testing.T) {
	certPEM, keyPEM := testSM2CertAndKeyPEM(t)
	dir := t.TempDir()
	cf := filepath.Join(dir, "sign.pem")
	kf := filepath.Join(dir, "sign.key")
	if err := os.WriteFile(cf, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kf, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadGMX509KeyPair(cf, kf)
	if err != nil {
		t.Fatalf("LoadGMX509KeyPair: %v", err)
	}
	if len(c.Certificate) < 1 {
		t.Fatal("expected certificate")
	}
}

func TestLoadX509KeyPair(t *testing.T) {
	certPEM, keyPEM := testSM2CertAndKeyPEM(t)
	dir := t.TempDir()
	cf := filepath.Join(dir, "cert.pem")
	kf := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cf, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kf, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadX509KeyPair(cf, kf)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
	if len(c.Certificate) < 1 {
		t.Fatal("expected cert")
	}
}

func TestX509KeyPair_Errors(t *testing.T) {
	if _, err := X509KeyPair([]byte("nope"), []byte("nope")); err == nil {
		t.Fatal("expected error for garbage PEM")
	}
}
