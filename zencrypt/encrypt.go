package zencrypt

// BaseEncrypt 空操作加密器（不加密），仅用于测试或明确不需要加密的内网场景。
type BaseEncrypt struct{}

func NewBaseEncrypt() *BaseEncrypt {
	return &BaseEncrypt{}
}
func (*BaseEncrypt) Encrypt(data []byte) ([]byte, error) {
	return data, nil
}

func (*BaseEncrypt) Decrypt(data []byte) ([]byte, error) {
	return data, nil
}
