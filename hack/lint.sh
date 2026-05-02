#!/usr/bin/env bash
set -euo pipefail

mode="${1:-check}"
if [[ "${mode}" != "check" && "${mode}" != "fix" ]]; then
  echo "usage: $0 [check|fix]" >&2
  exit 2
fi

go_files=()
while IFS= read -r go_file; do
  go_files+=("${go_file}")
done < <(find . \
  -type f \
  -name '*.go' \
  -not -path './buck-out/*' \
  -not -path './l[e]gacy/*' \
  -not -path './node_modules/*' \
  -not -path './vendor/*' \
  | sort)

if ((${#go_files[@]} > 0)); then
  if [[ "${mode}" == "fix" ]]; then
    gofmt -w "${go_files[@]}"
  else
    unformatted=$(gofmt -l "${go_files[@]}")
    if [[ -n "${unformatted}" ]]; then
      echo "gofmt needed for:" >&2
      echo "${unformatted}" >&2
      exit 1
    fi
  fi
fi

buck2 clean >/dev/null

go vet ./...

if [[ "${mode}" == "fix" ]]; then
  golangci-lint run --fix
  markdownlint-cli2 --fix
else
  golangci-lint run
  markdownlint-cli2
fi
