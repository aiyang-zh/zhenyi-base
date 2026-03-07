// Package zencrypt Package encrypt 提供SM2国密非对称加密和签名实现。
//
// SM2是中国国家密码管理局发布的椭圆曲线公钥密码算法标准（GM/T 0003-2012），
// 基于256位素数域椭圆曲线，安全强度等同于ECC P-256。
//
// 加密示例:
//
//	privKey, _ := encrypt.GenerateSM2Key()
//	pubKey := &privKey.PublicKey
//
//	ciphertext, _ := encrypt.SM2Encrypt(pubKey, []byte("secret data"))
//	plaintext, _ := encrypt.SM2Decrypt(privKey, ciphertext)
//
// 签名示例:
//
//	privKey, _ := encrypt.GenerateSM2Key()
//	signature, _ := encrypt.SM2Sign(privKey, []byte("data"))
//	err := encrypt.SM2Verify(&privKey.PublicKey, []byte("data"), signature)
//
// 密钥PEM序列化:
//
//	privPem := encrypt.SM2MarshalPrivateKey(privKey)
//	pubPem := encrypt.SM2MarshalPublicKey(&privKey.PublicKey)
//
//	privKey2, _ := encrypt.SM2ParsePrivateKey(privPem)
//	pubKey2, _ := encrypt.SM2ParsePublicKey(pubPem)
package zencrypt

import (
	"crypto/rand"
	"errors"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/x509"
)

// GenerateSM2Key 生成SM2密钥对。
func GenerateSM2Key() (*sm2.PrivateKey, error) {
	return sm2.GenerateKey(rand.Reader)
}

// SM2Encrypt 使用SM2公钥加密数据。
//
// SM2加密基于椭圆曲线，没有RSA的明文大小限制（理论上可加密任意长度数据，
// 但实际建议配合对称加密使用信封加密模式）。
func SM2Encrypt(pubKey *sm2.PublicKey, plaintext []byte) ([]byte, error) {
	if pubKey == nil {
		return nil, errors.New("sm2: public key is nil")
	}
	if len(plaintext) == 0 {
		return nil, errors.New("sm2: empty plaintext")
	}
	return sm2.Encrypt(pubKey, plaintext, rand.Reader, sm2.C1C3C2)
}

// SM2Decrypt 使用SM2私钥解密数据。
func SM2Decrypt(privKey *sm2.PrivateKey, ciphertext []byte) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	if len(ciphertext) == 0 {
		return nil, errors.New("sm2: empty ciphertext")
	}
	return sm2.Decrypt(privKey, ciphertext, sm2.C1C3C2)
}

// SM2Sign 使用SM2私钥对数据签名。
//
// 返回DER编码的签名。签名过程内部使用SM3做摘要。
func SM2Sign(privKey *sm2.PrivateKey, data []byte) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	return privKey.Sign(rand.Reader, data, nil)
}

// SM2Verify 使用SM2公钥验证签名。
//
// 签名有效返回nil，无效返回错误。
func SM2Verify(pubKey *sm2.PublicKey, data []byte, signature []byte) error {
	if pubKey == nil {
		return errors.New("sm2: public key is nil")
	}
	ok := pubKey.Verify(data, signature)
	if !ok {
		return errors.New("sm2: signature verification failed")
	}
	return nil
}

// SM2MarshalPrivateKey 将SM2私钥序列化为PEM格式。
func SM2MarshalPrivateKey(privKey *sm2.PrivateKey) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	return x509.WritePrivateKeyToPem(privKey, nil)
}

// SM2ParsePrivateKey 从PEM格式解析SM2私钥。
func SM2ParsePrivateKey(pemData []byte) (*sm2.PrivateKey, error) {
	return x509.ReadPrivateKeyFromPem(pemData, nil)
}

// SM2MarshalPublicKey 将SM2公钥序列化为PEM格式。
func SM2MarshalPublicKey(pubKey *sm2.PublicKey) ([]byte, error) {
	if pubKey == nil {
		return nil, errors.New("sm2: public key is nil")
	}
	return x509.WritePublicKeyToPem(pubKey)
}

// SM2ParsePublicKey 从PEM格式解析SM2公钥。
func SM2ParsePublicKey(pemData []byte) (*sm2.PublicKey, error) {
	return x509.ReadPublicKeyFromPem(pemData)
}
