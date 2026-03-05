#!/bin/bash
# 增强版测试脚本：运行功能、基准、覆盖率测试，归档并自动打开报告

set -e

DATE=$(date +%Y-%m-%d)
OUTPUT_DIR="test_results/$DATE"
mkdir -p "$OUTPUT_DIR"

echo "📁 结果将保存到：$OUTPUT_DIR"

# 记录环境信息
go version > "$OUTPUT_DIR/environment.txt"
echo "OS: $(uname -a)" >> "$OUTPUT_DIR/environment.txt"

START_TIME=$(date +%s)

# 1. 功能测试
echo "🔍 运行功能测试..."
go test ./... > "$OUTPUT_DIR/unit_tests.log" 2>&1
echo "✅ 功能测试完成"

# 2. 基准测试
echo "⏱️ 运行基准测试（可能会花几分钟）..."
go test -bench=. -benchmem ./... > "$OUTPUT_DIR/bench_tests.log" 2>&1

# 提取关键基准数据（每行包含 Benchmark 和 ns/op）
grep "^Benchmark" "$OUTPUT_DIR/bench_tests.log" | awk '{print $1, $3}' > "$OUTPUT_DIR/bench_summary.txt"
echo "✅ 基准测试完成，摘要已提取：$OUTPUT_DIR/bench_summary.txt"

# 3. 覆盖率测试
echo "📊 运行覆盖率测试..."
go test -coverprofile="$OUTPUT_DIR/coverage.out" ./...
if [ -f "$OUTPUT_DIR/coverage.out" ]; then
    go tool cover -html="$OUTPUT_DIR/coverage.out" -o "$OUTPUT_DIR/coverage.html"
    echo "✅ 覆盖率报告生成：$OUTPUT_DIR/coverage.html"
    # 总覆盖率摘要
    go tool cover -func="$OUTPUT_DIR/coverage.out" | tail -1 > "$OUTPUT_DIR/coverage_summary.txt"
    echo "覆盖率：$(cat $OUTPUT_DIR/coverage_summary.txt)"

    # 可选：自动打开 HTML 报告（macOS）
    open "$OUTPUT_DIR/coverage.html"
else
    echo "⚠️ 覆盖率测试未生成 coverage.out"
fi

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
echo "⏱️ 总耗时：$DURATION 秒" | tee -a "$OUTPUT_DIR/execution.log"

echo "🎉 所有测试完成！结果目录：$OUTPUT_DIR"