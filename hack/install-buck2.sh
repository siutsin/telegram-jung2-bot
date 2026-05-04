#!/usr/bin/env bash
set -euo pipefail

case "$(uname -s)" in
  Darwin) platform="apple-darwin" ;;
  Linux) platform="unknown-linux-gnu" ;;
  *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) cpu="aarch64" ;;
  x86_64|amd64) cpu="x86_64" ;;
  riscv64) cpu="riscv64gc" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if ! command -v zstd >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    brew install zstd
  else
    echo "zstd is required; install it with your package manager and rerun this script" >&2
    exit 1
  fi
fi

install_dir="${HOME}/.cargo/bin"
mkdir -p "${install_dir}"

archive="buck2-${cpu}-${platform}.zst"
url="https://github.com/facebook/buck2/releases/download/latest/${archive}"
tmp="${TMPDIR:-/tmp}/${archive}"

echo "Downloading ${url}"
curl -fL "${url}" -o "${tmp}"
zstd -d -f "${tmp}" -o "${install_dir}/buck2"
chmod +x "${install_dir}/buck2"
"${install_dir}/buck2" --version

echo "Installed ${install_dir}/buck2"
echo "Ensure ${install_dir} is in PATH."
