---
name: delegate
description: |
  Delegate tasks to AI agent CLIs (Claude Code, OpenCode) or run arbitrary
  terminal commands in isolation. Use when you need sub-task delegation,
  a second AI opinion, agentic code edits, or sandboxed shell execution.

  Triggers: /delegate, "delegate to claude", "call opencode",
  "run terminal command", "use claude to review this", "run this in a
  sandbox", or any situation requiring task isolation, parallel execution,
  or a different tool than the parent agent.

  The skill name is also the slash command: `/delegate`.
author: goclaw
version: 1.0.0
---

# delegate — goclaw agent delegation

One primitive, four targets. Choose based on task type and binary
availability. Binary-required targets exit cleanly with install URLs
when missing. No silent failures.

---

## Prerequisites

```bash
# Binary checks — run these before calling binary-required targets
which claude     || echo "MISSING: npm install -g @anthropic-ai/claude-code"
which opencode   || echo "MISSING: see https://opencode.ai for install"

# For goclaw HTTP target — check server is running
curl -sf http://localhost:8080/health || echo "MISSING: go run ./cmd/goclaw --http"

# For all targets — pre-installed in standard environments: bash, curl, timeout, python3
```

---

## Targets

| Target | Binary | Pre-installed only | Use for |
|---|---|---|---|
| `run_terminal` | none | bash, timeout, head | Any shell command |
| `goclaw` | none | bash, curl, python3 | Full agent loop with memory + sandbox |
| `claude` | `claude` CLI | — | Code review, analysis, writing |
| `opencode` | `opencode` CLI | — | Agentic code edits, refactors |

---

## Two Orchestration Modes

### Mode 1: Print Mode (PREFERRED)

Non-interactive one-shot. The tool runs, prints its result, and exits.

```bash
# run_terminal — any shell command
terminal(command="go build ./...", workdir="/project", timeout=60)

# claude — non-interactive print mode
terminal(command="claude -p 'Review src/auth.go for security issues'", workdir="/project", timeout=300)

# opencode — non-interactive print mode
terminal(command="opencode -p 'Add JWT middleware to all API routes'", workdir="/project", timeout=300)

# goclaw — HTTP API call (always non-interactive)
terminal(command="curl -s -X POST http://localhost:8080/task -H 'Content-Type: application/json' -d '{\"task\":\"List all exported functions\"}'", workdir="/project", timeout=300)
```

**When to use print mode:**
- One-shot tasks (review, fix, refactor, analyze)
- CI/CD automation and scripting
- Piped input processing
- Any task where you don't need multi-turn conversation

**Print mode skips ALL interactive prompts.** — No permission dialogs, no TUI. This makes it ideal for automation.

---

### Mode 2: Interactive Sessions

For multi-turn work where you need to send follow-up prompts, watch
progress in real time, or use the agent's slash commands.

**Requires `tmux` for reliable orchestration.**

```bash
# Start tmux session
terminal(command="tmux new-session -d -s delegate-work -x 140 -y 40")

# Launch target inside it
terminal(command="tmux send-keys -t delegate-work 'cd /project && claude' Enter")

# Wait for startup, send task
terminal(command="sleep 5 && tmux send-keys -t delegate-work 'Refactor auth to use JWT tokens' Enter")

# Monitor progress
terminal(command="sleep 15 && tmux capture-pane -t delegate-work -p -S -50")

# Send follow-up
terminal(command="tmux send-keys -t delegate-work 'Now add unit tests for the new JWT code' Enter")

# Exit when done
terminal(command="tmux send-keys -t delegate-work '/exit' Enter")
```

**When to use interactive mode:**
- Multi-turn iterative work (refactor → review → fix → test cycle)
- Tasks requiring human-in-the-loop decisions
- When you need the agent's slash commands (`/compact`, `/review`, `/model`)
- Exploratory coding sessions

---

## PTY Dialog Handling (Interactive Mode Only)

Claude Code presents dialogs on first launch. You MUST handle these via
tmux `send-keys`:

### Dialog 1: Workspace Trust (first visit to a directory)
```
❯ 1. Yes, I trust this folder    ← DEFAULT (just press Enter)
  2. No, exit
```
**Handling:** `tmux send-keys -t <session> Enter`

### Dialog 2: Permissions Warning (only with --dangerously-skip-permissions)
```
❯ 1. No, exit                    ← DEFAULT (WRONG choice!)
  2. Yes, I accept
```
**Handling:** Must navigate DOWN first:
```bash
tmux send-keys -t <session> Down && sleep 0.3 && tmux send-keys -t <session> Enter
```

**Note:** After the first trust acceptance for a directory, the trust dialog
won't appear again. The permissions dialog recurs each time you use
`--dangerously-skip-permissions`.

---

## Target Details

### run_terminal — Core Primitive

No binary required. Uses `bash`, `timeout` (coreutils), `head` — all
pre-installed in standard environments.

```bash
# Simple command
cd "<cwd>" && timeout <seconds> bash -c "<command>" 2>&1
```

**Env-style invocation:**
```bash
COMMAND="<cmd>" CWD="." TIMEOUT=30 bash skills/scripts/run_terminal.sh
```

| Parameter | Default | Description |
|---|---|---|
| `COMMAND` | _(required)_ | Shell command to execute |
| `CWD` | `.` | Working directory |
| `TIMEOUT` | `30` | Seconds before kill |

**Output:** combined stdout+stderr, capped at 16 KB.

**Always capture exit code immediately after the command:**
```bash
output=$(cd "$CWD" && timeout "$TIMEOUT" bash -c "$COMMAND" 2>&1)
exit_code=$?
```

---

### claude — Binary Required

Delegates `task` to the Claude Code CLI in non-interactive print mode.

**Requires:** `claude` on PATH; `ANTHROPIC_API_KEY` set in environment.

```bash
# Print mode (preferred)
terminal(command="claude -p '<task>' --max-turns 10", workdir="/project", timeout=300)

# With allowed tools restricted
cd "<cwd>" && timeout 300 claude -p "<task>" --allowedTools "Read,Edit" --max-turns 10 2>&1
```

**Print mode CLI flags:**
| Flag | Effect |
|---|---|
| `-p, --print` | Non-interactive one-shot mode (exits when done) |
| `--max-turns <n>` | Limit agentic loops (prevents runaway) |
| `--max-budget-usd <n>` | Cap API spend in dollars (print mode only) |
| `--allowedTools <tools>` | Whitelist specific tools (e.g., "Read,Edit") |
| `--disallowedTools <tools>` | Blacklist specific tools |
| `--model <alias>` | `sonnet`, `opus`, `haiku`, or full name |
| `--effort <level>` | Reasoning depth: `low`, `medium`, `high`, `max`, `auto` |
| `--output-format <fmt>` | `text` (default), `json` (single result), `stream-json` |
| `--bare` | Skip hooks, plugins, MCP discovery, CLAUDE.md (fastest startup) |
| `--continue` | Resume most recent conversation in directory |
| `--resume <id>` | Resume specific session by ID |
| `--dangerously-skip-permissions` | Auto-approve ALL tool use |
| `--fallback-model <model>` | Auto-fallback when default is overloaded |

**Piped input:**
```bash
terminal(command="git diff HEAD~3 | claude -p 'Summarize these changes' --max-turns 1", timeout=60)
```

**JSON schema for structured output:**
```bash
terminal(command="claude -p 'List all functions in src/' --output-format json --json-schema '{\"type\":\"object\",\"properties\":{\"functions\":{\"type\":\"array\",\"items\":{\"type\":\"string\"}}},\"required\":[\"functions\"]}' --max-turns 5", timeout=90)
```

**Session continuation:**
```bash
# Start and save session ID
cd /project && claude -p "Start refactoring" --output-format json --max-turns 10 > /tmp/session.json

# Resume with session ID
cd /project && claude -p "Continue and add pooling" --resume $(python3 -c "import json,sys; print(json.load(sys.stdin)['session_id'])") --max-turns 5 < /tmp/session.json
```

---

### opencode — Binary Required

Delegates `task` to the OpenCode CLI in non-interactive print mode.

**Requires:** `opencode` on PATH.

```bash
# Print mode (preferred)
terminal(command="opencode -p '<task>'", workdir="/project", timeout=300)
```

OpenCode's CLI is simpler than Claude Code — it has fewer flags. Core print
mode is `opencode -p "<task>"`. It runs the task and exits with the result.

**Key parameters:**
| Flag | Effect |
|---|---|
| `-p, --print` | Non-interactive mode (exits when done) |
| `--cwd <path>` | Set working directory |

---

### goclaw — No Binary Required

Submits `task` to a running goclaw HTTP server. Uses `curl` and `python3`
(pre-installed). No external binary needed.

**Requires:** goclaw running in `--http` mode.

```bash
# Start goclaw HTTP server first
go run ./cmd/goclaw --http

# Submit task via curl
curl -s -X POST "${GOCLAW_URL:-http://localhost:8080}/task" \
  -H "Content-Type: application/json" \
  -d "{\"task\": \"$TASK\"}"
```

**Response shape:**
```json
{ "output": "...", "tools": [...], "error": null }
```

```bash
# Extract output field; fall back to raw response on parse error
terminal(command="python3 -c \"
import sys, json, os
raw = sys.stdin.read()
try:
    d = json.loads(raw)
    err = d.get('error')
    if err:
        print(f'goclaw error: {err}', file=sys.stderr)
        sys.exit(1)
    print(d.get('output', ''))
except json.JSONDecodeError:
    sys.stdout.write(raw)
\" < <(curl -s -X POST http://localhost:8080/task -H 'Content-Type: application/json' -d '{\"task\": \"$TASK\"}')", timeout=300)
```

**When to use goclaw instead of claude/opencode directly:**
- Tasks that need shell execution + web search + persistent memory in one loop
- When you want to reuse goclaw's sandbox (goshell) rather than a raw shell
- When goclaw is already running and you want to leverage its session state

---

## Routing Guide

Pick the right target based on task shape:

| Task type | Target | Binary needed? |
|---|---|---|
| Code review, analysis, Q&A | `claude` | Yes |
| Code edits, refactors, file writes | `opencode` | Yes |
| Build, test, grep, git, any shell command | `run_terminal` | No |
| Full agent loop (shell + search + memory) | `goclaw` | No |
| Two or more independent sub-tasks | `parallel` (see below) | Varies |

---

## Parallel Execution

Run multiple targets concurrently when sub-tasks are independent.

**Using tmux background sessions:**
```bash
# Start multiple tmux sessions
cd /project && tmux new-session -d -s task1 -x 140 -y 40
cd /project && tmux new-session -d -s task2 -x 140 -y 40

# Fire tasks
tmux send-keys -t task1 "claude -p 'Audit security in auth module' --max-turns 8" Enter
tmux send-keys -t task2 "opencode -p 'Optimize database queries in user.go'" Enter

# Wait and collect
sleep 60
terminal(command="tmux capture-pane -t task1 -p -S -50")
terminal(command="tmux capture-pane -t task2 -p -S -50")

# Clean up
tmux kill-session -t task1
tmux kill-session -t task2
```

**Using shell background jobs (simpler, no tmux):**
```bash
TMPDIR=$(mktemp -d)

# Run in background, write to temp files
(cd /project && timeout 300 claude -p "Task A" 2>&1 | head -c 16384) > "$TMPDIR/a.txt" &
PID_A=$!

(cd /project && timeout 300 opencode -p "Task B" 2>&1 | head -c 16384) > "$TMPDIR/b.txt" &
PID_B=$!

# Wait for both
wait $PID_A; STATUS_A=$?
wait $PID_B; STATUS_B=$?

# Read results
OUT_A=$(cat "$TMPDIR/a.txt")
OUT_B=$(cat "$TMPDIR/b.txt")
rm -rf "$TMPDIR"

[ $STATUS_A -ne 0 ] && echo "claude failed (exit $STATUS_A)"
[ $STATUS_B -ne 0 ] && echo "opencode failed (exit $STATUS_B)"
```

---

## Workflow

```
1. IDENTIFY   — What is the sub-task? Can it run independently?
2. PICK       — Choose target from routing guide above.
3. PRECHECK   — Run binary check if target requires a binary.
                For binary-free targets, skip this step.
4. CALL       — Invoke via terminal(command=...). Set appropriate timeout.
5. VALIDATE   — Check exit code. Non-zero → surface error with command + output.
6. TRUNCATE   — Cap output at 16 KB before inserting into parent context.
7. COMPOSE    — Merge sub-task results into the parent answer.
```

---

## Examples

### Example 1: Review with Claude, implement with OpenCode

Sequential delegation. Claude reviews, OpenCode implements.

```bash
# Step 1: Claude reviews (binary required)
which claude || { echo "claude not found — skip review"; exit 0; }
REVIEW=$(cd /project && timeout 300 claude -p "Review src/auth.go for race conditions and suggest fixes" --max-turns 5 2>&1)

# Step 2: OpenCode implements (binary required)
which opencode || { echo "opencode not found"; exit 1; }
cd /project && timeout 300 opencode -p "Apply these fixes to src/auth.go:

$REVIEW" 2>&1 | head -c 16384
```

---

### Example 2: Build fails → Claude explains why

`run_terminal` checks the build. On failure, `claude` analyzes the error.

```bash
# run_terminal: no binary required
BUILD=$(cd /project && timeout 60 bash -c "go build ./..." 2>&1)
BUILD_EXIT=$?

if [ $BUILD_EXIT -ne 0 ]; then
  which claude || { echo "claude not found — manual fix needed"; echo "$BUILD"; exit 1; }

  # Claude explains and suggests exact fixes
  FIX=$(cd /project && timeout 300 claude -p "This Go build failed with exit code $BUILD_EXIT.
Explain the errors and provide exact fixes for each:

\`\`\`
$BUILD
\`\`\`" --max-turns 5 2>&1)

  echo "=== Build Output ==="
  echo "$BUILD" | head -c 2048
  echo ""
  echo "=== Claude's Analysis ==="
  echo "$FIX" | head -c 16384
fi
```

---

### Example 3: Parallel security + performance audit

Both targets run concurrently via background jobs.

```bash
TMPDIR=$(mktemp -d)

# Security audit (binary required: claude)
which claude && \
  (cd /project && timeout 300 claude -p "List all security issues in this codebase" --max-turns 5 2>&1 | head -c 16384) > "$TMPDIR/security.txt" &
PID_A=$!

# Performance audit (binary required: opencode)
which opencode && \
  (cd /project && timeout 300 opencode -p "List all performance issues in this codebase" 2>&1 | head -c 16384) > "$TMPDIR/perf.txt" &
PID_B=$!

# Wait for completion
wait $PID_A; wait $PID_B

# Output
echo "=== Security Audit ==="
cat "$TMPDIR/security.txt" 2>/dev/null || echo "(claude not available)"
echo ""
echo "=== Performance Audit ==="
cat "$TMPDIR/perf.txt" 2>/dev/null || echo "(opencode not available)"

rm -rf "$TMPDIR"
```

**Key:** `which claude && (cmd) &` — only runs if binary exists. No hard failure if one tool is missing.

---

### Example 4: Run any terminal command (no AI agent)

`run_terminal` directly — no AI delegation at all.

```bash
# git log
cd /project && timeout 30 bash -c "git log --oneline -20" 2>&1

# Run tests
cd /project && timeout 120 bash -c "go test ./..." 2>&1

# Check a port
timeout 5 bash -c "curl -sf http://localhost:8080/health" 2>&1

# Count lines in last commit
cd /project && timeout 10 bash -c "git diff HEAD~1 --stat" 2>&1
```

---

### Example 5: Query running goclaw (no binary required)

```bash
# Health check first
curl -sf http://localhost:8080/health || { echo "goclaw not running"; exit 1; }

# Submit task
RESPONSE=$(timeout 300 curl -s -X POST http://localhost:8080/task \
  -H "Content-Type: application/json" \
  -d '{"task": "Summarise all tools in internal/agent/agent.go"}' 2>&1)

# Extract output field
echo "$RESPONSE" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('output', ''))
except json.JSONDecodeError:
    print('Parse error:', sys.stdin.read()[:500])
" | head -c 16384
```

---

## Error Reference

| Condition | Action |
|---|---|
| `which <binary>` fails | Print binary name + install URL. Skip gracefully in parallel mode (`which X && cmd &`). |
| Non-zero exit code | Echo: `failed (exit N): <command>` then first 2 KB of output. |
| Timeout (exit 124 from `timeout`) | Echo: `timed out after N seconds`. Include any partial output. |
| Empty output | Warn, retry once with more explicit task string. |
| JSON parse error (goclaw) | Surface raw HTTP body (first 500 chars) for debugging. |
| `ANTHROPIC_API_KEY` missing | Warn before calling claude. Binary precheck covers this. |

---

## Gotchas

1. **Interactive mode requires tmux.** Claude Code and OpenCode are TUI apps. Using `pty=true` alone works but tmux gives you `capture-pane` and `send-keys`, essential for orchestration.

2. **Binary checks belong in pre-execution, not in SKILL.md.** The skill instructions tell you to run `which` before delegating. The actual bash execution is just `claude -p "..."`.

3. **Print mode (`-p`) skips ALL dialogs** — no workspace trust prompt, no permission confirmations. This is why print mode is preferred for automation.

4. **Claude Code's `--max-turns` is print-mode only.** In interactive tmux sessions it is ignored.

5. **Context degradation is real.** AI output quality drops above 70% context usage. If a delegated task returns >16 KB, it was probably too large. Use `/compact` or break into smaller sub-tasks.

6. **Timeout defaults differ:** `run_terminal` = 30s. `claude`/`opencode` = 300s. `goclaw` = 300s. Adjust per task.

7. **Parallel execution output files.** When running multiple targets concurrently, always write to separate temp files and `wait` for all PIDs before reading. Reading a file while a background process is still writing to it corrupts output.

8. **Cap at 16 KB before returning to parent context.** `head -c 16384` is the standard guard. Use it on every skill invocation.

9. **Don't kill slow sessions immediately.** A Claude Code task may take multiple turns. Use `tmux capture-pane` to check progress before deciding it's hung.

---

## Rules for Agents

1. **Prefer print mode (`-p`) for single tasks.** Cleaner, no dialog handling, no tmux overhead.
2. **Use tmux for multi-turn interactive work.** The only reliable way to orchestrate a TUI agent.
3. **Always set `workdir`.** Keeps the delegated agent focused on the right project directory.
4. **Set `--max-turns` in print mode.** Prevents runaway loops and runaway costs.
5. **Run `which` prechecks for binary-required targets.** Never silently fail.
6. **Use `--allowedTools` to restrict capabilities.** E.g., `--allowedTools "Read"` for a pure review task.
7. **Monitor tmux sessions with `capture-pane`.** Check for the `❯` prompt (waiting for input).
8. **Clean up tmux sessions.** `tmux kill-session -t <name>` when done.
9. **Report results to the user.** After completion, summarize what the delegated agent did and what changed.
10. **Never pass secrets in task strings.** API keys, tokens, passwords belong in env vars, never in `-p` task text.
11. **One task at a time per target unless using `parallel`.** Don't queue multiple tasks into the same tmux session — send one, wait for `❯`, then send the next.
12. **For goclaw HTTP target:** Check `http://localhost:8080/health` before every task. goclaw may have been restarted.
