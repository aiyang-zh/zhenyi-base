package ziface

// IEncrypt 加解密接口。
//
// 封装对称/非对称加密实现，供网络层在收发数据时透明使用。
type IEncrypt interface {
	// Encrypt 对明文数据进行加密，返回密文。
	Encrypt([]byte) ([]byte, error)

	// Decrypt 对密文数据进行解密，返回明文。
	Decrypt([]byte) ([]byte, error)
}
