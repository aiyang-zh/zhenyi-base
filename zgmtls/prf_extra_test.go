package gmtls

import (
	"bytes"
	"crypto"
	"testing"
)

func TestSplitPreMasterSecret(t *testing.T) {
	sec := []byte{1, 2, 3, 4, 5, 6}
	s1, s2 := splitPreMasterSecret(sec)
	if len(s1) != (len(sec)+1)/2 || len(s2) != len(sec)-len(sec)/2 {
		t.Fatalf("unexpected split lengths: len(s1)=%d len(s2)=%d", len(s1), len(s2))
	}
	if s1[0] != 1 || s2[0] != 4 {
		t.Fatal("split boundaries mismatch")
	}
}

func TestLookupTLSHash(t *testing.T) {
	h, err := lookupTLSHash(PKCS1WithSHA256)
	if err != nil || h != crypto.SHA256 {
		t.Fatalf("lookupTLSHash PKCS1WithSHA256: h=%v err=%v", h, err)
	}
	_, err = lookupTLSHash(SignatureScheme(0xdead))
	if err == nil {
		t.Fatal("expected error for unknown scheme")
	}
}

func TestPrfForVersion_GM(t *testing.T) {
	suite := gmCipherSuites[0]
	f := prfForVersion(VersionGMSSL, suite)
	out := make([]byte, 16)
	f(out, []byte("secret"), []byte("label"), []byte("seed"))
	if bytes.Equal(out, make([]byte, 16)) {
		t.Fatal("PRF output should not be all zeros")
	}
}

func TestPrf10(t *testing.T) {
	out := make([]byte, 32)
	prf10(out, []byte("premaster"), []byte("label"), []byte("seed"))
	if bytes.Equal(out, make([]byte, 32)) {
		t.Fatal("prf10 produced all-zero output")
	}
}

func TestPrf30(t *testing.T) {
	out := make([]byte, 48)
	prf30(out, []byte("secret"), []byte("lab"), []byte("seed"))
	if bytes.Equal(out, make([]byte, 48)) {
		t.Fatal("prf30 produced all-zero output")
	}
}

func TestPrfAndHashForVersion_TLS12(t *testing.T) {
	suite := gmCipherSuites[0]
	prf, h := prfAndHashForVersion(VersionTLS12, suite)
	if prf == nil || h != crypto.SHA256 {
		t.Fatalf("unexpected TLS1.2 PRF/hash: %v %v", prf != nil, h)
	}
}
