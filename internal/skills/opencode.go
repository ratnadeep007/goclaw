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

// OpenCode implements the call_opencode skill — delegates a task to the `opencode` CLI.
type OpenCode struct {
	timeoutSecs int
}

// NewOpenCode returns an OpenCode skill. timeoutSecs <= 0 defaults to 300.
func NewOpenCode(timeoutSecs int) *OpenCode {
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return &OpenCode{timeoutSecs: timeoutSecs}
}

func (o *OpenCode) Name() string { return "call_opencode" }

func (o *OpenCode) Available() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (o *OpenCode) Definition() llm.Tool {
	return makeTool(
		"call_opencode",
		"Delegate a task to the OpenCode AI coding agent CLI and return its output. Use this to run agentic coding tasks, refactors, or file edits in the host project.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The complete task description to send to OpenCode.",
				},
			},
			"required": []string{"task"},
		},
	)
}

func (o *OpenCode) Run(ctx context.Context, args map[string]any) (string, error) {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return "", fmt.Errorf("call_opencode error: opencode binary not found on PATH — install opencode (https://opencode.ai)")
	}

	task, _ := args["task"].(string)
	if strings.TrimSpace(task) == "" {
		return "", fmt.Errorf("call_opencode error: task argument is required")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(o.timeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-p", task)
	if cwd, err := os.Getwd(); err == nil {
		cmd.Dir = cwd
	}

	out, err := cmd.CombinedOutput()
	result := capOutput(string(out), maxOutputBytes)
	if err != nil {
		if result != "" {
			return result, fmt.Errorf("opencode exited with error: %w", err)
		}
		return "", fmt.Errorf("call_opencode error: %w", err)
	}
	return result, nil
}
