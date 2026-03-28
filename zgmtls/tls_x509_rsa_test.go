package gmtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	cryptoX509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestX509KeyPair_RSA_PKCS1(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &cryptoX509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "rsa-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     cryptoX509.KeyUsageDigitalSignature,
	}
	der, err := cryptoX509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER := cryptoX509.MarshalPKCS1PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})
	c, err := X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair RSA: %v", err)
	}
	if _, ok := c.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Fatal("expected RSA private key")
	}
}

func TestX509KeyPair_ECDSA_P256(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &cryptoX509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "ecdsa-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     cryptoX509.KeyUsageDigitalSignature,
	}
	der, err := cryptoX509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := cryptoX509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	c, err := X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair ECDSA: %v", err)
	}
	if _, ok := c.PrivateKey.(*ecdsa.PrivateKey); !ok {
		t.Fatal("expected ECDSA private key")
	}
}
