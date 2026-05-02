#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
coverage_dir="$(mktemp -d "${TMPDIR:-/tmp}/telegram-jung2-bot-coverage.XXXXXX")"
profiles_dir="$coverage_dir/profiles"
coverage_file="$coverage_dir/telegram-jung2-bot-coverage.out"

mkdir -p "$profiles_dir"
trap 'rm -rf "$coverage_dir"' EXIT

cd "$repo_root"

test_targets=()
while IFS= read -r target; do
  test_targets+=("$target")
done < <(buck2 uquery "kind('go_test', //...)" | sort)

if [[ "${#test_targets[@]}" -eq 0 ]]; then
  echo "no go_test targets found" >&2
  exit 1
fi

build_output="$(
  buck2 build \
    -m 'toolchains//:race' \
    -m 'prelude//go/constraints:coverage_mode[atomic]' \
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
      rg '^(cmd|internal)/' "$profile_path" || true
    } | rg -v '_test\.go:' | sed \
      "s#^cmd/#$repo_root/cmd/#; s#^internal/#$repo_root/internal/#" \
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
