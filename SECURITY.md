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

若启用 **GitHub CodeQL**（在仓库 **Settings → Code security → Code scanning** 中开启 **Default setup** 即可，本仓库**不**再附带单独的 CodeQL Actions 工作流），规则 **`go/weak-sensitive-data-hashing`** 可能对此类代码产生**误报**。本仓库**不在**配置中全局关闭该规则，以便其它源文件仍受检查；仅在 **`zgmtls/prf.go`** 中对 RFC 规定的 PRF/Finished 路径使用 **`// codeql[go/weak-sensitive-data-hashing]`** 抑制（见 CodeQL Go 的 `AlertSuppression` 实现）。**必须**是**单独一行**的 `// codeql[...]`（行首除空白外只能是注释，**不能**写在语句同一行末尾），且**只抑制紧邻的下一整行**；path-problem 对每个 sink 通常各需一条上一行注释。**勿**将此类告警当作「可独立修补的密码学漏洞」而修改协议实现。

**本地预检**：安装 [CodeQL CLI](https://github.com/github/codeql-cli-binaries/releases) 后执行 **`export CODEQL=…/codeql`**，在仓库根目录 **`make codeql-local`**（默认只跑 **`go/weak-sensitive-data-hashing`**），无需 push 即可看是否仍报；详见 **`scripts/README.md`**。
