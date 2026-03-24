package sandbox

import (
	"fmt"
	"os"
	"sync"

	goshell "github.com/ratnadeepbhattacharyya/goshell/pkg/goshell"

	"ratnadeep007/goclaw/internal/runtime"
)

type Sandbox struct {
	mu     sync.Mutex
	ID     string
	Kind   string
	Label  string
	inner  *goshell.Sandbox
	closed bool
}

type Options struct {
	GoshellBin    string
	Prompt        string
	ImportPath    string
	AllowedHosts  []string
	AllowedExtra  []string
	ExternalTools []string
}

func New(opts Options) (*Sandbox, error) {
	inner := goshell.New()
	if opts.ImportPath != "" {
		if err := inner.Import(opts.ImportPath); err != nil {
			inner.Close()
			return nil, err
		}
	}
	id := runtime.Global().StartSandbox("goshell", opts.ImportPath)
	return &Sandbox{ID: id, Kind: "goshell", Label: opts.ImportPath, inner: inner}, nil
}

func (s *Sandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	runtime.Global().FinishSandbox(s.ID)
	if s.inner != nil {
		s.inner.Close()
	}
	return nil
}

func (s *Sandbox) Exec(command string) (string, error) {
	if s == nil || s.inner == nil {
		return "", fmt.Errorf("sandbox unavailable")
	}
	return s.inner.Exec(command)
}

func (s *Sandbox) Dump(hostPath string) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("sandbox unavailable")
	}
	return s.inner.Dump(hostPath)
}

func (s *Sandbox) Reset() {
	if s != nil && s.inner != nil {
		s.inner.Reset()
	}
}

func (s *Sandbox) ReadFile(path string) (string, error) {
	if s == nil || s.inner == nil {
		return "", fmt.Errorf("sandbox unavailable")
	}
	return s.inner.ReadFile(path)
}

func (s *Sandbox) WriteFile(path, content string) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("sandbox unavailable")
	}
	return s.inner.WriteFile(path, content)
}

func (s *Sandbox) Env() map[string]string {
	if s == nil || s.inner == nil {
		return map[string]string{}
	}
	return s.inner.Env()
}

func (s *Sandbox) SetEnv(key, value string) {
	if s != nil && s.inner != nil {
		s.inner.SetEnv(key, value)
	}
}

func SnapshotImportPath(hostPath string) (string, error) {
	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return "", err
	}
	return hostPath, nil
}
