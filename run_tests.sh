#!/bin/bash
# 极简版测试脚本：运行功能、基准、覆盖率测试，结果归档到日期目录

set -e

DATE=$(date +%Y-%m-%d)
OUTPUT_DIR="test_results/$DATE"
mkdir -p "$OUTPUT_DIR"

echo "📁 结果保存到：$OUTPUT_DIR"

# 1. 功能测试
echo "🔍 运行功能测试..."
go test ./... -v > "$OUTPUT_DIR/unit_tests.log" 2>&1
echo "✅ 功能测试完成"

# 2. 基准测试
echo "⏱️ 运行基准测试..."
go test -bench=. -benchmem ./... > "$OUTPUT_DIR/bench_tests.log" 2>&1
echo "✅ 基准测试完成"

# 3. 覆盖率测试
echo "📊 运行覆盖率测试..."
go test -cover ./... > "$OUTPUT_DIR/cover.log" 2>&1

echo "🎉 所有测试完成！结果目录：$OUTPUT_DIR"