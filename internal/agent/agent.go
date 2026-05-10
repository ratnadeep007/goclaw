package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"ratnadeep007/goclaw/internal/config"
	"ratnadeep007/goclaw/internal/llm"
	"ratnadeep007/goclaw/internal/memory"
	"ratnadeep007/goclaw/internal/prompts"
	"ratnadeep007/goclaw/internal/runtime"
	"ratnadeep007/goclaw/internal/sandbox"
	"ratnadeep007/goclaw/internal/search"
	"ratnadeep007/goclaw/internal/skills"
)

type Agent struct {
	Cfg          config.Config
	Client       *llm.Client // main agent client (role: "agent")
	RouterClient *llm.Client // routing decisions (role: "router")
	MemoryClient *llm.Client // memory review (role: "memory")
	Search       *search.Client
	Memory       *memory.Store
	Prompts      *prompts.Manager
	AskUser      func(question string) (string, error)
	Sandbox      *sandbox.Sandbox
	MaxTurns     int
	Skills       []skills.Skill
}

type ToolResult struct {
	Name    string
	Call    string
	Output  string
	ErrText string
}

type routeDecision struct {
	Mode    string `json:"mode"`
	Message string `json:"message"`
}

func New(cfg config.Config, mem *memory.Store, sb *sandbox.Sandbox) *Agent {
	return &Agent{
		Cfg:          cfg,
		Client:       llm.NewClientForRole("agent", cfg),
		RouterClient: llm.NewClientForRole("router", cfg),
		MemoryClient: llm.NewClientForRole("memory", cfg),
		Search:       search.New(cfg.ExaBaseURL, cfg.ExaAPIKey),
		Memory:       mem,
		Prompts:      prompts.New(cfg.PromptDir),
		AskUser:      nil,
		Sandbox:      sb,
		MaxTurns:     cfg.MaxTurns,
		Skills: []skills.Skill{
			skills.NewClaude(cfg.SkillTimeoutSecs),
			skills.NewCursor(cfg.SkillTimeoutSecs),
			skills.NewOpenCode(cfg.SkillTimeoutSecs),
		},
	}
}

func (a *Agent) Run(ctx context.Context, task string, history []llm.Message, syncMemory func() error) (string, []ToolResult, error) {
	if a.Memory != nil {
		_ = a.Memory.Append("plans", "Task: "+task)
	}
	memoryCtx := ""
	if a.Memory != nil {
		memoryCtx, _ = a.Memory.LoadContext()
	}
	route, err := a.route(ctx, task, history, memoryCtx)
	if err != nil {
		return "", nil, err
	}
	if route.Mode == "clarify" || route.Mode == "direct" {
		final := strings.TrimSpace(route.Message)
		if route.Mode == "direct" {
			a.reviewAndUpdateMemory(task, final, nil, syncMemory)
		}
		return final, nil, nil
	}
	system, err := a.Prompts.Render("system", map[string]string{"MEMORY": memoryCtx})
	if err != nil {
		return "", nil, err
	}
	messages := make([]llm.Message, 0, len(history)+2)
	messages = append(messages, llm.Message{Role: "system", Content: system})
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: "user", Content: task})
	tools := a.buildTools()
	var results []ToolResult
	var lastAssistant string
	for turn := 0; turn < a.MaxTurns; turn++ {
		msg, err := a.Client.Chat(ctx, messages, tools)
		if err != nil {
			return "", results, err
		}
		messages = append(messages, msg)
		if len(msg.ToolCalls) == 0 {
			final := strings.TrimSpace(msg.Content)
			lastAssistant = final
			a.reviewAndUpdateMemory(task, final, results, syncMemory)
			return final, results, nil
		}
		if strings.TrimSpace(msg.Content) != "" {
			lastAssistant = strings.TrimSpace(msg.Content)
		}
		for _, call := range msg.ToolCalls {
			output, toolErr := a.runTool(ctx, call.Function.Name, call.Function.Arguments)
			if toolErr != nil {
				output = toolErr.Error()
			}
			results = append(results, ToolResult{Name: call.Function.Name, Call: formatToolCall(call.Function.Name, call.Function.Arguments), Output: output, ErrText: errString(toolErr)})
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: output})
		}
	}
	if strings.TrimSpace(lastAssistant) != "" {
		a.reviewAndUpdateMemory(task, lastAssistant, results, syncMemory)
		return lastAssistant, results, nil
	}
	return "", results, fmt.Errorf("max turns exceeded")
}

func (a *Agent) route(ctx context.Context, task string, history []llm.Message, memoryCtx string) (routeDecision, error) {
	prompt, err := a.Prompts.Render("router", map[string]string{
		"MEMORY":  memoryCtx,
		"HISTORY": renderHistory(history),
		"TASK":    task,
	})
	if err != nil {
		return routeDecision{}, err
	}
	msg, err := a.RouterClient.Chat(ctx, []llm.Message{{Role: "system", Content: prompt}}, nil)
	if err != nil {
		return routeDecision{}, err
	}
	var out routeDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &out); err != nil {
		return routeDecision{Mode: "tool", Message: "use tools"}, nil
	}
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	if out.Mode == "" {
		out.Mode = "tool"
	}
	if out.Mode != "direct" && out.Mode != "clarify" && out.Mode != "tool" {
		out.Mode = "tool"
	}
	return out, nil
}

func (a *Agent) runTool(ctx context.Context, name, argsJSON string) (string, error) {
	// Check skill tools first.
	for _, sk := range a.Skills {
		if sk.Name() == name {
			var args map[string]any
			if argsJSON != "" {
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return "", fmt.Errorf("parse args for %s: %w", name, err)
				}
			}
			return sk.Run(ctx, args)
		}
	}

	switch name {
	case "list_ollama_models":
		models, err := llm.ListModels(ctx, a.Cfg.OllamaBaseURL)
		if err != nil {
			return "", fmt.Errorf("list_ollama_models: %w", err)
		}
		if len(models) == 0 {
			return "No local Ollama models found.", nil
		}
		return "Available Ollama models:\n" + strings.Join(models, "\n"), nil
	case "shell_exec":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		return a.Sandbox.Exec(args.Command)
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		return a.Sandbox.ReadFile(args.Path)
	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if err := a.Sandbox.WriteFile(args.Path, args.Content); err != nil {
			return "", err
		}
		return "written " + args.Path, nil
	case "dump_sandbox":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if err := a.Sandbox.Dump(args.Path); err != nil {
			return "", err
		}
		return "dumped to " + args.Path, nil
	case "spawn_subagent":
		var args struct {
			Task      string `json:"task"`
			ImportDir string `json:"import_dir"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		return a.spawnSubAgent(ctx, args.Task, args.ImportDir)
	case "web_search":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		return a.webSearch(ctx, args.Query)
	case "ask_user":
		var args struct {
			Question string `json:"question"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if a.AskUser == nil {
			return "", fmt.Errorf("ask_user unavailable in this interface")
		}
		return a.AskUser(args.Question)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// buildTools assembles the full tool list: built-in tools + skill definitions.
// list_ollama_models is only added when the active provider is ollama.
func (a *Agent) buildTools() []llm.Tool {
	tools := []llm.Tool{
		functionTool("shell_exec", "Run a sandbox command", map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}, "required": []string{"command"}}),
		functionTool("read_file", "Read a sandbox file", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}),
		functionTool("write_file", "Write a sandbox file", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"path", "content"}}),
		functionTool("dump_sandbox", "Dump sandbox to host path", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}),
		functionTool("spawn_subagent", "Spawn a subagent", map[string]any{"type": "object", "properties": map[string]any{"task": map[string]any{"type": "string"}, "import_dir": map[string]any{"type": "string"}}, "required": []string{"task"}}),
		functionTool("web_search", "Search the web for current or external information", map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}),
		functionTool("ask_user", "Ask the user a follow-up question and wait for their answer", map[string]any{"type": "object", "properties": map[string]any{"question": map[string]any{"type": "string"}}, "required": []string{"question"}}),
	}

	// Add Ollama model listing only when using the Ollama provider.
	if strings.ToLower(a.Cfg.Provider) == "ollama" {
		tools = append(tools, functionTool("list_ollama_models", "List all locally available Ollama models", map[string]any{"type": "object", "properties": map[string]any{}}))
	}

	// Add skill tools.
	for _, sk := range a.Skills {
		tools = append(tools, sk.Definition())
	}

	return tools
}

func (a *Agent) spawnSubAgent(ctx context.Context, task, importDir string) (string, error) {
	sb, err := sandbox.New(sandbox.Options{GoshellBin: a.Cfg.GoshellBin, Prompt: a.Cfg.SandboxPrompt, ImportPath: importDir})
	if err != nil {
		return "", err
	}
	defer sb.Close()
	agentID := runtime.Global().StartAgent("subagent", task, "", sb.ID)
	defer runtime.Global().FinishAgent(agentID)
	sub := New(a.Cfg, a.Memory, sb)
	res, _, err := sub.Run(ctx, task, nil, nil)
	return res, err
}

func (a *Agent) reviewAndUpdateMemory(task, final string, results []ToolResult, syncMemory func() error) {
	if a.Memory == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		note, save := a.memoryReview(ctx, task, final, results)
		if !save || strings.TrimSpace(note) == "" {
			return
		}
		_ = a.Memory.Append("memory", note)
		_ = a.Memory.Append("learnings", fmt.Sprintf("Task: %s\nResult: %s\nMemory: %s", task, final, note))
		if syncMemory != nil {
			_ = syncMemory()
		}
	}()
}

func (a *Agent) memoryReview(ctx context.Context, task, final string, results []ToolResult) (string, bool) {
	if a.MemoryClient == nil {
		return "", false
	}
	var toolNotes []string
	for _, r := range results {
		toolNotes = append(toolNotes, fmt.Sprintf("%s=%s", r.Name, strings.TrimSpace(r.Output)))
	}
	prompt, err := a.Prompts.Render("memory_review", map[string]string{
		"TASK":         task,
		"ANSWER":       final,
		"TOOL_OUTPUTS": strings.Join(toolNotes, " | "),
	})
	if err != nil {
		return "", false
	}
	msg, err := a.MemoryClient.Chat(ctx, []llm.Message{{Role: "system", Content: prompt}}, nil)
	if err != nil {
		return "", false
	}
	var parsed struct {
		Save bool   `json:"save"`
		Note string `json:"note"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &parsed); err != nil {
		return "", false
	}
	return strings.TrimSpace(parsed.Note), parsed.Save
}

func functionTool(name, desc string, params map[string]any) llm.Tool {
	var t llm.Tool
	t.Type = "function"
	t.Function.Name = name
	t.Function.Description = desc
	t.Function.Parameters = params
	return t
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func formatToolCall(name, argsJSON string) string {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" {
		return name + "()"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return fmt.Sprintf("%s(%s)", name, argsJSON)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatToolValue(m[k])))
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(parts, ", "))
}

func formatToolValue(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	text := string(data)
	if len(text) > 120 {
		return text[:117] + "..."
	}
	return text
}

func (a *Agent) webSearch(ctx context.Context, query string) (string, error) {
	if a.Search == nil {
		return "", fmt.Errorf("search client unavailable")
	}
	results, err := a.Search.Search(ctx, query, 5)
	if err != nil {
		return "", err
	}
	var out []string
	for i, r := range results {
		line := fmt.Sprintf("%d. %s\n%s\n%s", i+1, r.Title, r.URL, strings.TrimSpace(firstNonEmpty(r.Snippet, r.Text)))
		out = append(out, line)
	}
	if len(out) == 0 {
		return "no results", nil
	}
	return strings.Join(out, "\n\n"), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func renderHistory(history []llm.Message) string {
	if len(history) == 0 {
		return ""
	}
	var lines []string
	for _, msg := range history {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	return strings.Join(lines, "\n")
}

func RunTask(cfg config.Config, mem *memory.Store, task string, history []llm.Message, syncMemory func() error, askUser func(question string) (string, error)) (string, []ToolResult, error) {
	sb, err := sandbox.New(sandbox.Options{GoshellBin: cfg.GoshellBin, Prompt: cfg.SandboxPrompt})
	if err != nil {
		return "", nil, err
	}
	defer sb.Close()
	agentID := runtime.Global().StartAgent("main", task, "", sb.ID)
	defer runtime.Global().FinishAgent(agentID)
	agent := New(cfg, mem, sb)
	agent.AskUser = askUser
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return agent.Run(ctx, task, history, syncMemory)
}
