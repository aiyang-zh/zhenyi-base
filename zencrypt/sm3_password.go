// Package zencrypt Package encrypt 提供基于 SM3 的密码哈希实现（信创默认）。
//
// 使用 SM3-HMAC + 随机 salt + 多轮迭代，安全强度等同于 PBKDF2-SHA256。
// 符合 GM/T 0004-2012 标准。
//
// 示例:
//
//	hasher := encrypt.NewSM3Password()
//
//	hash, _ := hasher.Encrypt("my-password")
//	ok := hasher.CompareHashAndPassword("my-password", hash) // true
//	ok = hasher.CompareHashAndPassword("wrong", hash)        // false
//
// 与 Argon2/Bcrypt 接口一致，可直接替换。
package zencrypt

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"

	"github.com/tjfoc/gmsm/sm3"
)

const (
	sm3SaltSize   = 16
	sm3Iterations = 10000
	sm3KeyLen     = 32
)

// SM3Password 基于 SM3 的密码哈希器（信创推荐）。
type SM3Password struct{}

// NewSM3Password 创建 SM3 密码哈希器。
func NewSM3Password() *SM3Password {
	return &SM3Password{}
}

// Encrypt 使用随机 salt + SM3 迭代生成密码哈希，返回 base64(salt + hash)。
func (s *SM3Password) Encrypt(password string) (string, error) {
	salt := make([]byte, sm3SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := sm3DeriveKey([]byte(password), salt, sm3Iterations, sm3KeyLen)
	result := make([]byte, sm3SaltSize+sm3KeyLen)
	copy(result, salt)
	copy(result[sm3SaltSize:], hash)
	return base64.RawStdEncoding.EncodeToString(result), nil
}

// CompareHashAndPassword 使用常量时间比较验证密码，防止时序攻击。
func (s *SM3Password) CompareHashAndPassword(password, hashPassword string) bool {
	decoded, err := base64.RawStdEncoding.DecodeString(hashPassword)
	if err != nil || len(decoded) < sm3SaltSize+sm3KeyLen {
		return false
	}
	salt := decoded[:sm3SaltSize]
	storedHash := decoded[sm3SaltSize : sm3SaltSize+sm3KeyLen]
	hash := sm3DeriveKey([]byte(password), salt, sm3Iterations, sm3KeyLen)
	return subtle.ConstantTimeCompare(hash, storedHash) == 1
}

// sm3DeriveKey PBKDF2 风格的 SM3 密钥派生。
func sm3DeriveKey(password, salt []byte, iterations, keyLen int) []byte {
	// PBKDF2-SM3: HMAC-SM3 多轮迭代
	dk := make([]byte, 0, keyLen)
	block := 1
	for len(dk) < keyLen {
		dk = append(dk, sm3PRF(password, salt, iterations, block)...)
		block++
	}
	return dk[:keyLen]
}

// sm3PRF PBKDF2 的 PRF 函数 (F)，使用 SM3 做 HMAC。
func sm3PRF(password, salt []byte, iterations, block int) []byte {
	// U1 = HMAC-SM3(password, salt || INT_32_BE(block))
	saltBlock := make([]byte, len(salt)+4)
	copy(saltBlock, salt)
	saltBlock[len(salt)] = byte(block >> 24)
	saltBlock[len(salt)+1] = byte(block >> 16)
	saltBlock[len(salt)+2] = byte(block >> 8)
	saltBlock[len(salt)+3] = byte(block)
	u := sm3HMAC(password, saltBlock)
	result := make([]byte, len(u))
	copy(result, u)

	// U2..Un = HMAC-SM3(password, U_{i-1}), result ^= Ui
	for i := 1; i < iterations; i++ {
		u = sm3HMAC(password, u)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}

// sm3HMAC 计算 HMAC-SM3。
func sm3HMAC(key, data []byte) []byte {
	const blockSize = 64
	if len(key) > blockSize {
		h := sm3.New()
		h.Write(key)
		key = h.Sum(nil)
	}

	iPad := make([]byte, blockSize)
	oPad := make([]byte, blockSize)
	copy(iPad, key)
	copy(oPad, key)
	for i := 0; i < blockSize; i++ {
		iPad[i] ^= 0x36
		oPad[i] ^= 0x5c
	}

	inner := sm3.New()
	inner.Write(iPad)
	inner.Write(data)
	innerSum := inner.Sum(nil)

	outer := sm3.New()
	outer.Write(oPad)
	outer.Write(innerSum)
	return outer.Sum(nil)
}
