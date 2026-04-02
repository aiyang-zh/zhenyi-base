# 贡献指南

感谢你对 zhenyi-base 的关注！

**维护者**：1093993119@qq.com

## 如何参与

1. **提交 Issue**：Bug 报告、功能建议、使用问题
2. **提交 PR**：修复、新功能、文档改进

## 提交流程

1. Fork 本仓库
2. 创建分支：`git checkout -b feat/xxx` 或 `fix/xxx`
3. 提交代码：`git commit -m "feat(xxx): 描述"`
4. 推送分支：`git push origin feat/xxx`
5. 在 GitHub 创建 Pull Request

## 代码规范

- 遵循 Go 官方 [Effective Go](https://go.dev/doc/effective_go)
- 新包需补充单元测试
- 提交前运行 `make test`（含 `go fmt` / `go vet` / `go mod tidy` 与功能与覆盖率测试）或更快的 `make test-unit` 确保通过（测试范围不包含 `examples/`、`ziface/`）；已执行 `make install-hooks` 时，提交前将跑完整 `make test`
- 可选：在 **`zserialize`、`znet`、`zqueue`** 等含 **`fuzz_test.go`** 的包上执行 **`go test -fuzz=…`** 做长时模糊测试（用法见 **`docs/API.md`**）
- **`go vet`** 为 `make test` 前置步骤，请勿跳过；**`zgmtls`**（国密 GM-TLS）等与协议/静态检查相关的说明见 **`CHANGELOG.md`**（例如 **`[1.1.0]`** 中 **`eccKeyAgreementGM`** 长度字段解析、`gm_key_agreement_test.go` 等）

## 提交信息格式

```
<type>(<scope>): <subject>

<body>
```

- type: `feat` / `fix` / `docs` / `refactor` / `test` / `chore`
- scope: 包名，如 `zqueue`、`znet`
