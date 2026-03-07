# 贡献指南

感谢你对 zhenyi-base 的关注！

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
- 提交前运行 `./run_tests.sh` 确保通过

## 提交信息格式

```
<type>(<scope>): <subject>

<body>
```

- type: `feat` / `fix` / `docs` / `refactor` / `test` / `chore`
- scope: 包名，如 `zqueue`、`znet`
