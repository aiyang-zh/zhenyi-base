// Package encrypt 提供SM3国密哈希实现。
//
// SM3是中国国家密码管理局发布的密码杂凑算法标准（GM/T 0004-2012），
// 输出256位（32字节）摘要，安全强度等同于SHA-256。
//
// 示例:
//
//	hash := encrypt.SM3("my-data")
//	// hash 为32字节摘要
//
//	hashHex := encrypt.SM3Hex("my-data")
//	// hashHex 为64字符十六进制字符串
//
//	hashBytes := encrypt.SM3Bytes([]byte("my-data"))
//	// 直接对字节切片计算哈希
package zencrypt

import (
	"encoding/hex"

	"github.com/emmansun/gmsm/sm3"
)

// SM3 计算字符串的SM3哈希，返回32字节摘要。
func SM3(data string) []byte {
	h := sm3.New()
	h.Write([]byte(data))
	return h.Sum(nil)
}

// SM3Bytes 计算字节切片的SM3哈希，返回32字节摘要。
func SM3Bytes(data []byte) []byte {
	h := sm3.New()
	h.Write(data)
	return h.Sum(nil)
}

// SM3Hex 计算字符串的SM3哈希，返回十六进制编码字符串。
func SM3Hex(data string) string {
	return hex.EncodeToString(SM3(data))
}
