// Package zencrypt Package enctypt 提供各种加密算法实现
//
// AES-CBC加密示例（传统模式）:
//
//	cipher := enctypt.NewAesEncrypt("your-secret-key")
//
//	// 加密
//	encrypted, err := cipher.Encrypt([]byte("secret data"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 解密
//	decrypted, err := cipher.Decrypt(encrypted)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// AES-GCM加密示例（推荐，现代标准）:
//
//	cipher := enctypt.NewAesGcmEncrypt("your-secret-key")
//
//	// 加密（自动提供完整性校验）
//	encrypted, err := cipher.Encrypt([]byte("secret data"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 解密（自动验证完整性）
//	decrypted, err := cipher.Decrypt(encrypted)
//	if err != nil {
//	    log.Fatal(err)  // 如果数据被篡改，这里会报错
//	}
//
// 注意事项:
//   - 密钥会通过SHA256派生为32字节
//   - CBC模式：IV随机生成并附加在密文前16字节，使用PKCS7填充
//   - GCM模式：Nonce随机生成并附加在密文前12字节，自带认证标签，无需填充
//   - 推荐使用GCM模式（更安全，更快，防篡改）
package zencrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"github.com/aiyang-zh/zhenyi-core/zpool"
	"io"
)

type AesEncrypt struct {
	SecretKey  string
	derivedKey []byte       // 预计算的32字节密钥，避免每次都SHA256
	block      cipher.Block // ✅ 缓存 cipher.Block，避免每次加密都创建
}

func NewAesEncrypt(key string) *AesEncrypt {
	derivedKey := Sha256(key)
	// ✅ 预创建 cipher.Block（AES-NI 加速）
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		panic("invalid key length: " + err.Error())
	}
	return &AesEncrypt{
		SecretKey:  key,
		derivedKey: derivedKey,
		block:      block,
	}
}

// Encrypt 使用AES-CBC加密（安全实现）
// 返回格式: [IV(16字节)][密文]
func (a *AesEncrypt) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	// ✅ 使用缓存的 cipher.Block（避免每次创建）
	block := a.block
	blockSize := block.BlockSize()

	// 填充
	paddedData := a.PKCS7Padding(data, blockSize)

	// ✅ 使用对象池减少内存分配
	bufSize := blockSize + len(paddedData)
	ciphertextBuf := zpool.GetBytesBuffer(bufSize)
	defer ciphertextBuf.Release()

	// 生成随机IV
	iv := ciphertextBuf.B[:blockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// 加密
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertextBuf.B[blockSize:], paddedData)

	// ✅ 深拷贝返回（池化缓冲会被归还）
	result := make([]byte, bufSize)
	copy(result, ciphertextBuf.B)
	return result, nil
}

// Decrypt 解密AES-CBC
// 输入格式: [IV(16字节)][密文]
func (a *AesEncrypt) Decrypt(data []byte) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// ✅ 使用缓存的 cipher.Block（避免每次创建）
	block := a.block
	blockSize := block.BlockSize()

	// 提取IV
	iv := data[:blockSize]
	ciphertext := data[blockSize:]

	if len(ciphertext)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	// ✅ 使用对象池减少内存分配
	plaintextBuf := zpool.GetBytesBuffer(len(ciphertext))
	defer plaintextBuf.Release()

	// 解密
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintextBuf.B, ciphertext)

	// 去填充（带验证）
	unpadded, err := a.PKCS7UnPaddingWithValidation(plaintextBuf.B)
	if err != nil {
		return nil, err
	}

	// ✅ 深拷贝返回（池化缓冲会被归还）
	result := make([]byte, len(unpadded))
	copy(result, unpadded)
	return result, nil
}

// PKCS7Padding 补码
func (a *AesEncrypt) PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

// PKCS7UnPaddingWithValidation 去填充（带验证）
func (a *AesEncrypt) PKCS7UnPaddingWithValidation(origData []byte) ([]byte, error) {
	length := len(origData)
	if length == 0 {
		return nil, errors.New("empty data")
	}

	unPadding := int(origData[length-1])

	// 验证填充
	if unPadding > length || unPadding == 0 {
		return nil, errors.New("invalid padding")
	}

	// 检查所有填充字节是否一致
	padding := origData[length-unPadding:]
	for _, b := range padding {
		if int(b) != unPadding {
			return nil, errors.New("invalid padding bytes")
		}
	}

	return origData[:length-unPadding], nil
}

// PKCS7UnPadding 去码（保留用于兼容性，不推荐使用）
// Deprecated: 使用 PKCS7UnPaddingWithValidation 代替
func (a *AesEncrypt) PKCS7UnPadding(origData []byte) []byte {
	result, err := a.PKCS7UnPaddingWithValidation(origData)
	if err != nil {
		// 为了兼容性，返回原始行为
		length := len(origData)
		if length == 0 {
			return origData
		}
		unPadding := int(origData[length-1])
		if unPadding > length {
			return origData
		}
		return origData[:(length - unPadding)]
	}
	return result
}

// ====================================
// AES-GCM 模式（推荐使用）
// ====================================

// AesGcmEncrypt AES-GCM加密器（认证加密，推荐使用）
// GCM模式优势：
//   - 自带完整性校验（防篡改）
//   - 无需填充（性能更好）
//   - 并行化加密（更快）
//   - 现代加密标准（TLS 1.3默认）
type AesGcmEncrypt struct {
	SecretKey  string
	derivedKey []byte
	block      cipher.Block // ✅ 缓存 cipher.Block
	gcm        cipher.AEAD  // ✅ 缓存 GCM 模式（AES-GCM 的核心）
}

// NewAesGcmEncrypt 创建AES-GCM加密器（推荐使用此方法）
func NewAesGcmEncrypt(key string) *AesGcmEncrypt {
	derivedKey := Sha256(key)

	// ✅ 预创建 cipher.Block 和 GCM
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		panic("invalid key length: " + err.Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic("failed to create GCM: " + err.Error())
	}

	return &AesGcmEncrypt{
		SecretKey:  key,
		derivedKey: derivedKey,
		block:      block,
		gcm:        gcm,
	}
}

// Encrypt 使用AES-GCM加密（认证加密）
// 返回格式: [Nonce(12字节)][密文+认证标签]
// GCM会自动添加16字节认证标签用于验证完整性
func (a *AesGcmEncrypt) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	// ✅ 使用缓存的 GCM（避免每次创建）
	gcm := a.gcm
	nonceSize := gcm.NonceSize()

	// ✅ 使用对象池减少内存分配
	nonceBuf := zpool.GetBytesBuffer(nonceSize)
	defer nonceBuf.Release()

	// 生成随机nonce（GCM使用12字节nonce）
	if _, err := io.ReadFull(rand.Reader, nonceBuf.B); err != nil {
		return nil, err
	}

	// ✅ 使用对象池分配结果缓冲
	bufSize := nonceSize + len(data) + gcm.Overhead()
	ret := zpool.GetBytesBuffer(bufSize)
	defer ret.Release()

	// 将 nonce 追加进去
	ret.B = ret.B[:0] // 重置长度
	ret.B = append(ret.B, nonceBuf.B...)
	// Seal 会把密文和 Tag 追加到 ret.B 后面
	ciphertext := gcm.Seal(ret.B, nonceBuf.B, data, nil)

	// ✅ 深拷贝返回（池化缓冲会被归还）
	result := make([]byte, len(ciphertext))
	copy(result, ciphertext)
	return result, nil
}

// Decrypt 解密AES-GCM（自动验证完整性）
// 输入格式: [Nonce(12字节)][密文+认证标签]
// 如果数据被篡改，会返回错误
func (a *AesGcmEncrypt) Decrypt(data []byte) ([]byte, error) {
	// ✅ 使用缓存的 GCM（避免每次创建）
	gcm := a.gcm
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// 提取nonce和密文
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// 解密并验证认证标签
	// 如果数据被篡改，Open会返回错误
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed: data may be corrupted or tampered")
	}

	return plaintext, nil
}
