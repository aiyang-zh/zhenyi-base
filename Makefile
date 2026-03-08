.PHONY: all test bench fmt vet tidy

# 默认跑测试
all: test

# 统一测试入口：功能、基准、覆盖率
test:
	bash run_tests.sh

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

