#!/usr/bin/env bash
set -euo pipefail

install_dir="${HOME}/.cargo/bin"
mkdir -p "${install_dir}"

case "$(uname -s)" in
  Darwin)
    for formula in golangci-lint shellcheck typos-cli markdownlint-cli2; do
      brew list "${formula}" >/dev/null 2>&1 || brew install "${formula}"
    done
    ;;
  Linux)
    if ! command -v golangci-lint >/dev/null 2>&1; then
      GOBIN="${install_dir}" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    fi

    if ! command -v shellcheck >/dev/null 2>&1; then
      sudo apt-get update -qq && sudo apt-get install -y -qq shellcheck
    fi

    if ! command -v typos >/dev/null 2>&1; then
      if ! command -v cargo >/dev/null 2>&1; then
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        # shellcheck disable=SC1091
        source "${HOME}/.cargo/env"
      fi
      cargo install typos-cli --locked
    fi

    if ! command -v markdownlint-cli2 >/dev/null 2>&1; then
      npm install -g markdownlint-cli2
    fi
    ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

echo "Lint tools ready."
