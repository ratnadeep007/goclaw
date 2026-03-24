package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed defaults/*.md
var defaultFiles embed.FS

type Manager struct {
	Dir string
}

func New(dir string) *Manager {
	return &Manager{Dir: dir}
}

func (m *Manager) EnsureDefaults() error {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return err
	}
	entries, err := defaultFiles.ReadDir("defaults")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		target := filepath.Join(m.Dir, entry.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		}
		data, err := defaultFiles.ReadFile(filepath.Join("defaults", entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Load(name string) (string, error) {
	if err := m.EnsureDefaults(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(m.Dir, name+".md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Manager) Render(name string, vars map[string]string) (string, error) {
	text, err := m.Load(name)
	if err != nil {
		return "", err
	}
	shared, err := m.Load("shared_rules")
	if err == nil && strings.TrimSpace(shared) != "" {
		text = text + "\n\n## Shared Rules\n\n" + shared
	}
	for key, value := range vars {
		text = strings.ReplaceAll(text, "{{"+key+"}}", value)
	}
	return text, nil
}

func (m *Manager) Summary() (string, error) {
	if err := m.EnsureDefaults(); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return fmt.Sprintf("prompt dir: %s\nfiles: %s", m.Dir, strings.Join(names, ", ")), nil
}
