#!/usr/bin/env bash
# claude.sh — binary-required skill
# Delegates TASK to the `claude` CLI in non-interactive print mode.
#
# Usage: TASK="<task>" [CWD="."] [TIMEOUT=300] bash claude.sh
#
# Requires: `claude` binary on PATH, ANTHROPIC_API_KEY set in environment.

set -euo pipefail

TASK="${TASK:?TASK is required}"
CWD="${CWD:-.}"
TIMEOUT="${TIMEOUT:-300}"

# Binary check — this skill requires the claude CLI.
if ! command -v claude &>/dev/null; then
  echo "skill:claude — binary not found on PATH"
  echo "Install: https://claude.ai/cli"
  exit 1
fi

if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "skill:claude — warning: ANTHROPIC_API_KEY is not set (claude may fail)"
fi

output=$(cd "$CWD" && timeout "$TIMEOUT" claude -p "$TASK" 2>&1) || {
  exit_code=$?
  echo "skill:claude failed (exit $exit_code)"
  echo "$output" | head -c 2048
  exit $exit_code
}

echo "$output" | head -c 16384
