// Package zencrypt Package encrypt 提供SM4国密对称加密实现。
//
// SM4-GCM加密示例（推荐）:
//
//	cipher := encrypt.NewSM4GcmEncrypt("your-secret-key")
//
//	encrypted, err := cipher.Encrypt([]byte("secret data"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	decrypted, err := cipher.Decrypt(encrypted)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// SM4-CBC加密示例:
//
//	cipher := encrypt.NewSM4Encrypt("your-secret-key")
//
//	encrypted, err := cipher.Encrypt([]byte("secret data"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	decrypted, err := cipher.Decrypt(encrypted)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// 注意事项:
//   - 密钥通过SM3派生为16字节（SM4密钥长度固定128位）
//   - CBC模式：IV随机生成并附加在密文前16字节，使用PKCS7填充
//   - GCM模式：Nonce随机生成并附加在密文前12字节，自带认证标签，无需填充
//   - 推荐使用GCM模式（更安全，更快，防篡改）
//   - 符合 GM/T 0002-2012 标准
package zencrypt

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/tjfoc/gmsm/sm4"
)

// SM4Encrypt SM4-CBC加密器
type SM4Encrypt struct {
	SecretKey  string
	derivedKey []byte
	block      cipher.Block
}

// NewSM4Encrypt 创建SM4-CBC加密器
func NewSM4Encrypt(key string) *SM4Encrypt {
	derivedKey := sm4DeriveKey(key)
	block, err := sm4.NewCipher(derivedKey)
	if err != nil {
		panic("sm4: invalid key: " + err.Error())
	}
	return &SM4Encrypt{
		SecretKey:  key,
		derivedKey: derivedKey,
		block:      block,
	}
}

// Encrypt 使用SM4-CBC加密
// 返回格式: [IV(16字节)][密文]
func (s *SM4Encrypt) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	block := s.block
	blockSize := block.BlockSize()

	paddedData := pkcs7Pad(data, blockSize)

	bufSize := blockSize + len(paddedData)
	ciphertextBuf := zpool.GetBytesBuffer(bufSize)
	defer ciphertextBuf.Release()

	iv := ciphertextBuf.B[:blockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertextBuf.B[blockSize:], paddedData)

	result := make([]byte, bufSize)
	copy(result, ciphertextBuf.B)
	return result, nil
}

// Decrypt 解密SM4-CBC
// 输入格式: [IV(16字节)][密文]
func (s *SM4Encrypt) Decrypt(data []byte) ([]byte, error) {
	block := s.block
	blockSize := block.BlockSize()
	if len(data) < blockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := data[:blockSize]
	ciphertext := data[blockSize:]

	if len(ciphertext)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	plaintextBuf := zpool.GetBytesBuffer(len(ciphertext))
	defer plaintextBuf.Release()

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintextBuf.B, ciphertext)

	unpadded, err := pkcs7Unpad(plaintextBuf.B)
	if err != nil {
		return nil, err
	}

	result := make([]byte, len(unpadded))
	copy(result, unpadded)
	return result, nil
}

// ====================================
// SM4-GCM 模式（推荐使用）
// ====================================

// SM4GcmEncrypt SM4-GCM加密器（认证加密，推荐）
type SM4GcmEncrypt struct {
	SecretKey  string
	derivedKey []byte
	block      cipher.Block
	gcm        cipher.AEAD
}

// NewSM4GcmEncrypt 创建SM4-GCM加密器
func NewSM4GcmEncrypt(key string) *SM4GcmEncrypt {
	derivedKey := sm4DeriveKey(key)

	block, err := sm4.NewCipher(derivedKey)
	if err != nil {
		panic("sm4: invalid key: " + err.Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic("sm4: failed to create GCM: " + err.Error())
	}

	return &SM4GcmEncrypt{
		SecretKey:  key,
		derivedKey: derivedKey,
		block:      block,
		gcm:        gcm,
	}
}

// Encrypt 使用SM4-GCM加密（认证加密）
// 返回格式: [Nonce(12字节)][密文+认证标签]
func (s *SM4GcmEncrypt) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	gcm := s.gcm
	nonceSize := gcm.NonceSize()

	nonceBuf := zpool.GetBytesBuffer(nonceSize)
	defer nonceBuf.Release()

	if _, err := io.ReadFull(rand.Reader, nonceBuf.B); err != nil {
		return nil, err
	}

	bufSize := nonceSize + len(data) + gcm.Overhead()
	ret := zpool.GetBytesBuffer(bufSize)
	defer ret.Release()

	ret.B = ret.B[:0]
	ret.B = append(ret.B, nonceBuf.B...)
	ciphertext := gcm.Seal(ret.B, nonceBuf.B, data, nil)

	result := make([]byte, len(ciphertext))
	copy(result, ciphertext)
	return result, nil
}

// Decrypt 解密SM4-GCM（自动验证完整性）
// 输入格式: [Nonce(12字节)][密文+认证标签]
func (s *SM4GcmEncrypt) Decrypt(data []byte) ([]byte, error) {
	gcm := s.gcm
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("sm4-gcm: decryption failed: data may be corrupted or tampered")
	}

	return plaintext, nil
}

// sm4DeriveKey 通过SM3从任意长度密钥派生出SM4所需的16字节密钥
func sm4DeriveKey(key string) []byte {
	hash := SM3(key)
	return hash[:16]
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	unPadding := int(data[length-1])
	if unPadding > length || unPadding == 0 {
		return nil, errors.New("invalid padding")
	}
	for _, b := range data[length-unPadding:] {
		if int(b) != unPadding {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return data[:length-unPadding], nil
}
