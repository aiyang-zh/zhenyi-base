# 安全政策

## 支持的版本

| 版本 | 支持状态 |
|------|----------|
| 最新 main 分支 | ✅ 支持 |
| 最新 release tag | ✅ 支持 |
| 更早版本 | ❌ 不再支持 |

## 漏洞报告

如发现安全漏洞，请**不要**在公开 Issue 中披露，请通过以下方式私下报告：

1. **邮件**：1093993119@qq.com
2. **GitHub 私密安全建议**：在仓库页面点击 "Security" → "Report a vulnerability"

我们会尽快确认并回复，修复后将致谢（在您同意的前提下）。

## 报告内容建议

- 漏洞类型与影响范围
- 复现步骤
- 可能的修复建议（可选）
- 是否允许公开致谢

感谢你帮助 zhenyi-base 更安全！

## CodeQL 与 `zgmtls`（弱哈希类告警）

`zgmtls` 中 **SSL 3.0 / TLS 1.0–1.1** 的 PRF、Finished 等实现按 **RFC 6101 / RFC 2246** 使用 **MD5、SHA-1**，属**协议规定**，不是实现上「改用 SHA-256」即可修复的问题。**国密 `VersionGMSSL`** 仅使用 **SM3**（见 `prfForVersion`、`newFinishedHash` 等 GM 分支）。

若启用 **GitHub CodeQL**（仓库 **Settings → Code security → Code scanning** 中 **Default setup**），规则 **`go/weak-sensitive-data-hashing`** 会对上述 RFC 路径产生**误报**。本仓库在 **`.github/codeql/codeql-config.yml`** 中用 **`query-filters`** **按规则 ID 排除**该查询，以免 PR 与 Security 页被刷屏。

**取舍说明**：在当前 CodeQL CLI（如 2.25）下，**带 `paths` 的 exclude 对弱哈希结果过滤并不可靠**；若仅排除 **`zgmtls/prf.go`**，`prf.go` 中告警仍会出现在显式传入查询的解释结果里。因此采用**对 `go/weak-sensitive-data-hashing` 全仓排除**。本仓库其余 Go 代码不依赖该条规则作为主要防护；其它 CodeQL 安全检查仍照常运行。**勿**将此类告警当作「可独立修补的密码学漏洞」去改协议实现。

**本地预检**：安装 [CodeQL CLI](https://github.com/github/codeql-cli-binaries/releases) 后 **`export CODEQL=…/codeql`**，在仓库根目录 **`make codeql-local`**（`database create` 使用同一 `codeql-config`；**`database analyze` 不要额外传入 .qls**，否则不会应用 query-filters）；详见 **`scripts/README.md`**。
