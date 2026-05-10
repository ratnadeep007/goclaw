#!/usr/bin/env bash
# opencode.sh — binary-required skill
# Delegates TASK to the `opencode` CLI in non-interactive print mode.
#
# Usage: TASK="<task>" [CWD="."] [TIMEOUT=300] bash opencode.sh
#
# Requires: `opencode` binary on PATH.

set -euo pipefail

TASK="${TASK:?TASK is required}"
CWD="${CWD:-.}"
TIMEOUT="${TIMEOUT:-300}"

# Binary check — this skill requires the opencode CLI.
if ! command -v opencode &>/dev/null; then
  echo "skill:opencode — binary not found on PATH"
  echo "Install: https://opencode.ai"
  exit 1
fi

output=$(cd "$CWD" && timeout "$TIMEOUT" opencode -p "$TASK" 2>&1) || {
  exit_code=$?
  echo "skill:opencode failed (exit $exit_code)"
  echo "$output" | head -c 2048
  exit $exit_code
}

echo "$output" | head -c 16384
