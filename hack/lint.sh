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
  -not -path './vendor/*' \
  | sort)

shell_files=()
while IFS= read -r shell_file; do
  shell_files+=("${shell_file}")
done < <(find . \
  -type f \
  -name '*.sh' \
  -not -path './buck-out/*' \
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

go_packages=(./cmd/... ./internal)
while IFS= read -r go_package; do
	if find "${go_package}" -maxdepth 1 -type f -name '*.go' -print -quit | rg -q .; then
		go_packages+=("${go_package}/...")
	fi
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

if [[ "${mode}" == "fix" ]]; then
  "${golangci_lint}" run --fix "${go_packages[@]}"
  markdownlint-cli2 --fix
else
  "${golangci_lint}" run "${go_packages[@]}"
  markdownlint-cli2
fi
