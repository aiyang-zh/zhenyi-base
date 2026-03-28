package gmtls

import (
	"errors"
	"testing"

	x509 "github.com/emmansun/gmsm/smx509"
)

// eccKeyAgreementGM 使用 ciphertext[0:2] / skx.key[0:2] 作为 16 位大端长度；须用 uint16 拼接，避免 uint8<<8 恒为 0。
func TestEccKeyAgreementGM_processClientKeyExchange_lengthBigEndian(t *testing.T) {
	ka := &eccKeyAgreementGM{}

	// 声明长度 0x0100 = 256 字节密文，实际仅 255 字节 → 长度校验失败（应在解密之前返回）
	payload := make([]byte, 255)
	ct := make([]byte, 2+len(payload))
	ct[0], ct[1] = 0x01, 0x00
	copy(ct[2:], payload)

	ckx := &clientKeyExchangeMsg{ciphertext: ct}
	_, err := ka.processClientKeyExchange(&Config{}, &Certificate{}, ckx, VersionGMSSL)
	if !errors.Is(err, errClientKeyExchange) {
		t.Fatalf("want errClientKeyExchange for length mismatch, got %v", err)
	}
}

func TestEccKeyAgreementGM_processServerKeyExchange_sigLenBigEndian(t *testing.T) {
	ka := &eccKeyAgreementGM{}

	// skx.key[0:2]=0x0100 → 签名长度 256，总长应为 2+256；实际仅 3 字节 → 长度校验失败
	skx := &serverKeyExchangeMsg{key: []byte{0x01, 0x00, 0xab}}
	err := ka.processServerKeyExchange(&Config{}, &clientHelloMsg{}, &serverHelloMsg{}, &x509.Certificate{}, skx)
	if !errors.Is(err, errServerKeyExchange) {
		t.Fatalf("want errServerKeyExchange when sigLen+2 != len(key), got %v", err)
	}
}

func TestEccKeyAgreementGM_processClientKeyExchange_lengthLowByteOnlyStillConsistent(t *testing.T) {
	ka := &eccKeyAgreementGM{}

	// 0x0005 = 5 字节密文，总长 7；少 1 字节 → 长度不一致
	ct := []byte{0x00, 0x05, 1, 2, 3} // 仅 3 字节 payload，声明 5
	ckx := &clientKeyExchangeMsg{ciphertext: ct}
	_, err := ka.processClientKeyExchange(&Config{}, &Certificate{}, ckx, VersionGMSSL)
	if !errors.Is(err, errClientKeyExchange) {
		t.Fatalf("want errClientKeyExchange, got %v", err)
	}
}
