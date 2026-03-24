package config

import (
	"os"
	"path/filepath"
	"strings"

	"ratnadeep007/goclaw/internal/paths"
)

type Config struct {
	OpenAIAPIKey  string
	OpenAIBaseURL string
	OpenAIModel   string
	ExaAPIKey     string
	ExaBaseURL    string
	GoshellBin    string
	MemoryDir     string
	PromptDir     string
	ShowToolCalls bool
	MaxTurns      int
	SandboxPrompt string
}

func Load() Config {
	cfg := Config{
		OpenAIBaseURL: env("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:   env("OPENAI_MODEL", "gpt-4o-mini"),
		ExaBaseURL:    env("EXA_BASE_URL", "https://api.exa.ai/search"),
		ExaAPIKey:     os.Getenv("EXA_API_KEY"),
		GoshellBin:    env("GOSHELL_BIN", "../golang-shell/goshell"),
		MemoryDir:     env("GCLAW_MEMORY_DIR", "./memory"),
		PromptDir:     env("GCLAW_PROMPT_DIR", filepath.Join(paths.ConfigDir("goclaw"), "prompts")),
		ShowToolCalls: envBool("SHOW_TOOL_CALLS", false),
		MaxTurns:      12,
		SandboxPrompt: env("GOSHELL_PROMPT", "__G_CLAW__> "),
	}
	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	if cfg.OpenAIAPIKey == "" {
		cfg.OpenAIAPIKey = os.Getenv("OPENAI_KEY")
	}
	return cfg
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
