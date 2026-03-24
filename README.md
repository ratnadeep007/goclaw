# goclaw

An agentic CLI and HTTP service that runs code in a goshell sandbox, calls OpenAI-compatible chat APIs, and keeps markdown memory on the host filesystem.

## Run

```bash
go run ./cmd/goclaw
```

```bash
go run ./cmd/goclaw --http
```

## Env

Create a `.env` file with:

```bash
OPENAI_API_KEY=...
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini
GOSHELL_BIN=../golang-shell/goshell
GCLAW_MEMORY_DIR=./memory
```
