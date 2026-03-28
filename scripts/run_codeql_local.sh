#!/usr/bin/env bash
# 本地跑 CodeQL（Go），与仓库根目录 `.github/codeql/codeql-config.yml`（含 query-filters）对齐，便于对照 GitHub Code scanning。
# 依赖：安装 CodeQL CLI，并 export CODEQL=…/codeql（或包含 codeql 可执行文件的目录）。
# 首次会下载查询包，需联网：codeql pack download codeql/go-queries
#
# 注意：database analyze 时请勿显式传入 .qls / 单条 .ql，否则 query-filters 不会作用于解释结果（弱哈希等仍会出现在 CSV）。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -z "${CODEQL:-}" ]]; then
	echo "未设置 CODEQL。请先安装 CodeQL CLI 后执行，例如：" >&2
	echo "  export CODEQL=\"\$HOME/codeql/codeql\"" >&2
	echo "发布包：https://github.com/github/codeql-cli-binaries/releases" >&2
	exit 1
fi

CODEQL_BIN="$CODEQL"
if [[ -d "$CODEQL" ]]; then
	CODEQL_BIN="${CODEQL%/}/codeql"
fi
if [[ ! -x "$CODEQL_BIN" ]]; then
	echo "CODEQL 指向的路径不可执行: $CODEQL_BIN" >&2
	exit 1
fi

CONFIG="${ROOT}/.github/codeql/codeql-config.yml"
if [[ ! -f "$CONFIG" ]]; then
	echo "缺少 CodeQL 配置文件: $CONFIG" >&2
	exit 1
fi

mkdir -p "${ROOT}/.codeql"
DB="${ROOT}/.codeql/go-db"
rm -rf "$DB"

echo "==> codeql database create (--codescanning-config + go build ./...)"
"$CODEQL_BIN" database create "$DB" --language=go --source-root="$ROOT" \
	--codescanning-config="$CONFIG" \
	--command="go build -o /dev/null ./..."

echo "==> codeql pack download codeql/go-queries（首次或缺包时下载）"
"$CODEQL_BIN" pack download codeql/go-queries

# CodeQL 2.25+ 不再支持 --format=text；不设查询列表，使用 create 时写入 DB 的套件 + query-filters
FMT="${CODEQL_LOCAL_FORMAT:-csv}"
OUT="${ROOT}/.codeql/analyze-out.${FMT}"
echo "==> codeql database analyze（默认套件 + 应用 codeql-config 中的 query-filters；format=$FMT）"
"$CODEQL_BIN" database analyze "$DB" --format="$FMT" --output="$OUT"

echo "==> 结果文件: $OUT"
if [[ "$FMT" == "csv" ]] && [[ -f "$OUT" ]]; then
	if [[ ! -s "$OUT" ]]; then
		echo "（CSV 无数据行：当前套件下无告警输出）"
	else
		head -50 "$OUT"
		[[ "$(wc -l <"$OUT" | tr -d ' ')" -gt 50 ]] && echo "...（行数多，请打开 $OUT 查看全文）"
	fi
fi

echo "==> 完成。数据库目录: $DB（可删除整个 .codeql/ 以省空间）"
