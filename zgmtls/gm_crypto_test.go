package gmtls

import (
	"bytes"
	"crypto/cipher"
	"testing"
)

func TestGMSupport_Defaults(t *testing.T) {
	g := NewGMSupport()
	if g.GetVersion() != VersionGMSSL {
		t.Fatalf("GetVersion: got %x want %x", g.GetVersion(), VersionGMSSL)
	}
	if !g.IsAvailable() {
		t.Fatal("IsAvailable should be true")
	}
	if g.IsAutoSwitchMode() {
		t.Fatal("default should not be auto switch")
	}
	g.EnableMixMode()
	if !g.IsAutoSwitchMode() {
		t.Fatal("after EnableMixMode, IsAutoSwitchMode should be true")
	}
}

func TestMutualCipherSuiteGM(t *testing.T) {
	s := mutualCipherSuiteGM([]uint16{GMTLS_SM2_WITH_SM4_SM3}, GMTLS_SM2_WITH_SM4_SM3)
	if s == nil || s.id != GMTLS_SM2_WITH_SM4_SM3 {
		t.Fatalf("unexpected suite %v", s)
	}
	if mutualCipherSuiteGM([]uint16{GMTLS_ECDHE_SM2_WITH_SM4_SM3}, GMTLS_SM2_WITH_SM4_SM3) != nil {
		t.Fatal("expected nil when id not in have list")
	}
	if mutualCipherSuiteGM([]uint16{GMTLS_SM2_WITH_SM4_SM3}, 0x9999) != nil {
		t.Fatal("expected nil for unknown want id")
	}
}

func TestGetCipherSuites_Default(t *testing.T) {
	c := &Config{}
	got := getCipherSuites(c)
	if len(got) != 2 || got[0] != GMTLS_ECDHE_SM2_WITH_SM4_SM3 || got[1] != GMTLS_SM2_WITH_SM4_SM3 {
		t.Fatalf("default cipher suites: %v", got)
	}
}

func TestCipherSM4_CBC(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, 16)
	iv := bytes.Repeat([]byte{0xcd}, 16)
	plain := []byte("0123456789abcdef")
	enc := cipherSM4(key, iv, false).(cipher.BlockMode)
	dec := cipherSM4(key, iv, true).(cipher.BlockMode)
	dst := make([]byte, len(plain))
	enc.CryptBlocks(dst, plain)
	out := make([]byte, len(plain))
	dec.CryptBlocks(out, dst)
	if !bytes.Equal(out, plain) {
		t.Fatal("SM4 CBC round-trip failed")
	}
}

func TestMacSM3_TLS10MAC(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	m := macSM3(VersionGMSSL, key)
	seq := make([]byte, 8)
	hdr := make([]byte, 5)
	data := []byte("payload")
	extra := make([]byte, 64)
	sum := m.MAC(nil, seq, hdr, data, extra)
	if len(sum) == 0 {
		t.Fatal("expected non-empty MAC")
	}
}

func TestNewFinishedHashGM_Sum(t *testing.T) {
	suite := mutualCipherSuiteGM([]uint16{GMTLS_SM2_WITH_SM4_SM3}, GMTLS_SM2_WITH_SM4_SM3)
	if suite == nil {
		t.Fatal("suite")
	}
	h := newFinishedHashGM(suite)
	h.Write([]byte("clienthello"))
	h.Write([]byte("serverhello"))
	ms := make([]byte, 48)
	for i := range ms {
		ms[i] = byte(i)
	}
	client := h.clientSum(ms)
	server := h.serverSum(ms)
	if bytes.Equal(client, server) {
		t.Fatal("client and server finished sums should differ")
	}
}

func BenchmarkMutualCipherSuiteGM(b *testing.B) {
	have := []uint16{GMTLS_SM2_WITH_SM4_SM3, GMTLS_ECDHE_SM2_WITH_SM4_SM3}
	for i := 0; i < b.N; i++ {
		_ = mutualCipherSuiteGM(have, GMTLS_SM2_WITH_SM4_SM3)
	}
}

func BenchmarkMasterFromPreMasterSecret_GM(b *testing.B) {
	suite := gmCipherSuites[0]
	pms := make([]byte, 48)
	for i := range pms {
		pms[i] = byte(i)
	}
	cr := make([]byte, 32)
	sr := make([]byte, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = masterFromPreMasterSecret(VersionGMSSL, suite, pms, cr, sr)
	}
}

func BenchmarkKeysFromMasterSecret_GM(b *testing.B) {
	suite := gmCipherSuites[0]
	ms := make([]byte, 48)
	cr := make([]byte, 32)
	sr := make([]byte, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _, _ = keysFromMasterSecret(VersionGMSSL, suite, ms, cr, sr, 32, 16, 16)
	}
}
