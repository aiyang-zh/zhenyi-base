#!/usr/bin/env bash
# 信创环境适配度检查：在 Docker 中按多架构（amd64/arm64/loong64）构建并跑测试，输出简要报告。
# 依赖 Docker。龙芯使用社区镜像 ghcr.io/loong64/golang（官方无 linux/loong64）。
# 非龙芯主机跑龙芯镜像时会自动安装 QEMU binfmt（需 Docker 可用且具备权限）。
# 用法：
#   ./scripts/check_xinchuang_compat.sh              # 全架构
#   ./scripts/check_xinchuang_compat.sh linux/amd64  # 仅 x86
#   PLATFORM=linux/arm64 ./scripts/check_xinchuang_compat.sh  # 仅 ARM64
#   make check-xinchuang-amd64 | check-xinchuang-arm64 | check-xinchuang-loong64

set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
SINGLE_PLATFORM="${PLATFORM:-$1}"

GO_IMAGE="${GO_IMAGE:-golang:1.24}"
GO_IMAGE_LOONG64="${GO_IMAGE_LOONG64:-ghcr.io/loong64/golang:1.24}"
BINFMT_IMAGE="${BINFMT_IMAGE:-ghcr.io/loong64/binfmt}"
OUT_DIR="test_results/xc_$(date +%Y-%m-%d)"
mkdir -p "$OUT_DIR"

# 非龙芯主机跑 linux/loong64 前自动安装 QEMU binfmt
ensure_loong64_binfmt() {
	case "$(uname -m)" in
		loongarch64|loong64) return 0 ;;
		*) ;;
	esac
	echo "  安装龙芯 QEMU binfmt（非龙芯主机需此步骤）..."
	docker run --privileged --rm "$BINFMT_IMAGE" --install all 2>/dev/null || true
}

# run_one platform name [image]
# 龙芯无官方镜像，传第 3 参使用社区镜像；输出同时打屏并写入 log，便于看进度
run_one() {
	local platform=$1
	local name=$2
	local img="${3:-$GO_IMAGE}"
	local key="${platform//\//_}"
	local log="$OUT_DIR/${key}.log"
	local build_ok=0
	local test_ok=0
	echo "  [${name}] 构建 (image: ${img}) ..."
	docker run --rm --platform "$platform" -v "$(pwd)":/app -w /app "$img" sh -c "go mod download && go build ./..." 2>&1 | tee "$log"
	[[ ${PIPESTATUS[0]} -eq 0 ]] && build_ok=1
	if [[ $build_ok -eq 1 ]]; then
		echo "  [${name}] 测试 ..."
		docker run --rm --platform "$platform" -v "$(pwd)":/app -w /app "$img" sh -c "go mod download && go test -v \$(go list ./... | grep -vE '/examples/|/ziface/')" 2>&1 | tee -a "$log"
		[[ ${PIPESTATUS[0]} -eq 0 ]] && test_ok=1
	fi
	echo "$build_ok $test_ok" >"$OUT_DIR/${key}.result"
}

if [[ -n "$SINGLE_PLATFORM" ]]; then
	echo "信创单架构检查（Docker）：$SINGLE_PLATFORM"
else
	echo "信创多架构适配检查（Docker）"
fi
echo "结果目录: $OUT_DIR"
echo ""

run_single() {
	case "$1" in
		linux/amd64)  run_one "linux/amd64"   "x86_64 (麒麟/统信/欧拉等)" || true ;;
		linux/arm64)  run_one "linux/arm64"   "ARM64 (鲲鹏/飞腾等)"       || true ;;
		linux/loong64) ensure_loong64_binfmt; run_one "linux/loong64" "LoongArch64 (龙芯)" "$GO_IMAGE_LOONG64" || true ;;
		*) echo "未知架构: $1，支持: linux/amd64, linux/arm64, linux/loong64"; exit 1 ;;
	esac
}

if [[ -n "$SINGLE_PLATFORM" ]]; then
	run_single "$SINGLE_PLATFORM"
else
	# 常见信创相关架构：x86_64(麒麟/统信等)、ARM64(鲲鹏/飞腾)、LoongArch64(龙芯)
	# 龙芯用社区镜像（官方 golang 无 linux/loong64）；某架构失败不中断
	run_one "linux/amd64"   "x86_64 (麒麟/统信/欧拉等)" || true
	run_one "linux/arm64"   "ARM64 (鲲鹏/飞腾等)"       || true
	ensure_loong64_binfmt
	run_one "linux/loong64" "LoongArch64 (龙芯)"       "$GO_IMAGE_LOONG64" || true
fi

echo ""
echo "----------------------------------------"
echo "  架构           | 构建    | 测试"
echo "----------------------------------------"
for platform in linux/amd64 linux/arm64 linux/loong64; do
	key="${platform//\//_}"
	short="${platform#linux/}"
	if [[ -f "$OUT_DIR/${key}.result" ]]; then
		read -r b t <"$OUT_DIR/${key}.result"
		case "$short" in
			amd64)   name="x86_64 (麒麟/统信等)" ;;
			arm64)   name="ARM64 (鲲鹏/飞腾)"   ;;
			loong64) name="LoongArch64 (龙芯)"  ;;
			*)       name="$short" ;;
		esac
		printf "  %-18s | %-6s | %-6s\n" "$name" "$([ "$b" = 1 ] && echo "通过" || echo "失败")" "$([ "$t" = 1 ] && echo "通过" || echo "失败")"
	else
		printf "  %-18s | (未执行或镜像不可用)\n" "${platform#linux/}"
	fi
done
echo "----------------------------------------"
echo "详细日志: $OUT_DIR/*.log"
