# 脚本说明

脚本均由 **Makefile** 在仓库根目录调用，也可直接执行（会自动 `cd` 到仓库根）。

**测试范围**：`make test`、`make test-unit` 及信创检查均**不包含** `examples/`、`ziface/`，仅测库代码。

**`make test`** 会先执行 `go fmt ./...`、`go vet ./...`、`go mod tidy`，再调用 `run_tests.sh`。其中 **`go vet`** 会拦截易错写法（如不当位移）；**`zgmtls`** 等国密相关修复与单测说明见仓库根目录 **`CHANGELOG.md`**（**`[1.1.0]`** 起）。

| 脚本 | Make 目标 | 说明 |
|------|-----------|------|
| `run_tests.sh` | `make test` | 功能测试 + 覆盖率，结果写入 `test_results/` |
| `run_echo_bench.sh` | `make bench` | Echo 压测（tcp/ws/kcp），可传参场景与协议 |
| `run_tests_docker.sh` | `make test-docker` | Docker 内跑完整测试，复现 CI（含 Linux 专用代码） |
| `check_xinchuang_compat.sh` | `make check-xinchuang` | 信创多架构适配检查（amd64/arm64/loong64） |
| （同上） | `make check-xinchuang-amd64` / `-arm64` / `-loong64` | 仅跑指定架构；或 `make check-xinchuang PLATFORM=linux/amd64` |