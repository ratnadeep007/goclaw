package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"ratnadeep007/goclaw/internal/agent"
	"ratnadeep007/goclaw/internal/config"
	"ratnadeep007/goclaw/internal/llm"
	"ratnadeep007/goclaw/internal/memory"
	"ratnadeep007/goclaw/internal/runtime"
	"ratnadeep007/goclaw/internal/session"
	"ratnadeep007/goclaw/internal/skills"
)

var (
	appStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("235"))
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("228")).
			Background(lipgloss.Color("24")).
			Padding(0, 1)
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	inputStyle     = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("238")).
			Padding(0, 1)
	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("223")).
			Background(lipgloss.Color("236")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
)

type slashCommand struct {
	Command string
	Sub     string
	Desc    string
}

var slashCommands = []slashCommand{
	{Command: "exit", Desc: "exit the TUI"},
	{Command: "memory", Sub: "brief", Desc: "show a brief memory summary"},
	{Command: "memory", Sub: "clear", Desc: "clear this session memory"},
	{Command: "agents", Sub: "list", Desc: "list subagents"},
	{Command: "sandbox", Sub: "list", Desc: "list goshell environments"},
	{Command: "provider", Desc: "show active provider and model"},
	{Command: "provider", Sub: "openai", Desc: "switch to OpenAI provider"},
	{Command: "provider", Sub: "ollama", Desc: "switch to Ollama provider"},
	{Command: "ollama", Sub: "models", Desc: "list available Ollama models"},
	{Command: "ollama", Sub: "models <name>", Desc: "switch active Ollama model"},
	{Command: "claude", Desc: "delegate a task to the Claude CLI (<task>)"},
	{Command: "opencode", Desc: "delegate a task to the OpenCode CLI (<task>)"},
}

type runDoneMsg struct {
	task   string
	output string
	tools  []agent.ToolResult
	err    error
}

type askUserMsg struct {
	question string
	replyCh  chan string
}

type model struct {
	sess     *session.Session
	cfg      config.Config
	mem      *memory.Store
	input    textinput.Model
	vp       viewport.Model
	spin     spinner.Model
	messages []string
	hints    []string
	running  bool
	waiting  bool
	replyCh  chan string
	w        int
	h        int
	agentCh  chan tea.Msg
}

func New(sess *session.Session) tea.Model {
	cfg := sess.Config

	// Apply any provider overrides stored in settings, but only when the
	// corresponding env var was NOT explicitly set (env vars always win).
	if os.Getenv("LLM_PROVIDER") == "" {
		if p, ok := sess.GetSetting("provider"); ok {
			cfg.Provider = p
		}
	}
	if os.Getenv("OLLAMA_MODEL") == "" {
		if m, ok := sess.GetSetting("ollama_model"); ok {
			cfg.OllamaModel = m
		}
	}
	if os.Getenv("OPENAI_MODEL") == "" {
		if m, ok := sess.GetSetting("openai_model"); ok {
			cfg.OpenAIModel = m
		}
	}

	mem := sess.Memory
	input := textinput.New()
	input.Placeholder = "Ask goclaw to do something..."
	input.Prompt = "┃ "
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	input.Focus()
	input.CharLimit = 4000
	input.Width = 80

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to goclaw\n")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	msgs := []string{"Welcome to goclaw"}
	for _, msg := range sess.Messages {
		msgs = append(msgs, renderMessage(msg.Role, msg.Content, 80))
	}

	return model{sess: sess, cfg: cfg, mem: mem, input: input, vp: vp, spin: sp, messages: msgs, agentCh: make(chan tea.Msg)}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.ClearScreen, textinput.Blink, waitForAgentMsg(m.agentCh))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.vp.Width = msg.Width - 4
		m.vp.Height = msg.Height - 7
		m.input.Width = msg.Width - 4
		m.refreshHints()
		m.rebuild()
	case runDoneMsg:
		m.running = false
		m.waiting = false
		m.replyCh = nil
		if msg.err != nil {
			m.messages = append(m.messages, errorStyle.Render("Error: "+msg.err.Error()))
		} else {
			if m.cfg.ShowToolCalls {
				for _, tool := range msg.tools {
					call := strings.TrimSpace(tool.Call)
					if call == "" {
						call = tool.Name + "()"
					}
					m.messages = append(m.messages, toolStyle.Render("Tool: "+call))
					_ = m.sess.AppendMessage("tool", call)
				}
			}
			m.messages = append(m.messages, renderAssistantMessage(strings.TrimSpace(msg.output), m.vp.Width))
			_ = m.sess.AppendMessage("assistant", strings.TrimSpace(msg.output))
			_ = m.sess.SyncMemory()
		}
		m.rebuild()
		cmds = append(cmds, waitForAgentMsg(m.agentCh))
	case askUserMsg:
		m.waiting = true
		m.running = true
		m.replyCh = msg.replyCh
		m.messages = append(m.messages, renderAssistantMessage(strings.TrimSpace(msg.question), m.vp.Width))
		m.rebuild()
		cmds = append(cmds, waitForAgentMsg(m.agentCh))
	case tea.KeyMsg:
		if m.running && !m.waiting {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			task := strings.TrimSpace(m.input.Value())
			if task == "" {
				return m, nil
			}
			if m.waiting && m.replyCh != nil {
				m.messages = append(m.messages, userStyle.Render("You: "+task))
				_ = m.sess.AppendMessage("user", task)
				m.input.SetValue("")
				m.waiting = false
				m.rebuild()
				m.replyCh <- task
				return m, nil
			}
			if strings.HasPrefix(task, "/") {
				return m.handleSlash(task)
			}
			history := append([]llm.Message(nil), m.sess.Messages...)
			_ = m.sess.AppendMessage("user", task)
			m.messages = append(m.messages, userStyle.Render("You: "+task))
			m.input.SetValue("")
			m.refreshHints()
			m.running = true
			m.waiting = false
			m.rebuild()
			cmds = append(cmds, m.spin.Tick, startTask(m.agentCh, m.sess, m.cfg, task, history))
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.refreshHints()
	m.vp, cmd = m.vp.Update(msg)
	cmds = append(cmds, cmd)
	m.spin, cmd = m.spin.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) rebuild() {
	body := strings.Join(m.messages, "\n\n")
	if m.running {
		body += "\n\n" + m.spin.View() + " Working..."
	}
	m.vp.SetContent(body)
	m.vp.GotoBottom()
}

func (m model) View() string {
	title := headerStyle.Render("goclaw")
	status := "idle"
	if m.running {
		status = m.spin.View() + " running"
	}
	if m.waiting {
		status = "waiting for input"
	}
	activeModel := m.cfg.ActiveModel("agent")
	header := headerStyle.Render(fmt.Sprintf("%s | %s | provider: %s | model: %s | session: %s",
		title, status, m.cfg.Provider, activeModel, m.sess.ID))
	content := borderStyle.Width(m.vp.Width).Render(m.vp.View())
	input := inputStyle.Width(m.input.Width + 2).Render(m.input.View())
	out := header + "\n" + content + "\n" + input
	if hint := m.commandHintsView(); hint != "" {
		out += "\n" + hint
	}
	return appStyle.Render(out)
}

func (m model) handleSlash(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	name := strings.TrimPrefix(parts[0], "/")
	if name == "clear" && len(parts) > 1 && parts[1] == "memory" {
		name = "memory"
		parts = []string{"/memory", "clear"}
	}
	m.input.SetValue("")

	switch name {
	case "exit", "quit":
		return m, tea.Quit

	case "memory":
		sub := "brief"
		if len(parts) > 1 {
			sub = parts[1]
		}
		switch sub {
		case "brief":
			brief, err := m.sess.BriefMemory()
			if err != nil {
				m.messages = append(m.messages, errorStyle.Render("Memory: "+err.Error()))
			} else {
				m.messages = append(m.messages, renderAssistantMessage("Memory:\n"+brief, m.vp.Width))
			}
		case "clear":
			if err := m.sess.ClearMemory(); err != nil {
				m.messages = append(m.messages, errorStyle.Render("Clear memory: "+err.Error()))
			} else {
				_ = m.sess.SyncMemory()
				m.messages = append(m.messages, renderAssistantMessage("Memory cleared", m.vp.Width))
			}
		default:
			m.messages = append(m.messages, errorStyle.Render("Unknown memory sub-command: "+sub))
		}

	case "agents":
		sub := "list"
		if len(parts) > 1 {
			sub = parts[1]
		}
		if sub != "list" {
			m.messages = append(m.messages, errorStyle.Render("Unknown agents sub-command: "+sub))
			break
		}
		items := runtime.Global().ListAgents()
		if len(items) == 0 {
			m.messages = append(m.messages, renderAssistantMessage("Agents: none", m.vp.Width))
			break
		}
		var lines []string
		for _, it := range items {
			lines = append(lines, fmt.Sprintf("%s [%s] sandbox=%s task=%s", it.ID, it.Status, it.SandboxID, strings.TrimSpace(it.Task)))
		}
		m.messages = append(m.messages, renderAssistantMessage("Agents:\n"+strings.Join(lines, "\n"), m.vp.Width))

	case "sandbox", "sandboxes":
		sub := "list"
		if len(parts) > 1 {
			sub = parts[1]
		}
		if sub != "list" {
			m.messages = append(m.messages, errorStyle.Render("Unknown sandbox sub-command: "+sub))
			break
		}
		items := runtime.Global().ListSandboxes()
		if len(items) == 0 {
			m.messages = append(m.messages, renderAssistantMessage("Sandboxes: none", m.vp.Width))
			break
		}
		var lines []string
		for _, it := range items {
			lines = append(lines, fmt.Sprintf("%s [%s] kind=%s label=%s", it.ID, it.Status, it.Kind, strings.TrimSpace(it.Label)))
		}
		m.messages = append(m.messages, renderAssistantMessage("Sandboxes:\n"+strings.Join(lines, "\n"), m.vp.Width))

	case "provider":
		if len(parts) == 1 {
			info := fmt.Sprintf("**Provider:** %s\n**Model (agent):** %s\n**Model (router):** %s\n**Model (memory):** %s",
				m.cfg.Provider,
				m.cfg.ActiveModel("agent"),
				m.cfg.ActiveModel("router"),
				m.cfg.ActiveModel("memory"),
			)
			m.messages = append(m.messages, renderAssistantMessage(info, m.vp.Width))
			break
		}
		sub := strings.ToLower(parts[1])
		switch sub {
		case "openai", "ollama":
			m.cfg.Provider = sub
			m.sess.Config.Provider = sub
			_ = m.sess.SyncConfig()
			_ = m.sess.SetSetting("provider", sub)
			m.messages = append(m.messages, renderAssistantMessage(
				fmt.Sprintf("Switched provider to **%s** (agent model: %s)", sub, m.cfg.ActiveModel("agent")),
				m.vp.Width,
			))
		default:
			m.messages = append(m.messages, errorStyle.Render(
				fmt.Sprintf("Unknown provider: %s — supported: openai, ollama", sub),
			))
		}

	case "ollama":
		if len(parts) < 2 || strings.ToLower(parts[1]) != "models" {
			m.messages = append(m.messages, errorStyle.Render("Usage: /ollama models [<model-name>]"))
			break
		}
		if len(parts) == 2 {
			// List available models.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			models, err := llm.ListModels(ctx, m.cfg.OllamaBaseURL)
			if err != nil {
				m.messages = append(m.messages, errorStyle.Render("Ollama models: "+err.Error()))
				break
			}
			if len(models) == 0 {
				m.messages = append(m.messages, renderAssistantMessage("No local Ollama models found. Try `ollama pull llama3` first.", m.vp.Width))
				break
			}
			m.messages = append(m.messages, renderAssistantMessage("**Ollama models:**\n- "+strings.Join(models, "\n- "), m.vp.Width))
		} else {
			// Switch active Ollama model.
			newModel := parts[2]
			m.cfg.OllamaModel = newModel
			m.sess.Config.OllamaModel = newModel
			_ = m.sess.SyncConfig()
			_ = m.sess.SetSetting("ollama_model", newModel)
			m.messages = append(m.messages, renderAssistantMessage(
				fmt.Sprintf("Ollama model switched to **%s**", newModel), m.vp.Width,
			))
		}

	case "claude":
		task := strings.TrimSpace(strings.TrimPrefix(cmd, "/claude"))
		if task == "" {
			m.messages = append(m.messages, errorStyle.Render("Usage: /claude <task>"))
			break
		}
		sk := skills.NewClaude(m.cfg.SkillTimeoutSecs)
		if !sk.Available() {
			m.messages = append(m.messages, errorStyle.Render("claude binary not found on PATH — install the Claude CLI from https://claude.ai/cli"))
			break
		}
		m.running = true
		m.rebuild()
		return m, skillTask(m.agentCh, sk, task, m.cfg.SkillTimeoutSecs)

	case "opencode":
		task := strings.TrimSpace(strings.TrimPrefix(cmd, "/opencode"))
		if task == "" {
			m.messages = append(m.messages, errorStyle.Render("Usage: /opencode <task>"))
			break
		}
		sk := skills.NewOpenCode(m.cfg.SkillTimeoutSecs)
		if !sk.Available() {
			m.messages = append(m.messages, errorStyle.Render("opencode binary not found on PATH — install opencode (https://opencode.ai)"))
			break
		}
		m.running = true
		m.rebuild()
		return m, skillTask(m.agentCh, sk, task, m.cfg.SkillTimeoutSecs)

	default:
		m.messages = append(m.messages, errorStyle.Render("Unknown command: "+cmd))
	}

	m.rebuild()
	m.refreshHints()
	return m, nil
}

func (m *model) refreshHints() {
	value := strings.TrimSpace(m.input.Value())
	if !strings.HasPrefix(value, "/") {
		m.hints = nil
		return
	}
	query := strings.ToLower(strings.TrimPrefix(value, "/"))
	var matched []string
	var rest []string
	for _, c := range slashCommands {
		n := "/" + c.Command
		if c.Sub != "" {
			n += " " + c.Sub
		}
		line := fmt.Sprintf("%s — %s", n, c.Desc)
		if query == "" || strings.Contains(strings.ToLower(strings.TrimPrefix(n, "/")), query) || strings.Contains(strings.ToLower(c.Desc), query) {
			matched = append(matched, line)
		} else {
			rest = append(rest, line)
		}
	}
	m.hints = append(matched, rest...)
}

func (m model) commandHintsView() string {
	if len(m.hints) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(commandStyle.Render("commands"))
	b.WriteString("\n")
	b.WriteString(commandStyle.Render(strings.Join(m.hints, "\n")))
	return b.String()
}

// startTask dispatches a full agent task in a goroutine. It accepts the TUI's
// live cfg so that provider/model changes via /provider take effect immediately.
func startTask(ch chan tea.Msg, sess *session.Session, cfg config.Config, task string, history []llm.Message) tea.Cmd {
	return func() tea.Msg {
		go func() {
			askUser := func(question string) (string, error) {
				replyCh := make(chan string)
				ch <- askUserMsg{question: question, replyCh: replyCh}
				answer := <-replyCh
				return answer, nil
			}
			out, tools, err := agent.RunTask(cfg, sess.Memory, task, history, sess.SyncMemory, askUser)
			ch <- runDoneMsg{task: task, output: out, tools: tools, err: err}
		}()
		return nil
	}
}

// skillTask dispatches a single skill invocation in a goroutine and sends the
// result through ch as a runDoneMsg.
func skillTask(ch chan tea.Msg, sk skills.Skill, task string, timeoutSecs int) tea.Cmd {
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return func() tea.Msg {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
			defer cancel()
			out, err := sk.Run(ctx, map[string]any{"task": task})
			ch <- runDoneMsg{task: sk.Name() + ": " + task, output: out, err: err}
		}()
		return nil
	}
}

func waitForAgentMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func renderMessage(role, content string, width int) string {
	switch role {
	case "user":
		return userStyle.Render("You: " + content)
	case "assistant":
		return renderAssistantMessage(content, width)
	case "tool":
		return toolStyle.Render("Tool: " + content)
	default:
		return content
	}
}

func renderAssistantMessage(content string, width int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return assistantStyle.Render("Assistant:")
	}
	wrap := width - 6
	if wrap < 40 {
		wrap = 80
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrap),
	)
	if err != nil {
		return assistantStyle.Render("Assistant: " + content)
	}
	out, err := renderer.Render(content)
	if err != nil {
		return assistantStyle.Render("Assistant: " + content)
	}
	return assistantStyle.Render("Assistant:") + "\n" + strings.TrimRight(out, "\n")
}
