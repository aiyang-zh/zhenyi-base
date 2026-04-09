#!/usr/bin/env bash

set -euo pipefail

# Usage:
#   FUZZTIME=5s bash ./scripts/run_fuzz_all.sh
#   bash ./scripts/run_fuzz_all.sh 30s
FUZZTIME="${FUZZTIME:-${1:-5s}}"

found=0
for pkg in $(go list ./...); do
  targets="$(go test "$pkg" -list '^Fuzz' | awk '/^Fuzz/ {print $1}')"
  if [ -z "$targets" ]; then
    continue
  fi
  found=1
  while IFS= read -r target; do
    [ -z "$target" ] && continue
    echo "==> fuzz ${pkg}#${target} (fuzztime=${FUZZTIME})"
    go test "$pkg" -run='^$' -fuzz="^${target}$" -fuzztime="${FUZZTIME}"
  done <<< "$targets"
done

if [ "$found" -eq 0 ]; then
  echo "No fuzz targets found."
fi
