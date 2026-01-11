#!/bin/bash

# Hook to automatically load agent instructions at session start.
# This is generic so different agent apps can use it without changes.

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-$PWD}}"
ROOT_AGENTS_MD="$PROJECT_DIR/AGENTS.md"

print_file() {
    local path="$1"
    if [ -f "$path" ]; then
        echo "[INFO] Loading agent instructions from: $path"
        echo ""
        cat "$path"
        echo ""
    fi
}

print_file "$ROOT_AGENTS_MD"
