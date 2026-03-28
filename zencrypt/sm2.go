// Package zencrypt Package encrypt 提供SM2国密非对称加密和签名实现。
//
// SM2是中国国家密码管理局发布的椭圆曲线公钥密码算法标准（GM/T 0003-2012），
// 基于256位素数域椭圆曲线，安全强度等同于ECC P-256。
//
// 加密示例:
//
//	privKey, _ := encrypt.GenerateSM2Key()
//	pubKey := privKey.PublicKey()
//
//	ciphertext, _ := encrypt.SM2Encrypt(pubKey, []byte("secret data"))
//	plaintext, _ := encrypt.SM2Decrypt(privKey, ciphertext)
//
// 签名示例:
//
//	privKey, _ := encrypt.GenerateSM2Key()
//	signature, _ := encrypt.SM2Sign(privKey, []byte("data"))
//	err := encrypt.SM2Verify(privKey.PublicKey(), []byte("data"), signature)
//
// 密钥PEM序列化:
//
//	privPem := encrypt.SM2MarshalPrivateKey(privKey)
//	pubPem := encrypt.SM2MarshalPublicKey(privKey.PublicKey())
//
//	privKey2, _ := encrypt.SM2ParsePrivateKey(privPem)
//	pubKey2, _ := encrypt.SM2ParsePublicKey(pubPem)
package zencrypt

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/pem"
	"errors"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
)

// SM2PrivateKey 封装 SM2 私钥，对外不暴露底层 *sm2.PrivateKey，便于后续替换实现。
type SM2PrivateKey struct {
	k *sm2.PrivateKey
}

// SM2PublicKey 封装 SM2 公钥。
type SM2PublicKey struct {
	k *ecdsa.PublicKey
}

// PublicKey 返回与私钥对应的公钥封装。
func (p *SM2PrivateKey) PublicKey() *SM2PublicKey {
	if p == nil || p.k == nil {
		return nil
	}
	return &SM2PublicKey{k: &p.k.PublicKey}
}

// GenerateSM2Key 生成SM2密钥对。
func GenerateSM2Key() (*SM2PrivateKey, error) {
	k, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &SM2PrivateKey{k: k}, nil
}

// SM2Encrypt 使用SM2公钥加密数据。
//
// SM2加密基于椭圆曲线，没有RSA的明文大小限制（理论上可加密任意长度数据，
// 但实际建议配合对称加密使用信封加密模式）。
func SM2Encrypt(pubKey *SM2PublicKey, plaintext []byte) ([]byte, error) {
	if pubKey == nil || pubKey.k == nil {
		return nil, errors.New("sm2: public key is nil")
	}
	if len(plaintext) == 0 {
		return nil, errors.New("sm2: empty plaintext")
	}
	return sm2.Encrypt(rand.Reader, pubKey.k, plaintext, sm2.NewPlainEncrypterOpts(sm2.MarshalUncompressed, sm2.C1C3C2))
}

// SM2Decrypt 使用SM2私钥解密数据。
func SM2Decrypt(privKey *SM2PrivateKey, ciphertext []byte) ([]byte, error) {
	if privKey == nil || privKey.k == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	if len(ciphertext) == 0 {
		return nil, errors.New("sm2: empty ciphertext")
	}
	return sm2.Decrypt(privKey.k, ciphertext)
}

// SM2Sign 使用SM2私钥对数据签名。
//
// 返回DER编码的签名。签名过程内部使用 SM2 标准签名（含默认 UID 与 SM3 摘要）。
func SM2Sign(privKey *SM2PrivateKey, data []byte) ([]byte, error) {
	if privKey == nil || privKey.k == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	return privKey.k.Sign(rand.Reader, data, sm2.DefaultSM2SignerOpts)
}

// SM2Verify 使用SM2公钥验证签名。
//
// 签名有效返回nil，无效返回错误。
func SM2Verify(pubKey *SM2PublicKey, data []byte, signature []byte) error {
	if pubKey == nil || pubKey.k == nil {
		return errors.New("sm2: public key is nil")
	}
	if !sm2.VerifyASN1WithSM2(pubKey.k, nil, data, signature) {
		return errors.New("sm2: signature verification failed")
	}
	return nil
}

// SM2MarshalPrivateKey 将SM2私钥序列化为PEM格式。
func SM2MarshalPrivateKey(privKey *SM2PrivateKey) ([]byte, error) {
	if privKey == nil || privKey.k == nil {
		return nil, errors.New("sm2: private key is nil")
	}
	der, err := smx509.MarshalPKCS8PrivateKey(privKey.k)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// SM2ParsePrivateKey 从PEM格式解析SM2私钥。
func SM2ParsePrivateKey(pemData []byte) (*SM2PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("sm2: failed to decode PEM block")
	}
	key, err := smx509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(*sm2.PrivateKey)
	if !ok {
		return nil, errors.New("sm2: not an SM2 private key")
	}
	return &SM2PrivateKey{k: priv}, nil
}

// SM2MarshalPublicKey 将SM2公钥序列化为PEM格式。
func SM2MarshalPublicKey(pubKey *SM2PublicKey) ([]byte, error) {
	if pubKey == nil || pubKey.k == nil {
		return nil, errors.New("sm2: public key is nil")
	}
	der, err := smx509.MarshalPKIXPublicKey(pubKey.k)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// SM2ParsePublicKey 从PEM格式解析SM2公钥。
func SM2ParsePublicKey(pemData []byte) (*SM2PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("sm2: failed to decode PEM block")
	}
	pub, err := smx509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := pub.(*ecdsa.PublicKey)
	if !ok || !sm2.IsSM2PublicKey(ek) {
		return nil, errors.New("sm2: not an SM2 public key")
	}
	return &SM2PublicKey{k: ek}, nil
}
