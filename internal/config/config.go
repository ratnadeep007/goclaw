package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ratnadeep007/goclaw/internal/paths"
)

type Config struct {
	// OpenAI provider
	OpenAIAPIKey  string
	OpenAIBaseURL string
	OpenAIModel   string

	// Provider selection
	Provider string // "openai" | "ollama" — env: LLM_PROVIDER

	// Ollama provider
	OllamaBaseURL string // env: OLLAMA_BASE_URL (fallback: OLLAMA_HOST)
	OllamaModel   string // env: OLLAMA_MODEL

	// Per-role model overrides (fall back to provider default if empty)
	RouterModel string // env: ROUTER_MODEL
	AgentModel  string // env: AGENT_MODEL
	MemoryModel string // env: MEMORY_MODEL

	// Skills
	SkillTimeoutSecs int // env: SKILL_TIMEOUT_SECS (default: 300)

	// Infrastructure
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
		OpenAIBaseURL:    env("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:      env("OPENAI_MODEL", "gpt-4o-mini"),
		Provider:         env("LLM_PROVIDER", "openai"),
		OllamaBaseURL:    ollamaBaseURL(),
		OllamaModel:      env("OLLAMA_MODEL", "llama3"),
		RouterModel:      os.Getenv("ROUTER_MODEL"),
		AgentModel:       os.Getenv("AGENT_MODEL"),
		MemoryModel:      os.Getenv("MEMORY_MODEL"),
		SkillTimeoutSecs: envInt("SKILL_TIMEOUT_SECS", 300),
		ExaBaseURL:       env("EXA_BASE_URL", "https://api.exa.ai/search"),
		ExaAPIKey:        os.Getenv("EXA_API_KEY"),
		GoshellBin:       env("GOSHELL_BIN", "../golang-shell/goshell"),
		MemoryDir:        env("GCLAW_MEMORY_DIR", "./memory"),
		PromptDir:        env("GCLAW_PROMPT_DIR", filepath.Join(paths.ConfigDir("goclaw"), "prompts")),
		ShowToolCalls:    envBool("SHOW_TOOL_CALLS", false),
		MaxTurns:         12,
		SandboxPrompt:    env("GOSHELL_PROMPT", "__G_CLAW__> "),
	}
	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	if cfg.OpenAIAPIKey == "" {
		cfg.OpenAIAPIKey = os.Getenv("OPENAI_KEY")
	}
	return cfg
}

// ActiveModel returns the model string for the given role ("router", "agent", "memory"),
// using per-role overrides if set and falling back to the provider default.
func (c *Config) ActiveModel(role string) string {
	switch role {
	case "router":
		if c.RouterModel != "" {
			return c.RouterModel
		}
	case "agent":
		if c.AgentModel != "" {
			return c.AgentModel
		}
	case "memory":
		if c.MemoryModel != "" {
			return c.MemoryModel
		}
	}
	if strings.ToLower(c.Provider) == "ollama" {
		return c.OllamaModel
	}
	return c.OpenAIModel
}

// ActiveBaseURL returns the base URL for the active provider.
func (c *Config) ActiveBaseURL() string {
	if strings.ToLower(c.Provider) == "ollama" {
		return c.OllamaBaseURL
	}
	return c.OpenAIBaseURL
}

// ActiveAPIKey returns the API key for the active provider.
// Ollama does not require a key; returns empty string.
func (c *Config) ActiveAPIKey() string {
	if strings.ToLower(c.Provider) == "ollama" {
		return ""
	}
	return c.OpenAIAPIKey
}

// ollamaBaseURL resolves OLLAMA_BASE_URL with fallback to OLLAMA_HOST.
func ollamaBaseURL() string {
	if v := os.Getenv("OLLAMA_BASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		// OLLAMA_HOST may be bare host:port without /v1 path
		v = strings.TrimRight(v, "/")
		if !strings.HasSuffix(v, "/v1") {
			v += "/v1"
		}
		return v
	}
	return "http://localhost:11434/v1"
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

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
