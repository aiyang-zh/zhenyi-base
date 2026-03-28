#!/usr/bin/env bash
# 本地跑 CodeQL（Go），默认只跑与 GitHub 相同的 weak-sensitive-data-hashing，避免反复 push 试扫描。
# 依赖：安装 CodeQL CLI，并 export CODEQL=…/codeql（或包含 codeql 可执行文件的目录）。
# 首次会下载查询包，需联网：codeql pack download codeql/go-queries
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

mkdir -p "${ROOT}/.codeql"
DB="${ROOT}/.codeql/go-db"
rm -rf "$DB"

echo "==> codeql database create (go build ./...)"
"$CODEQL_BIN" database create "$DB" --language=go --source-root="$ROOT" \
	--command="go build -o /dev/null ./..."

echo "==> codeql pack download codeql/go-queries（首次或缺包时下载）"
"$CODEQL_BIN" pack download codeql/go-queries

# 默认仅跑 weak-hash 与 GitHub Code scanning 对齐；设 CODEQL_LOCAL_SUITE=1 可跑完整 go-code-scanning 套件（较慢）
# CodeQL 2.25+ 不再支持 --format=text，改用 csv 打印到终端（或设 CODEQL_LOCAL_FORMAT=sarif-latest 写 SARIF）
FMT="${CODEQL_LOCAL_FORMAT:-csv}"
OUT="${ROOT}/.codeql/analyze-out.${FMT}"
if [[ "${CODEQL_LOCAL_SUITE:-}" == "1" ]]; then
	echo "==> analyze: codeql/go-queries codeql-suites/go-code-scanning.qls (format=$FMT)"
	"$CODEQL_BIN" database analyze "$DB" --format="$FMT" --output="$OUT" \
		codeql/go-queries:codeql-suites/go-code-scanning.qls
else
	# 必须同时跑 AlertSuppression.ql，interpret 才会应用 // codeql[rule-id]；只跑单条 Weak*.ql 时抑制不会生效（CSV/SARIF 仍列告警）。
	echo "==> analyze: AlertSuppression.ql + go/weak-sensitive-data-hashing (format=$FMT)"
	"$CODEQL_BIN" database analyze "$DB" --format="$FMT" --output="$OUT" \
		codeql/go-queries:AlertSuppression.ql \
		codeql/go-queries:Security/CWE-327/WeakSensitiveDataHashing.ql
fi
echo "==> 结果文件: $OUT"
if [[ "$FMT" == "csv" ]] && [[ -f "$OUT" ]]; then
	if [[ ! -s "$OUT" ]]; then
		echo "（无 CSV 行：未命中告警，或已全部被源码 // codeql[...] 抑制）"
	else
		head -50 "$OUT"
		[[ "$(wc -l <"$OUT" | tr -d ' ')" -gt 50 ]] && echo "...（行数多，请打开 $OUT 查看全文）"
	fi
fi

echo "==> 完成。数据库目录: $DB（可删除整个 .codeql/ 以省空间）"
