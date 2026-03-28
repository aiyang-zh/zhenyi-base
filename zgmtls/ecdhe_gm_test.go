package gmtls

import (
	"bytes"
	"crypto"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
	x509 "github.com/emmansun/gmsm/smx509"
)

// VersionGMSSL + SM2 时 ServerKeyExchange 签名原文为 client_random || server_random || ECDH 参数，经 SM3。
func TestHashForServerKeyExchange_GMSSL_SM2_SM3(t *testing.T) {
	cr := make([]byte, 32)
	sr := make([]byte, 32)
	for i := range cr {
		cr[i] = byte(i)
	}
	for i := range sr {
		sr[i] = byte(i + 0x20)
	}
	// 与 generateServerKeyExchange 中 serverECDHParams 格式一致：named_curve(3) + CurveP256 + 65 字节未压缩点
	params := make([]byte, 4+65)
	params[0] = 3
	params[1] = byte(CurveP256 >> 8)
	params[2] = byte(CurveP256)
	params[3] = 65
	for i := 4; i < len(params); i++ {
		params[i] = byte(i)
	}

	got, err := hashForServerKeyExchange(signatureSM2, crypto.SHA1, VersionGMSSL, cr, sr, params)
	if err != nil {
		t.Fatal(err)
	}
	h := sm3.New()
	_, _ = h.Write(cr)
	_, _ = h.Write(sr)
	_, _ = h.Write(params)
	want := h.Sum(nil)
	if !bytes.Equal(got, want) {
		t.Fatalf("SM3 digest mismatch")
	}
}

// generateServerKeyExchange → processServerKeyExchange → generateClientKeyExchange → processClientKeyExchange 后双方 pre-master 一致。
func TestEcdheKeyAgreementGM_ECDHSharedSecretMatches(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	sign := &certs[0]
	leaf, err := x509.ParseCertificate(sign.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}

	kaSrv := &ecdheKeyAgreementGM{version: VersionGMSSL}
	ch := &clientHelloMsg{random: make([]byte, 32)}
	sh := &serverHelloMsg{random: make([]byte, 32)}
	if _, err := rand.Read(ch.random); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(sh.random); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	skx, err := kaSrv.generateServerKeyExchange(cfg, sign, sign, ch, sh)
	if err != nil {
		t.Fatalf("generateServerKeyExchange: %v", err)
	}
	if skx == nil || len(skx.key) < 8 {
		t.Fatalf("unexpected skx")
	}

	kaCli := &ecdheKeyAgreementGM{version: VersionGMSSL}
	if err := kaCli.processServerKeyExchange(cfg, ch, sh, leaf, skx); err != nil {
		t.Fatalf("processServerKeyExchange: %v", err)
	}

	pmCli, ckx, err := kaCli.generateClientKeyExchange(cfg, ch, nil)
	if err != nil {
		t.Fatalf("generateClientKeyExchange: %v", err)
	}
	if ckx == nil || len(ckx.ciphertext) < 2 {
		t.Fatalf("unexpected ckx")
	}

	pmSrv, err := kaSrv.processClientKeyExchange(cfg, sign, ckx, VersionGMSSL)
	if err != nil {
		t.Fatalf("processClientKeyExchange: %v", err)
	}
	if !bytes.Equal(pmCli, pmSrv) {
		t.Fatalf("ECDH pre-master mismatch: client %d bytes vs server %d bytes", len(pmCli), len(pmSrv))
	}
}

func TestEcdheKeyAgreementGM_processServerKeyExchange_sigLenBigEndian(t *testing.T) {
	ka := &ecdheKeyAgreementGM{version: VersionGMSSL}
	curve := sm2.P256()
	_, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := elliptic.Marshal(curve, x, y)
	serverECDH := make([]byte, 4+len(pub))
	serverECDH[0] = 3
	serverECDH[1] = byte(CurveP256 >> 8)
	serverECDH[2] = byte(CurveP256)
	serverECDH[3] = byte(len(pub))
	copy(serverECDH[4:], pub)
	// skx.key = params || sigLen(0x0100=256) || 仅 1 字节签名体 → sigLen+2 != len(sig)
	skx := &serverKeyExchangeMsg{key: append(append([]byte{}, serverECDH...), 0x01, 0x00, 0xab)}
	ch := &clientHelloMsg{}
	sh := &serverHelloMsg{}
	certs := newTestGMServerCertificates(t)
	leaf, err := x509.ParseCertificate(certs[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	err = ka.processServerKeyExchange(&Config{}, ch, sh, leaf, skx)
	if !errors.Is(err, errServerKeyExchange) {
		t.Fatalf("want errServerKeyExchange when sigLen+2 != len(sig), got %v", err)
	}
}
