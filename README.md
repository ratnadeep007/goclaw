# goclaw

An agentic CLI and HTTP service that runs code in a goshell sandbox, calls OpenAI-compatible chat APIs, and keeps markdown memory on the host filesystem.

## Run

```bash
go run ./cmd/goclaw        # TUI mode
go run ./cmd/goclaw --http # HTTP server mode
```

## Env

Copy `.env.example` to `.env` and fill in the values:

```bash
cp .env.example .env
```

### OpenAI (default)

```env
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini
```

### Ollama (local)

```env
LLM_PROVIDER=ollama
OLLAMA_BASE_URL=http://localhost:11434/v1
OLLAMA_MODEL=llama3
```

Make sure Ollama is running (`ollama serve`) and the model is pulled (`ollama pull llama3`).

### Per-role model overrides

Use different models for routing decisions, the main agent loop, and memory review:

```env
ROUTER_MODEL=llama3
AGENT_MODEL=llama3:70b
MEMORY_MODEL=llama3
```

## TUI slash commands

| Command | Description |
|---|---|
| `/provider` | Show active provider and model |
| `/provider openai` | Switch to OpenAI |
| `/provider ollama` | Switch to Ollama |
| `/ollama models` | List locally available Ollama models |
| `/ollama models <name>` | Switch active Ollama model |
| `/claude <task>` | Delegate a task to the Claude CLI |
| `/opencode <task>` | Delegate a task to the OpenCode CLI |
| `/memory brief` | Show session memory summary |
| `/memory clear` | Clear session memory |
| `/agents list` | List active subagents |
| `/sandbox list` | List active goshell sandboxes |
| `/exit` | Quit |

Provider and model changes made via `/provider` or `/ollama models <name>` are persisted to the session database and restored on the next run (unless overridden by an env var).

## Agent delegation skills

goclaw can delegate sub-tasks to external AI CLI agents:

| Tool | Binary | Notes |
|---|---|---|
| `call_claude` | `claude` | Requires `claude` CLI installed and `ANTHROPIC_API_KEY` set |
| `call_cursor_agent` | `cursor` | Auto-detects headless subcommand from `cursor --help` |
| `call_opencode` | `opencode` | Uses `opencode -p "<task>"` |

The LLM will use these tools autonomously when it decides to delegate. You can also invoke them directly with `/claude <task>` or `/opencode <task>` in the TUI.

### Skill timeout

```env
SKILL_TIMEOUT_SECS=300  # 5 minutes (default)
```
