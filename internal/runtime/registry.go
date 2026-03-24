package runtime

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type SandboxInfo struct {
	ID        string
	Kind      string
	Label     string
	Status    string
	CreatedAt time.Time
	ClosedAt  time.Time
}

type AgentInfo struct {
	ID        string
	ParentID  string
	SandboxID string
	Role      string
	Task      string
	Status    string
	CreatedAt time.Time
	ClosedAt  time.Time
}

type Registry struct {
	mu        sync.Mutex
	seq       atomic.Uint64
	sandboxes map[string]*SandboxInfo
	agents    map[string]*AgentInfo
}

var global = NewRegistry()

func Global() *Registry { return global }

func NewRegistry() *Registry {
	return &Registry{sandboxes: make(map[string]*SandboxInfo), agents: make(map[string]*AgentInfo)}
}

func (r *Registry) nextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, r.seq.Add(1))
}

func (r *Registry) StartSandbox(kind, label string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID("sb")
	r.sandboxes[id] = &SandboxInfo{ID: id, Kind: kind, Label: label, Status: "active", CreatedAt: time.Now()}
	return id
}

func (r *Registry) FinishSandbox(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if sb, ok := r.sandboxes[id]; ok {
		sb.Status = "closed"
		sb.ClosedAt = time.Now()
	}
}

func (r *Registry) StartAgent(role, task, parentID, sandboxID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID("ag")
	r.agents[id] = &AgentInfo{ID: id, ParentID: parentID, SandboxID: sandboxID, Role: role, Task: task, Status: "active", CreatedAt: time.Now()}
	return id
}

func (r *Registry) FinishAgent(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ag, ok := r.agents[id]; ok {
		ag.Status = "done"
		ag.ClosedAt = time.Now()
	}
}

func (r *Registry) ListSandboxes() []SandboxInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SandboxInfo, 0, len(r.sandboxes))
	for _, v := range r.sandboxes {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (r *Registry) ListAgents() []AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AgentInfo, 0, len(r.agents))
	for _, v := range r.agents {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (r *Registry) Summary() string {
	agents := r.ListAgents()
	sandboxes := r.ListSandboxes()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("agents: %d, sandboxes: %d", len(agents), len(sandboxes)))
	return b.String()
}
