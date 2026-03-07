package zencrypt

import (
	"crypto/sha256"
)

// Sha256 计算字符串的SHA-256哈希。
//
// SHA-256是安全的，适合用于密码学目的。
//
// 返回32字节（256位）哈希。
//
// 示例：
//
//	hash := enctypt.Sha256("my-secret-key")
//	// 可用于加密密钥、HMAC等
func Sha256(key string) []byte {
	h := sha256.New()
	h.Write([]byte(key))
	return h.Sum(nil)
}
