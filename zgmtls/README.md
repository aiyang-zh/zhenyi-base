# zgmtls — 国密 GM-TLS（`package gmtls`）

本目录实现 **GM/T 0024** 风格的 TLS 记录与握手（`VersionGMSSL`），与 **SM2 / SM3 / SM4** 套件配合使用。改编自 [tjfoc/gmsm](https://github.com/tjfoc/gmsm) 的 GM-TLS 相关代码，许可见 [NOTICE](NOTICE)。

上层业务请优先通过 **`ziface` / `znet`** 的 **`GMTLSConfig`**、**`NewGMTLSConfig*`**、**`DialTLS`** 等封装使用；本说明面向直接使用 **`gmtls.Config`** / **`gmtls.Dial`** / **`gmtls.Listen`** 的场景。

---

## 密码套件与默认协商

| 套件常量 | 含义 |
|----------|------|
| **`GMTLS_ECDHE_SM2_WITH_SM4_SM3`** | **ECDHE**：服务端临时 SM2 密钥 + **ServerKeyExchange**（SM2 签名）；客户端与服务器做 **ECDH** 派生主密钥。 |
| **`GMTLS_SM2_WITH_SM4_SM3`** | **ECC / 静态**：无 ECDHE；客户端密钥用 **加密证书公钥** 做 SM2 加密封装预主密钥（双证书场景）。 |

未设置 **`Config.CipherSuites`** 时，**`getCipherSuites`** 默认顺序为：

1. **`ECDHE_SM2`**（优先）
2. **`SM2`（ECC）**

若需**只使用**静态 ECC 套件（与旧行为一致），请显式设置：

```go
&gmtls.Config{
    GMSupport: gmtls.NewGMSupport(),
    CipherSuites: []uint16{gmtls.GMTLS_SM2_WITH_SM4_SM3},
    // ...
}
```

---

## 服务端

- **双证书**（默认 `!single_cert` 构建）：`Certificates` 需 **至少两条** `gmtls.Certificate`（签名证书、加密证书）；与 **`ecdhe` / `ecc`** 密钥协商一致。
- **签名证书**：须为 **SM2**，且能 **`crypto.Signer.Sign`**；**ECDHE** 的 **ServerKeyExchange** 仅支持 **SM2 签名**（`signatureSM2`）。
- **`GMSupport`**：使用 **`gmtls.NewGMSupport()`**，`Config` 中 **`GMSupport` 非 nil** 时走 GM 握手与套件表。

---

## 客户端

- 当 ClientHello 中包含 **`GMTLS_ECDHE_SM2_WITH_SM4_SM3`** 时，实现会自动附带 **`supported_curves`** 扩展，曲线为 **`CurveP256`（TLS 命名曲线 23）**，与 **`sm2.P256()`** 参数一致。
- 需设置 **`ServerName`** 或 **`InsecureSkipVerify`**（与其它 TLS 客户端相同）。

---

## ECDHE 实现要点（与 CHANGELOG 一致）

- **ServerKeyExchange** 签名原文的 SM3：在 **`VersionGMSSL`** 且 **SM2 签名** 时，对 **`client_random || server_random || ECDH 参数`** 做 **SM3**，与 **`sm2.VerifyASN1`** 所需的预计算 **hash** 一致（见 **`key_agreement.hashForServerKeyExchange`**）。
- **曲线**：ECDHE 在 **`sm2.P256()`** 上完成；线路上 **named curve** 使用 **`CurveP256`（0x0017）**。

---

## 互通性

不同厂商 GM-TLS 栈在 **曲线编号、扩展、签名绑定** 上可能有差异。若与第三方设备握手失败，可尝试：

- 显式 **`CipherSuites`** 仅保留 **`GMTLS_SM2_WITH_SM4_SM3`**；
- 核对对端是否要求特定 **supported_groups / signature_algorithms**。

---

## 测试与静态检查

仓库 **`make test`** 会执行 **`go vet`**（含位移等检查）、**`zgmtls`** 单测等。详细说明见根目录 [CONTRIBUTING.md](../CONTRIBUTING.md)、[CHANGELOG.md](../CHANGELOG.md)（**`[1.1.0]`** 起）。
