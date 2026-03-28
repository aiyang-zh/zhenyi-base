package gmtls

import (
	"crypto/rand"
	"io"
	"testing"
)

func TestClientHelloMsg_marshalUnmarshal(t *testing.T) {
	m := &clientHelloMsg{
		vers:               VersionGMSSL,
		random:             make([]byte, 32),
		cipherSuites:       []uint16{GMTLS_SM2_WITH_SM4_SM3},
		compressionMethods: []uint8{compressionNone},
	}
	if _, err := io.ReadFull(rand.Reader, m.random); err != nil {
		t.Fatal(err)
	}
	raw := m.marshal()
	var m2 clientHelloMsg
	if !m2.unmarshal(raw) {
		t.Fatal("unmarshal failed")
	}
	if !m.equal(&m2) {
		t.Fatal("clientHello round-trip mismatch")
	}
}

func TestCertificateMsg_marshalUnmarshal(t *testing.T) {
	m := &certificateMsg{
		certificates: [][]byte{{9, 8, 7}, {6, 5, 4, 3}},
	}
	raw := m.marshal()
	var m2 certificateMsg
	if !m2.unmarshal(raw) {
		t.Fatal("certificateMsg unmarshal failed")
	}
	if !m.equal(&m2) {
		t.Fatal("certificateMsg round-trip mismatch")
	}
}

func TestServerHelloMsg_marshalUnmarshal(t *testing.T) {
	m := &serverHelloMsg{
		vers:              VersionGMSSL,
		random:            make([]byte, 32),
		cipherSuite:       GMTLS_SM2_WITH_SM4_SM3,
		compressionMethod: compressionNone,
	}
	if _, err := io.ReadFull(rand.Reader, m.random); err != nil {
		t.Fatal(err)
	}
	raw := m.marshal()
	var m2 serverHelloMsg
	if !m2.unmarshal(raw) {
		t.Fatal("unmarshal failed")
	}
	if !m.equal(&m2) {
		t.Fatal("serverHello round-trip mismatch")
	}
}
