.PHONY: all test test-unit bench fmt vet tidy install-hooks test-docker check-xinchuang check-xinchuang-amd64 check-xinchuang-arm64 check-xinchuang-loong64 codeql-local

# 默认跑测试
all: test

# 统一测试入口：fmt → vet → tidy → 功能与覆盖率（基准见 make bench）
test:
	go fmt ./...
	go vet ./...
	go mod tidy
	bash scripts/run_tests.sh

# 仅单元测试（供 pre-commit 等快速检查），排除 examples
test-unit:
	go test -race $$(go list ./... | grep -vE '/examples/|/ziface/') -count=5

# Echo 压测入口
bench:
	bash scripts/run_echo_bench.sh

# Docker 内跑完整测试（复现 CI，含 Linux 专用代码）
test-docker:
	bash scripts/run_tests_docker.sh

# 信创多架构适配检查（Docker：amd64/arm64/loong64）
# 支持单架构：make check-xinchuang-amd64 | check-xinchuang-arm64 | check-xinchuang-loong64
# 或 make check-xinchuang PLATFORM=linux/amd64
check-xinchuang:
	bash scripts/check_xinchuang_compat.sh $(PLATFORM)
check-xinchuang-amd64:
	bash scripts/check_xinchuang_compat.sh linux/amd64
check-xinchuang-arm64:
	bash scripts/check_xinchuang_compat.sh linux/arm64
check-xinchuang-loong64:
	bash scripts/check_xinchuang_compat.sh linux/loong64

# go fmt 全部包
fmt:
	go fmt ./...

# go vet 静态检查
vet:
	go vet ./...

# 清理依赖
tidy:
	go mod tidy

# 启用 Git 钩子（提交前运行 make test：fmt / vet / tidy / 测试）
install-hooks:
	git config core.hooksPath .githooks
	@echo "已启用 .githooks，提交前将运行 make test"

# 本地 CodeQL：需已安装 CLI 并 export CODEQL=…/codeql；默认只跑 go/weak-sensitive-data-hashing
# 跑完整 go-code-scanning：CODEQL_LOCAL_SUITE=1 make codeql-local
codeql-local:
	bash scripts/run_codeql_local.sh

