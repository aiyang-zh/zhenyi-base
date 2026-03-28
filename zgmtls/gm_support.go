// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gmtls

import (
	"crypto"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/sm3"
	"github.com/emmansun/gmsm/sm4"
	x509 "github.com/emmansun/gmsm/smx509"
)

const VersionGMSSL = 0x0101 // GM/T 0024-2014

func getCAs() []*x509.Certificate {
	// 不注入内置 CA；信任根仅来自 Config.RootCAs（历史修改：remove pre insert ca certs）。
	return nil
}

// A list of cipher suite IDs that are, or have been, implemented by this
// package.
const (
	//GM crypto suites ID  Taken from GM/T 0024-2014
	GMTLS_ECDHE_SM2_WITH_SM1_SM3 uint16 = 0xe001
	GMTLS_SM2_WITH_SM1_SM3       uint16 = 0xe003
	GMTLS_IBSDH_WITH_SM1_SM3     uint16 = 0xe005
	GMTLS_IBC_WITH_SM1_SM3       uint16 = 0xe007
	GMTLS_RSA_WITH_SM1_SM3       uint16 = 0xe009
	GMTLS_RSA_WITH_SM1_SHA1      uint16 = 0xe00a
	GMTLS_ECDHE_SM2_WITH_SM4_SM3 uint16 = 0xe011
	GMTLS_SM2_WITH_SM4_SM3       uint16 = 0xe013
	GMTLS_IBSDH_WITH_SM4_SM3     uint16 = 0xe015
	GMTLS_IBC_WITH_SM4_SM3       uint16 = 0xe017
	GMTLS_RSA_WITH_SM4_SM3       uint16 = 0xe019
	GMTLS_RSA_WITH_SM4_SHA1      uint16 = 0xe01a
)

var gmCipherSuites = []*cipherSuite{
	{GMTLS_SM2_WITH_SM4_SM3, 16, 32, 16, eccGMKA, suiteECDSA, cipherSM4, macSM3, nil},
	{GMTLS_ECDHE_SM2_WITH_SM4_SM3, 16, 32, 16, ecdheGMKA, suiteECDHE | suiteECDSA, cipherSM4, macSM3, nil},
}

func getCipherSuites(c *Config) []uint16 {
	s := c.CipherSuites
	if s == nil {
		// 优先 ECDHE_SM2（临时密钥），回退 ECC_SM2（加密证书密钥封装）
		s = []uint16{GMTLS_ECDHE_SM2_WITH_SM4_SM3, GMTLS_SM2_WITH_SM4_SM3}
	}
	return s
}

func cipherSM4(key, iv []byte, isRead bool) interface{} {
	block, _ := sm4.NewCipher(key)
	if isRead {
		return cipher.NewCBCDecrypter(block, iv)
	}
	return cipher.NewCBCEncrypter(block, iv)
}

// macSHA1 returns a macFunction for the given protocol version.
func macSM3(version uint16, key []byte) macFunction {
	return tls10MAC{hmac.New(sm3.New, key)}
}

// used for adapt the demand of finishHash write
type nilMD5Hash struct{}

func (nilMD5Hash) Write(p []byte) (n int, err error) {
	return 0, nil
}

func (nilMD5Hash) Sum(b []byte) []byte {
	return nil
}

func (nilMD5Hash) Reset() {
}

func (nilMD5Hash) Size() int {
	return 0
}

func (nilMD5Hash) BlockSize() int {
	return 0
}

func newFinishedHashGM(cipherSuite *cipherSuite) finishedHash {
	return finishedHash{sm3.New(), sm3.New(), new(nilMD5Hash), new(nilMD5Hash), []byte{}, VersionGMSSL, prf12(sm3.New)}

}

func ecdheGMKA(version uint16) keyAgreement {
	return &ecdheKeyAgreementGM{
		version: version,
	}
}

func eccGMKA(version uint16) keyAgreement {
	return &eccKeyAgreementGM{
		version: version,
	}
}

// mutualCipherSuite returns a cipherSuite given a list of supported
// ciphersuites and the id requested by the peer.
func mutualCipherSuiteGM(have []uint16, want uint16) *cipherSuite {
	for _, id := range have {
		if id == want {
			for _, suite := range gmCipherSuites {
				if suite.id == want {
					return suite
				}
			}
			return nil
		}
	}
	return nil
}

const (
	ModeGMSSLOnly  = "GMSSLOnly"  // 仅支持 国密SSL模式
	ModeAutoSwitch = "AutoSwitch" // GMSSL/TLS 自动切换模式
)

type GMSupport struct {
	WorkMode string // 工作模式
}

func NewGMSupport() *GMSupport {
	return &GMSupport{WorkMode: ModeGMSSLOnly}
}

func (support *GMSupport) GetVersion() uint16 {
	return VersionGMSSL
}

func (support *GMSupport) IsAvailable() bool {
	return true
}

func (support *GMSupport) cipherSuites() []*cipherSuite {
	return gmCipherSuites
}

// EnableMixMode 启用 GMSSL/TLS 自动切换的工作模式
func (support *GMSupport) EnableMixMode() {
	support.WorkMode = ModeAutoSwitch
}

// IsAutoSwitchMode 是否处于混合工作模式
// return true - GMSSL/TLS 均支持, false - 不处于混合模式
func (support *GMSupport) IsAutoSwitchMode() bool {
	return support.WorkMode == ModeAutoSwitch
}

// LoadGMX509KeyPairs reads and parses two public/private key pairs from pairs
// of files. The files must contain PEM encoded data. The certificate file
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain. On successful return, Certificate.Leaf will
// be nil because the parsed form of the certificate is not retained.
func LoadGMX509KeyPairs(certFile, keyFile, encCertFile, encKeyFile string) (Certificate, error) {
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return Certificate{}, err
	}
	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return Certificate{}, err
	}
	encCertPEMBlock, err := ioutil.ReadFile(encCertFile)
	if err != nil {
		return Certificate{}, err
	}
	encKeyPEMBlock, err := ioutil.ReadFile(encKeyFile)
	if err != nil {
		return Certificate{}, err
	}

	return GMX509KeyPairs(certPEMBlock, keyPEMBlock, encCertPEMBlock, encKeyPEMBlock)
}

// add by syl add sigle key pair sitiation
func LoadGMX509KeyPair(certFile, keyFile string) (Certificate, error) {
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return Certificate{}, err
	}
	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return Certificate{}, err
	}

	return GMX509KeyPairsSingle(certPEMBlock, keyPEMBlock)
}

////load sign/enc certs and sign/enc privatekey from one single file respectively
//func LoadGMX509KeyPairs2(certFile, keyFile string) (Certificate, error) {
//	certPEMBlock, err := ioutil.ReadFile(certFile)
//	if err != nil {
//		return Certificate{}, err
//	}
//	keyPEMBlock, err := ioutil.ReadFile(keyFile)
//	if err != nil {
//		return Certificate{}, err
//	}
//	encCertPEMBlock, err := ioutil.ReadFile(encCertFile)
//	if err != nil {
//		return Certificate{}, err
//	}
//	encKeyPEMBlock, err := ioutil.ReadFile(encKeyFile)
//	if err != nil {
//		return Certificate{}, err
//	}
//
//	return GMX509KeyPairs(certPEMBlock, keyPEMBlock, encCertPEMBlock, encKeyPEMBlock)
//}

func getCert(certPEMBlock []byte) ([][]byte, error) {

	var certs [][]byte
	var skippedBlockTypes []string
	for {
		var certDERBlock *pem.Block
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			certs = append(certs, certDERBlock.Bytes)
		} else {
			skippedBlockTypes = append(skippedBlockTypes, certDERBlock.Type)
		}
	}

	if len(certs) == 0 {
		if len(skippedBlockTypes) == 0 {
			return nil, errors.New("tls: failed to find any PEM data in certificate input")
		}
		if len(skippedBlockTypes) == 1 && strings.HasSuffix(skippedBlockTypes[0], "PRIVATE KEY") {
			return nil, errors.New("tls: failed to find certificate PEM data in certificate input, but did find a private key; PEM inputs may have been switched")
		}
		return nil, fmt.Errorf("tls: failed to find \"CERTIFICATE\" PEM block in certificate input after skipping PEM blocks of the following types: %v", skippedBlockTypes)
	}
	return certs, nil
}

func getKey(keyPEMBlock []byte) (*pem.Block, error) {
	var skippedBlockTypes []string
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyPEMBlock = pem.Decode(keyPEMBlock)
		if keyDERBlock == nil {
			if len(skippedBlockTypes) == 0 {
				return nil, errors.New("tls: failed to find any PEM data in key input")
			}
			if len(skippedBlockTypes) == 1 && skippedBlockTypes[0] == "CERTIFICATE" {
				return nil, errors.New("tls: found a certificate rather than a key in the PEM for the private key")
			}
			return nil, fmt.Errorf("tls: failed to find PEM block with type ending in \"PRIVATE KEY\" in key input after skipping PEM blocks of the following types: %v", skippedBlockTypes)
		}
		if keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
		skippedBlockTypes = append(skippedBlockTypes, keyDERBlock.Type)
	}
	return keyDERBlock, nil
}

func matchKeyCert(keyDERBlock *pem.Block, certDERBlock []byte) (crypto.PrivateKey, error) {
	// We don't need to parse the public key for TLS, but we so do anyway
	// to check that it looks sane and matches the private key.
	x509Cert, err := x509.ParseCertificate(certDERBlock)
	if err != nil {
		return nil, err
	}

	privateKey, err := parsePrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return nil, err
	}

	switch pub := x509Cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if !sm2.IsSM2PublicKey(pub) {
			return nil, errors.New("tls: expected SM2 public key in certificate")
		}
		priv, ok := privateKey.(*sm2.PrivateKey)
		if !ok {
			return nil, errors.New("tls: private key type does not match public key type")
		}
		if pub.X.Cmp(priv.PublicKey.X) != 0 || pub.Y.Cmp(priv.PublicKey.Y) != 0 {
			return nil, errors.New("tls: private key does not match public key")
		}
	default:
		return nil, errors.New("tls: unknown public key algorithm")
	}
	return privateKey, nil
}

// X509KeyPair parses a public/private key pair from a pair of
// PEM encoded data. On successful return, Certificate.Leaf will be nil because
// the parsed form of the certificate is not retained.
func GMX509KeyPairs(certPEMBlock, keyPEMBlock, encCertPEMBlock, encKeyPEMBlock []byte) (Certificate, error) {
	fail := func(err error) (Certificate, error) { return Certificate{}, err }

	var certificate Certificate

	signCerts, err := getCert(certPEMBlock)
	if err != nil {
		return certificate, err
	}
	if len(signCerts) == 0 {
		return certificate, errors.New("tls: failed to find any sign cert PEM data in cert input")
	}
	certificate.Certificate = append(certificate.Certificate, signCerts[0])

	encCerts, err := getCert(encCertPEMBlock)
	if err != nil {
		return certificate, err
	}
	if len(encCerts) == 0 {
		return certificate, errors.New("tls: failed to find any enc cert PEM data in cert input")
	}
	certificate.Certificate = append(certificate.Certificate, encCerts[0])

	keyDERBlock, err := getKey(keyPEMBlock)
	if err != nil {
		return certificate, err
	}

	certificate.PrivateKey, err = matchKeyCert(keyDERBlock, certificate.Certificate[0])
	if err != nil {
		return fail(err)
	}

	return certificate, nil
}

// one cert for enc and sign
func GMX509KeyPairsSingle(certPEMBlock, keyPEMBlock []byte) (Certificate, error) {
	fail := func(err error) (Certificate, error) { return Certificate{}, err }

	var certificate Certificate

	certs, err := getCert(certPEMBlock)
	if err != nil {
		return certificate, err
	}
	if len(certs) == 0 {
		return certificate, errors.New("tls: failed to find any sign cert PEM data in cert input")
	}
	checkCert, err := x509.ParseCertificate(certs[0])
	if err != nil {
		return certificate, errors.New("tls: failed to parse certificate")
	}

	// 非 SM2 证书走标准库 TLS 密钥对解析
	if pub, ok := checkCert.PublicKey.(*ecdsa.PublicKey); !ok || !sm2.IsSM2PublicKey(pub) {
		return X509KeyPair(certPEMBlock, keyPEMBlock)
	}

	certificate.Certificate = append(certificate.Certificate, certs[0]) //this is for sign and env

	keyDERBlock, err := getKey(keyPEMBlock)
	if err != nil {
		return certificate, err
	}

	certificate.PrivateKey, err = matchKeyCert(keyDERBlock, certificate.Certificate[0])
	if err != nil {
		return fail(err)
	}

	return certificate, nil
}
