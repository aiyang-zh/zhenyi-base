package gmtls

import (
	"bytes"
	"testing"
)

func TestSessionState_marshalUnmarshal(t *testing.T) {
	s := &sessionState{
		vers:         VersionGMSSL,
		cipherSuite:  GMTLS_SM2_WITH_SM4_SM3,
		masterSecret: bytes.Repeat([]byte{7}, 48),
		certificates: [][]byte{{1, 2, 3}, {4, 5}},
	}
	data := s.marshal()
	var s2 sessionState
	if !s2.unmarshal(data) {
		t.Fatal("unmarshal failed")
	}
	if !s.equal(&s2) {
		t.Fatal("round-trip mismatch")
	}
}

func TestSessionState_equal(t *testing.T) {
	s := &sessionState{vers: 1, cipherSuite: 2, masterSecret: []byte{1}}
	if !s.equal(s) {
		t.Fatal("self equal")
	}
	if s.equal(nil) {
		t.Fatal("expected false for nil")
	}
}
