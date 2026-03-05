#!/bin/bash
# 极简版测试脚本：运行功能、基准、覆盖率测试，结果归档到日期目录

set -e

DATE=$(date +%Y-%m-%d)
OUTPUT_DIR="test_results/$DATE"
mkdir -p "$OUTPUT_DIR"

echo "📁 结果保存到：$OUTPUT_DIR"

# 1. 功能测试
echo "🔍 运行功能测试..."
go test ./... > "$OUTPUT_DIR/unit_tests.log" 2>&1
echo "✅ 功能测试完成"

# 2. 基准测试
echo "⏱️ 运行基准测试（可能稍慢）..."
go test -bench=. -benchmem ./... > "$OUTPUT_DIR/bench_tests.log" 2>&1
echo "✅ 基准测试完成"

# 3. 覆盖率测试
echo "📊 运行覆盖率测试..."
go test -coverprofile="$OUTPUT_DIR/coverage.out" ./...
if [ -f "$OUTPUT_DIR/coverage.out" ]; then
    go tool cover -func="$OUTPUT_DIR/coverage.out" | tail -1 > "$OUTPUT_DIR/coverage_summary.txt"
    echo "覆盖率：$(cat $OUTPUT_DIR/coverage_summary.txt)"
fi

echo "🎉 所有测试完成！结果目录：$OUTPUT_DIR"