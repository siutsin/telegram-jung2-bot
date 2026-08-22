#!/usr/bin/env bash
set -euo pipefail

mode="${1:-check}"
if [[ "${mode}" != "check" && "${mode}" != "fix" ]]; then
  echo "usage: $0 [check|fix]" >&2
  exit 2
fi

golangci_lint="${GOLANGCI_LINT:-golangci-lint}"

go_files=()
while IFS= read -r go_file; do
  go_files+=("${go_file}")
done < <(find . \
  -type f \
  -name '*.go' \
  -not -path './buck-out/*' \
  -not -path './internal/mock/*' \
  -not -path './l[e]gacy/*' \
  -not -path './node_modules/*' \
  -not -path './vendor/*' \
  | sort)

shell_files=()
while IFS= read -r shell_file; do
  shell_files+=("${shell_file}")
done < <(find . \
  -type f \
  -name '*.sh' \
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

go_packages=(./cmd/...)
if [[ -d ./tools ]]; then
  go_packages+=(./tools/...)
fi
while IFS= read -r go_package; do
  go_packages+=("${go_package}/...")
done < <(find ./internal \
  -mindepth 1 \
  -maxdepth 1 \
  -type d \
  -not -path './internal/mock' \
  | sort)

go vet "${go_packages[@]}"

if ((${#shell_files[@]} > 0)); then
  shellcheck "${shell_files[@]}"
fi

typos_args=(
  --exclude ./buck-out
  --exclude ./internal/mock
  --exclude ./node_modules
  --exclude ./vendor
  .
)

if [[ "${mode}" == "fix" ]]; then
  "${golangci_lint}" run --fix "${go_packages[@]}"
  typos --write-changes "${typos_args[@]}"
  markdownlint-cli2 --fix
else
  "${golangci_lint}" run "${go_packages[@]}"
  typos "${typos_args[@]}"
  markdownlint-cli2
fi
