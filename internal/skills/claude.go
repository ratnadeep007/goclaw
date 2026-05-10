package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"ratnadeep007/goclaw/internal/llm"
)

// Claude implements the call_claude skill — delegates a task to the `claude` CLI.
type Claude struct {
	timeoutSecs int
}

// NewClaude returns a Claude skill. timeoutSecs <= 0 defaults to 300.
func NewClaude(timeoutSecs int) *Claude {
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return &Claude{timeoutSecs: timeoutSecs}
}

func (c *Claude) Name() string { return "call_claude" }

func (c *Claude) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (c *Claude) Definition() llm.Tool {
	return makeTool(
		"call_claude",
		"Delegate a task to the Claude CLI agent and return its output. Use this when you want a second opinion, need Claude to write or review code, or want to parallelize sub-tasks.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The complete task description to send to Claude.",
				},
			},
			"required": []string{"task"},
		},
	)
}

func (c *Claude) Run(ctx context.Context, args map[string]any) (string, error) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("call_claude error: claude binary not found on PATH — install the Claude CLI from https://claude.ai/cli")
	}

	task, _ := args["task"].(string)
	if strings.TrimSpace(task) == "" {
		return "", fmt.Errorf("call_claude error: task argument is required")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.timeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-p", task)
	if cwd, err := os.Getwd(); err == nil {
		cmd.Dir = cwd
	}

	out, err := cmd.CombinedOutput()
	result := capOutput(string(out), maxOutputBytes)
	if err != nil {
		if result != "" {
			return result, fmt.Errorf("claude exited with error: %w", err)
		}
		return "", fmt.Errorf("call_claude error: %w", err)
	}
	return result, nil
}
