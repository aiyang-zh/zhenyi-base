#!/bin/bash
# 极简版测试脚本：运行功能、覆盖率测试，结果归档到日期目录
# 应由 Makefile 在仓库根目录调用：make test（基准测试请用 make bench 或 go test -bench）

set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DATE=$(date +%Y-%m-%d)
OUTPUT_DIR="test_results/$DATE"
mkdir -p "$OUTPUT_DIR"
# 排除 examples，仅测库代码
PACKAGES="$(go list ./... | grep -vE '/examples/|/ziface/')"

TOTAL_START=$SECONDS

echo "📁 结果保存到：$OUTPUT_DIR"

# 1. 功能测试
echo "🔍 运行功能测试..."
START=$SECONDS
go test -race $PACKAGES -v 2>&1 | tee "$OUTPUT_DIR/unit_tests.log"
[[ ${PIPESTATUS[0]} -ne 0 ]] && exit ${PIPESTATUS[0]}
echo "✅ 功能测试完成（耗时 $((SECONDS - START)) 秒）"

# 2. 覆盖率测试
echo "📊 运行覆盖率测试..."
START=$SECONDS
go test -race -cover $PACKAGES > "$OUTPUT_DIR/cover.log" 2>&1
echo "✅ 覆盖率测试完成（耗时 $((SECONDS - START)) 秒）"

echo "🎉 所有测试完成！总耗时 $((SECONDS - TOTAL_START)) 秒，结果目录：$OUTPUT_DIR"
