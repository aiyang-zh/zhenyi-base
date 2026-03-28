#!/usr/bin/env bash
# 在 Linux 容器内跑完整测试（功能 + 覆盖率），用于 Mac/Windows 本地复现 CI（含 zreactor 等 linux 专用代码）。
# 依赖 Docker，镜像带 make（golang:1.24 基于 Debian）。用法：./scripts/run_tests_docker.sh

set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
# make test = scripts/run_tests.sh：功能测试、覆盖率测试，结果写入 test_results/$(date +%Y-%m-%d)
docker run --rm -v "$(pwd)":/app -w /app golang:1.24 sh -c "go mod download && make test"
