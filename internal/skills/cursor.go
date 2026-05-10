package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"ratnadeep007/goclaw/internal/llm"
)

// Cursor implements the call_cursor_agent skill.
// It auto-detects the cursor CLI's headless subcommand by inspecting `cursor --help`.
type Cursor struct {
	timeoutSecs int

	once         sync.Once
	subCmd       string // detected headless subcommand, e.g. "agent"
	detectErr    string // human-readable message if no headless mode found
	helpOutput   string // raw --help text, used in error message
}

// NewCursor returns a Cursor skill. timeoutSecs <= 0 defaults to 300.
func NewCursor(timeoutSecs int) *Cursor {
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return &Cursor{timeoutSecs: timeoutSecs}
}

func (c *Cursor) Name() string { return "call_cursor_agent" }

func (c *Cursor) Available() bool {
	_, err := exec.LookPath("cursor")
	return err == nil
}

func (c *Cursor) Definition() llm.Tool {
	return makeTool(
		"call_cursor_agent",
		"Delegate a task to the Cursor AI agent CLI and return its output.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The complete task description to send to Cursor.",
				},
				"cwd": map[string]any{
					"type":        "string",
					"description": "Optional working directory for the Cursor CLI. Defaults to the host process cwd.",
				},
			},
			"required": []string{"task"},
		},
	)
}

// headlessSubcmds are subcommand names to look for in `cursor --help` output.
var headlessSubcmds = regexp.MustCompile(`(?i)\b(agent|chat|run|headless|exec)\b`)

func (c *Cursor) detectSubCmd() {
	bin, err := exec.LookPath("cursor")
	if err != nil {
		c.detectErr = "cursor binary not found on PATH"
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(ctx, bin, "--help").CombinedOutput()
	c.helpOutput = string(out)

	if m := headlessSubcmds.FindString(c.helpOutput); m != "" {
		c.subCmd = strings.ToLower(m)
		return
	}

	c.detectErr = fmt.Sprintf(
		"call_cursor_agent: no headless subcommand detected in `cursor --help` output.\n"+
			"Detected help text snippet:\n%s",
		truncate(c.helpOutput, 512),
	)
}

func (c *Cursor) Run(ctx context.Context, args map[string]any) (string, error) {
	c.once.Do(c.detectSubCmd)

	if c.detectErr != "" {
		return "", fmt.Errorf("%s", c.detectErr)
	}

	bin, _ := exec.LookPath("cursor")
	task, _ := args["task"].(string)
	if strings.TrimSpace(task) == "" {
		return "", fmt.Errorf("call_cursor_agent error: task argument is required")
	}

	workDir, _ := args["cwd"].(string)
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.timeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, c.subCmd, task)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	result := capOutput(string(out), maxOutputBytes)
	if err != nil {
		if result != "" {
			return result, fmt.Errorf("cursor exited with error: %w", err)
		}
		return "", fmt.Errorf("call_cursor_agent error: %w", err)
	}
	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
