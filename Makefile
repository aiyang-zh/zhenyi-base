.PHONY: all test test-unit bench fmt vet tidy install-hooks

# 默认跑测试
all: test

# 统一测试入口：功能、基准、覆盖率
test:
	bash run_tests.sh

# 仅单元测试（供 pre-commit 等快速检查）
test-unit:
	go test ./...

# Echo 压测入口
bench:
	bash run_echo_bench.sh

# go fmt 全部包
fmt:
	go fmt ./...

# go vet 静态检查
vet:
	go vet ./...

# 清理依赖
tidy:
	go mod tidy

# 启用 Git 钩子（提交前运行 make test-unit）
install-hooks:
	git config core.hooksPath .githooks
	@echo "已启用 .githooks，提交前将运行 make test-unit"

