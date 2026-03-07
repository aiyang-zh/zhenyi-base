package zencrypt

import "encoding/base64"

func EncodeToString(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func DecodeString(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(key)
}
