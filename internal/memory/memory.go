package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	Dir string
	mu  sync.Mutex
}

func New(dir string) *Store { return &Store{Dir: dir} }

func (s *Store) ensure() error {
	return os.MkdirAll(s.Dir, 0o755)
}

func (s *Store) path(name string) string {
	return filepath.Join(s.Dir, name+".md")
}

func (s *Store) LoadContext() (string, error) {
	if err := s.ensure(); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	var b strings.Builder
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			continue
		}
		b.WriteString("## ")
		b.WriteString(strings.TrimSuffix(name, ".md"))
		b.WriteString("\n")
		b.Write(data)
		if !strings.HasSuffix(string(data), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func (s *Store) Append(section, content string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.path(section)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## %s\n%s\n", time.Now().Format(time.RFC3339), strings.TrimSpace(content))
	return err
}

func (s *Store) Replace(section, content string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.path(section), []byte(strings.TrimSpace(content)+"\n"), 0o644)
}

func (s *Store) Snapshot() (map[string]string, error) {
	if err := s.ensure(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, entry.Name()))
		if err != nil {
			continue
		}
		out[strings.TrimSuffix(entry.Name(), ".md")] = string(data)
	}
	return out, nil
}

func (s *Store) Restore(files map[string]string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		_ = os.Remove(filepath.Join(s.Dir, entry.Name()))
	}
	for name, content := range files {
		if err := os.WriteFile(s.path(name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Clear() error {
	return s.Restore(map[string]string{})
}

func (s *Store) Brief() (string, error) {
	if err := s.ensure(); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(s.path(name))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		preview := strings.Join(lines[len(lines)-min(4, len(lines)):], "\n")
		if preview == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(strings.ReplaceAll(preview, "\n", " | "))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "no memory yet", nil
	}
	return b.String(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
