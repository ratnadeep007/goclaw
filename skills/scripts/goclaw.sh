#!/usr/bin/env bash
# goclaw.sh — no binary required
# Submits TASK to a running goclaw HTTP server and returns the agent output.
# Uses curl and python3 — both pre-installed in standard environments.
#
# Usage: TASK="<task>" [BASE_URL="http://localhost:8080"] [TIMEOUT=300] bash goclaw.sh

set -euo pipefail

TASK="${TASK:?TASK is required}"
BASE_URL="${GOCLAW_URL:-${BASE_URL:-http://localhost:8080}}"
TIMEOUT="${TIMEOUT:-300}"

# Health check — no binary required, just curl.
if ! curl -sf "$BASE_URL/health" &>/dev/null; then
  echo "skill:goclaw — server not reachable at $BASE_URL"
  echo "Start goclaw with: go run ./cmd/goclaw --http"
  exit 1
fi

# Submit the task.
response=$(timeout "$TIMEOUT" curl -s -X POST "$BASE_URL/task" \
  -H "Content-Type: application/json" \
  -d "{\"task\": $(python3 -c "import json,sys; print(json.dumps(sys.argv[1]))" "$TASK")}" \
  2>&1) || {
  exit_code=$?
  echo "skill:goclaw — curl failed (exit $exit_code): $response"
  exit $exit_code
}

# Extract the output field; fall back to raw response on parse error.
python3 - "$response" <<'EOF'
import sys, json
raw = sys.argv[1]
try:
    d = json.loads(raw)
    err = d.get("error")
    if err:
        print(f"skill:goclaw error: {err}", file=sys.stderr)
        sys.exit(1)
    print(d.get("output", ""))
except json.JSONDecodeError:
    print(raw)
EOF
