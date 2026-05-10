#!/usr/bin/env bash
# run_terminal.sh — core primitive
# Runs any shell command, captures combined stdout+stderr, caps output at 16 KB.
#
# Usage: COMMAND="<cmd>" [CWD="."] [TIMEOUT=30] bash run_terminal.sh
#
# No binary required. Uses bash, timeout (coreutils), head — all pre-installed.

set -euo pipefail

COMMAND="${COMMAND:?COMMAND is required}"
CWD="${CWD:-.}"
TIMEOUT="${TIMEOUT:-30}"

output=$(cd "$CWD" && timeout "$TIMEOUT" bash -c "$COMMAND" 2>&1) || {
  exit_code=$?
  echo "run_terminal: command failed (exit $exit_code)"
  echo "  command : $COMMAND"
  echo "  cwd     : $CWD"
  echo "--- output ---"
  echo "$output" | head -c 2048
  exit $exit_code
}

echo "$output" | head -c 16384
