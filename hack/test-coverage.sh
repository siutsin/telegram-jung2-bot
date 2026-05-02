#!/usr/bin/env bash
set -euo pipefail

coverage_file="${TMPDIR:-/tmp}/telegram-jung2-bot-coverage.out"
packages=()

while IFS= read -r dir; do
  packages+=("$dir")
done < <(
  find . \
    -path './buck-out' -prune -o \
    -path './l[e]gacy' -prune -o \
    -path './node_modules' -prune -o \
    -path './vendor' -prune -o \
    -name '*.go' -print0 |
    while IFS= read -r -d '' file; do
      dirname "$file"
    done |
    sort -u
)

go test "${packages[@]}" -coverprofile="$coverage_file"
go tool cover -func="$coverage_file"

coverage=$(go tool cover -func="$coverage_file" | awk '/^total:/ {print $3}')
if [[ "$coverage" != "100.0%" ]]; then
  echo "expected 100.0% test coverage, got $coverage" >&2
  exit 1
fi
