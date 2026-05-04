#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
coverage_dir="$(mktemp -d "${TMPDIR:-/tmp}/telegram-jung2-bot-coverage.XXXXXX")"
profiles_dir="$coverage_dir/profiles"
coverage_file="$coverage_dir/telegram-jung2-bot-coverage.out"

mkdir -p "$profiles_dir"
trap 'rm -rf "$coverage_dir"' EXIT

cd "$repo_root"

if [[ -z "${COVERAGE_TEST_TARGETS:-}" ]]; then
  echo "COVERAGE_TEST_TARGETS must be set by make coverage" >&2
  exit 1
fi

read -r -a test_targets <<<"${COVERAGE_TEST_TARGETS}"
read -r -a coverage_modifiers <<<"${COVERAGE_TEST_MODIFIERS:-}"

build_output="$(
  buck2 build \
    "${coverage_modifiers[@]}" \
    "${test_targets[@]}" \
    --show-full-json-output
)"
build_json="$(printf '%s\n' "$build_output" | tail -n 1)"

printf 'mode: atomic\n' > "$coverage_file"

while IFS=$'\t' read -r target binary; do
  profile_name="$(printf '%s' "$target" | tr -c '[:alnum:]._' '_')"
  profile_path="$profiles_dir/$profile_name.out"
  stdout_path="$profiles_dir/$profile_name.stdout"
  stderr_path="$profiles_dir/$profile_name.stderr"

  if ! "$binary" -test.coverprofile="$profile_path" >"$stdout_path" 2>"$stderr_path"; then
    cat "$stdout_path"
    cat "$stderr_path" >&2
    exit 1
  fi

  if [[ -f "$profile_path" ]]; then
    {
      rg '^internal/' "$profile_path" || true
    } | rg -v '(_test\.go:|^internal/mock/)' | sed \
      "s#^internal/#$repo_root/internal/#" \
      >> "$coverage_file"
  fi
done < <(
  printf '%s\n' "$build_json" | python3 -c '
import json
import sys

for target, binary in sorted(json.load(sys.stdin).items()):
    print(f"{target}\t{binary}")
'
)

report="$(go tool cover -func="$coverage_file")"
printf '%s\n' "$report"

coverage="$(printf '%s\n' "$report" | awk '/^total:/ {print $3}')"
if [[ "$coverage" != "100.0%" ]]; then
  echo "expected 100.0% test coverage, got $coverage" >&2
  exit 1
fi
