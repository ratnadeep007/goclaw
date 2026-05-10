package skills

import (
	"context"

	"ratnadeep007/goclaw/internal/llm"
)

// Skill represents a delegatable external agent tool.
type Skill interface {
	// Name returns the tool name used in LLM function-calling (e.g. "call_claude").
	Name() string
	// Definition returns the llm.Tool JSON-schema used in the tool list sent to the LLM.
	Definition() llm.Tool
	// Run executes the skill with the provided arguments (decoded from the LLM's JSON call).
	Run(ctx context.Context, args map[string]any) (string, error)
	// Available reports whether the required external binary is present on PATH.
	Available() bool
}

// maxOutputBytes is the hard cap for skill output returned to the LLM context window (16 KB).
const maxOutputBytes = 16 * 1024

// capOutput truncates s to maxBytes, appending a notice if truncation occurred.
func capOutput(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[output truncated at 16 KB]"
}

// makeTool is a convenience helper to build an llm.Tool for function-calling.
func makeTool(name, desc string, params map[string]any) llm.Tool {
	var t llm.Tool
	t.Type = "function"
	t.Function.Name = name
	t.Function.Description = desc
	t.Function.Parameters = params
	return t
}
