// Package zencrypt Package encrypt 提供RSA加密和签名工具。
//
// # 安全建议
//
// 对于加密，请使用OAEP模式（EncryptOAEP/DecryptOAEP）而不是PKCS1v15。
// PKCS1v15已被弃用，因为容易受到选择密文攻击。
//
// # 使用示例
//
// 基础加密（推荐 - OAEP）：
//
//	privKey, _ := encrypt.GenerateSecretKey()
//	pubKey := &privKey.PublicKey
//	ciphertext, _ := encrypt.EncryptOAEP(pubKey, plaintext)
//	decrypted, _ := encrypt.DecryptOAEP(privKey, ciphertext)
//
// 带Label的加密（防重放）：
//
//	label := []byte("user:12345")  // 绑定到特定用户
//	ciphertext, _ := encrypt.EncryptOAEPWithLabel(pubKey, plaintext, label)
//	decrypted, _ := encrypt.DecryptOAEPWithLabel(privKey, ciphertext, label)
//
// 自定义哈希函数（大密钥）：
//
//	// ⚠️ 每次调用必须创建新的Hash实例，不能复用
//	ciphertext, _ := encrypt.EncryptOAEPWithHash(pubKey, sha512.New(), plaintext)
//	decrypted, _ := encrypt.DecryptOAEPWithHash(privKey, sha512.New(), ciphertext)
//
// 数字签名：
//
//	privKey, _ := encrypt.GenerateSecretKey()
//	signature, _ := encrypt.Sign(privKey, data)
//	err := encrypt.Verify(&privKey.PublicKey, data, signature)
package zencrypt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"hash"
)

// GenerateSecretKey 生成一个新的2048位RSA私钥。
//
// 对于更高的安全要求，建议使用3072或4096位。
func GenerateSecretKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// GenerateSecretKeyWithBits 生成指定位数的RSA私钥。
//
// 常用大小：2048（标准）、3072（高安全）、4096（非常高安全）。
func GenerateSecretKeyWithBits(bits int) (*rsa.PrivateKey, error) {
	if bits < 2048 {
		return nil, errors.New("密钥大小至少为2048位")
	}
	return rsa.GenerateKey(rand.Reader, bits)
}

// EncryptOAEP 使用RSA-OAEP和SHA-256加密数据。
//
// OAEP（最优非对称加密填充）是推荐的RSA加密填充方案。
// 它提供语义安全性，并能抵抗选择密文攻击。
//
// 最大明文大小 = (密钥大小/8) - 2*哈希大小 - 2
// 对于2048位密钥配合SHA-256：最大明文 = 190字节
//
// 如果明文超过密钥大小限制会返回错误。
func EncryptOAEP(pubKey *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, plaintext, nil)
}

// EncryptOAEPWithHash 使用RSA-OAEP和自定义哈希函数加密数据。
//
// 允许使用不同的哈希函数（例如，大密钥可以使用SHA-512）。
// 解密时必须使用相同的哈希函数。
//
// # 并发安全警告
//
// hash参数必须是一个新创建的Hash实例，不能在多个goroutine间共享。
// hash.Hash是有状态的且非线程安全。
//
// 正确用法：
//
//	// ✅ 每次调用创建新实例
//	ciphertext, _ := EncryptOAEPWithHash(pubKey, sha512.New(), plaintext)
//
// 错误用法：
//
//	// ❌ 不要复用Hash实例
//	h := sha512.New()
//	ciphertext1, _ := EncryptOAEPWithHash(pubKey, h, plaintext1)  // 第一次使用
//	ciphertext2, _ := EncryptOAEPWithHash(pubKey, h, plaintext2)  // ❌ h已被污染
func EncryptOAEPWithHash(pubKey *rsa.PublicKey, hash hash.Hash, plaintext []byte) ([]byte, error) {
	return rsa.EncryptOAEP(hash, rand.Reader, pubKey, plaintext, nil)
}

// EncryptOAEPWithLabel 使用RSA-OAEP、SHA-256和自定义Label加密数据。
//
// Label（也称为关联数据）是一个可选的公开字节序列，用于将密文绑定到特定上下文。
// Label不保密，但会参与密文生成，防止密文在不同上下文间的重放攻击。
//
// 典型用途：
//   - 将密文绑定到用户ID、会话ID等
//   - 防止跨协议或跨应用的密文重放
//   - 实现domain separation
//
// 解密时必须提供相同的Label，否则解密失败。
//
// 示例：
//
//	// 加密时绑定用户ID
//	label := []byte("user:12345")
//	ciphertext, _ := EncryptOAEPWithLabel(pubKey, plaintext, label)
//
//	// 解密时必须提供相同Label
//	plaintext, _ := DecryptOAEPWithLabel(privKey, ciphertext, label)
//
// 大多数简单场景可以传nil作为label（等同于EncryptOAEP）。
func EncryptOAEPWithLabel(pubKey *rsa.PublicKey, plaintext []byte, label []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, plaintext, label)
}

// DecryptOAEP 解密使用RSA-OAEP和SHA-256加密的数据。
//
// 如果解密失败或密文无效会返回错误。
func DecryptOAEP(privKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, ciphertext, nil)
}

// DecryptOAEPWithHash 使用RSA-OAEP和自定义哈希函数解密数据。
//
// 哈希函数必须与加密时使用的相同。
//
// # 并发安全警告
//
// hash参数必须是一个新创建的Hash实例，不能在多个goroutine间共享。
// hash.Hash是有状态的且非线程安全。
//
// 正确用法：
//
//	// ✅ 每次调用创建新实例
//	plaintext, _ := DecryptOAEPWithHash(privKey, sha512.New(), ciphertext)
func DecryptOAEPWithHash(privKey *rsa.PrivateKey, hash hash.Hash, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(hash, rand.Reader, privKey, ciphertext, nil)
}

// DecryptOAEPWithLabel 使用RSA-OAEP、SHA-256和自定义Label解密数据。
//
// Label必须与加密时使用的完全相同，否则解密失败。
//
// 示例：
//
//	label := []byte("user:12345")
//	plaintext, _ := DecryptOAEPWithLabel(privKey, ciphertext, label)
func DecryptOAEPWithLabel(privKey *rsa.PrivateKey, ciphertext []byte, label []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, ciphertext, label)
}

// Sign 使用RSA和SHA-256对数据进行签名。
//
// 创建一个可以用公钥验证的数字签名。
// 返回签名字节。
func Sign(privKey *rsa.PrivateKey, data []byte) ([]byte, error) {
	hashed := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed[:])
}

// Verify 使用SHA-256验证RSA签名。
//
// 如果签名有效返回nil，验证失败返回错误。
func Verify(pubKey *rsa.PublicKey, data []byte, signature []byte) error {
	hashed := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signature)
}

// Rsa2 提供带PEM密钥加载功能的RSA签名。
//
// 注意：尽管名为Rsa2.Encrypt，实际上执行的是签名而不是加密。
// 为了清晰起见，建议直接使用Sign函数。
type Rsa2 struct {
	PrivateKey *rsa.PrivateKey
}

// NewRsa2 创建一个新的Rsa2实例。
//
// 在使用Encrypt（Sign）之前必须调用LoadPrivateKey设置私钥。
func NewRsa2() *Rsa2 {
	return &Rsa2{}
}

// LoadPrivateKey 从PEM格式加载RSA私钥。
//
// 采用防御性解析策略：
//  1. 优先根据PEM块类型选择解析器（快速路径）
//  2. 如果类型不匹配或解析失败，尝试所有可能的解析方法（防御路径）
//  3. 返回第一个成功的解析结果
//
// 支持的格式：
//   - 标准PKCS#1: -----BEGIN RSA PRIVATE KEY-----
//   - 标准PKCS#8: -----BEGIN PRIVATE KEY-----
//   - 非标准头部但内容合法的PEM（例如某些工具生成的特殊Type）
//
// PEM格式示例：
//
//	-----BEGIN RSA PRIVATE KEY-----  (PKCS#1)
//	...
//	-----END RSA PRIVATE KEY-----
//
//	或
//
//	-----BEGIN PRIVATE KEY-----       (PKCS#8)
//	...
//	-----END PRIVATE KEY-----
//
// 防御性设计说明：
// 某些工具可能生成非标准的Type字符串，但内容是合法的密钥数据。
// 本实现会尝试多种解析方法，提供最大兼容性。
func (r *Rsa2) LoadPrivateKey(privateKey string) error {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return errors.New("解码PEM块失败：无效的PEM格式")
	}

	var privKey *rsa.PrivateKey
	var lastErr error

	// 策略1: 根据Type快速路径（常见情况，90%+）
	switch block.Type {
	case "RSA PRIVATE KEY":
		// 标准PKCS#1格式
		privKey, lastErr = x509.ParsePKCS1PrivateKey(block.Bytes)
		if lastErr == nil {
			r.PrivateKey = privKey
			return nil
		}
		// 如果失败，继续尝试其他方法（防御）

	case "PRIVATE KEY":
		// 标准PKCS#8格式
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			var ok bool
			privKey, ok = key.(*rsa.PrivateKey)
			if ok {
				r.PrivateKey = privKey
				return nil
			}
			lastErr = errors.New("PKCS#8密钥不是RSA类型")
		} else {
			lastErr = err
		}
		// 如果失败，继续尝试其他方法
	}

	// 策略2: 防御性尝试所有可能的解析方法（非标准情况）
	// 某些工具可能使用非标准的Type，但内容是合法的

	// 尝试PKCS#1（如果前面没成功）
	if privKey == nil {
		pk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err == nil {
			r.PrivateKey = pk
			return nil
		}
		lastErr = err
	}

	// 尝试PKCS#8（如果前面没成功）
	if privKey == nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			var ok bool
			privKey, ok = key.(*rsa.PrivateKey)
			if ok {
				r.PrivateKey = privKey
				return nil
			}
			lastErr = errors.New("密钥不是RSA类型")
		} else {
			lastErr = err
		}
	}

	// 所有方法都失败，返回详细错误信息
	return errors.New("无法解析RSA私钥: " + lastErr.Error() +
		" (PEM Type: \"" + block.Type + "\")")
}

// Encrypt 使用RSA和SHA-256对数据进行签名。
//
// 注意：尽管名为Encrypt，此方法实际执行签名而不是加密。
// 这是为了向后兼容而保留的。建议使用Sign函数以获得更清晰的语义。
//
// 返回可以用对应公钥验证的签名。
func (r *Rsa2) Encrypt(data []byte) ([]byte, error) {
	if r.PrivateKey == nil {
		return nil, errors.New("未加载私钥")
	}
	return Sign(r.PrivateKey, data)
}

// GetPublicKey 返回与已加载私钥对应的公钥。
func (r *Rsa2) GetPublicKey() (*rsa.PublicKey, error) {
	if r.PrivateKey == nil {
		return nil, errors.New("未加载私钥")
	}
	return &r.PrivateKey.PublicKey, nil
}
