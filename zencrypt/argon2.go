package zencrypt

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"

	"golang.org/x/crypto/argon2"
)

const argon2SaltSize = 16

type Argon2 struct{}

func NewArgon2() *Argon2 {
	return &Argon2{}
}

// Encrypt 使用随机 salt 生成 Argon2id hash，返回 base64(salt+hash)
func (b *Argon2) Encrypt(password string) (string, error) {
	salt := make([]byte, argon2SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	result := make([]byte, argon2SaltSize+len(hash))
	copy(result, salt)
	copy(result[argon2SaltSize:], hash)
	return base64.RawStdEncoding.EncodeToString(result), nil
}

// CompareHashAndPassword 使用常量时间比较，防止时序攻击
func (b *Argon2) CompareHashAndPassword(password, hashPassword string) bool {
	decoded, err := base64.RawStdEncoding.DecodeString(hashPassword)
	if err != nil || len(decoded) <= argon2SaltSize {
		return false
	}
	salt := decoded[:argon2SaltSize]
	storedHash := decoded[argon2SaltSize:]
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(hash, storedHash) == 1
}
