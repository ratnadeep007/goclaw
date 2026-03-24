package session

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"ratnadeep007/goclaw/internal/config"
	"ratnadeep007/goclaw/internal/llm"
	"ratnadeep007/goclaw/internal/memory"
	"ratnadeep007/goclaw/internal/paths"
	"ratnadeep007/goclaw/internal/runtime"
)

type Store struct {
	db      *sql.DB
	dbPath  string
	baseDir string
}

type Session struct {
	ID        string
	Config    config.Config
	Memory    *memory.Store
	Messages  []llm.Message
	store     *Store
	memoryDir string
	created   time.Time
	updated   time.Time
}

type messageRow struct {
	Role    string
	Content string
}

func DefaultDBPath() string {
	return filepath.Join(paths.AppDataDir("goclaw"), "sessions.db")
}

func OpenDefault() (*Store, error) {
	return Open(DefaultDBPath())
}

func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, dbPath: dbPath, baseDir: filepath.Dir(dbPath)}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS session_memory (
			session_id TEXT NOT NULL,
			section TEXT NOT NULL,
			content TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (session_id, section)
		)`,
		`CREATE TABLE IF NOT EXISTS session_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func NewID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Store) OpenSession(ctx context.Context, id string, cfg config.Config) (*Session, error) {
	if strings.TrimSpace(id) == "" {
		id = NewID()
	}
	sess := &Session{ID: id, store: s}
	if err := s.loadOrCreateSession(ctx, sess, cfg); err != nil {
		return nil, err
	}
	if err := s.loadMemory(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.loadMessages(ctx, sess); err != nil {
		return nil, err
	}
	sess.Config = mergeConfig(sess.Config, cfg)
	if err := sess.SyncConfig(); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Store) loadOrCreateSession(ctx context.Context, sess *Session, cfg config.Config) error {
	var configJSON string
	var createdAt, updatedAt string
	row := s.db.QueryRowContext(ctx, `SELECT config_json, created_at, updated_at FROM sessions WHERE session_id = ?`, sess.ID)
	if err := row.Scan(&configJSON, &createdAt, &updatedAt); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return s.saveSessionRow(ctx, sess.ID, cfg)
	}
	if err := json.Unmarshal([]byte(configJSON), &sess.Config); err != nil {
		return err
	}
	sess.created = parseTime(createdAt)
	sess.updated = parseTime(updatedAt)
	return nil
}

func (s *Store) saveSessionRow(ctx context.Context, id string, cfg config.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(session_id, config_json, created_at, updated_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET config_json=excluded.config_json, updated_at=excluded.updated_at`, id, string(data), now, now)
	return err
}

func (s *Store) loadMemory(ctx context.Context, sess *Session) error {
	memDir := filepath.Join(paths.ConfigDir("goclaw"), "sessions", sess.ID, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return err
	}
	sess.memoryDir = memDir
	sess.Memory = memory.New(memDir)
	rows, err := s.db.QueryContext(ctx, `SELECT section, content FROM session_memory WHERE session_id = ?`, sess.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	files := map[string]string{}
	for rows.Next() {
		var section, content string
		if err := rows.Scan(&section, &content); err != nil {
			return err
		}
		files[section] = content
	}
	return sess.Memory.Restore(files)
}

func (s *Store) loadMessages(ctx context.Context, sess *Session) error {
	rows, err := s.db.QueryContext(ctx, `SELECT role, content FROM session_messages WHERE session_id = ? ORDER BY id ASC`, sess.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return err
		}
		sess.Messages = append(sess.Messages, llm.Message{Role: role, Content: content})
	}
	return nil
}

func (s *Session) SyncConfig() error {
	return s.store.saveSessionRow(context.Background(), s.ID, s.Config)
}

func (s *Session) SyncMemory() error {
	if s == nil || s.Memory == nil {
		return nil
	}
	snap, err := s.Memory.Snapshot()
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_memory WHERE session_id = ?`, s.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for section, content := range snap {
		if _, err := tx.ExecContext(ctx, `INSERT INTO session_memory(session_id, section, content, updated_at) VALUES(?, ?, ?, ?)`, s.ID, section, content, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Session) AppendMessage(role, content string) error {
	if s == nil {
		return nil
	}
	msg := llm.Message{Role: role, Content: content}
	s.Messages = append(s.Messages, msg)
	_, err := s.store.db.ExecContext(context.Background(), `INSERT INTO session_messages(session_id, role, content, created_at) VALUES(?, ?, ?, ?)`, s.ID, role, content, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Session) BriefMemory() (string, error) { return s.Memory.Brief() }

func (s *Session) ClearMemory() error {
	if s.Memory == nil {
		return nil
	}
	if err := s.Memory.Clear(); err != nil {
		return err
	}
	_, err := s.store.db.ExecContext(context.Background(), `DELETE FROM session_memory WHERE session_id = ?`, s.ID)
	return err
}

func (s *Session) SandboxSummary() string { return runtime.Global().Summary() }

func parseTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}

func mergeConfig(stored, current config.Config) config.Config {
	if current.OpenAIAPIKey != "" {
		stored.OpenAIAPIKey = current.OpenAIAPIKey
	}
	if current.OpenAIBaseURL != "" {
		stored.OpenAIBaseURL = current.OpenAIBaseURL
	}
	if current.OpenAIModel != "" {
		stored.OpenAIModel = current.OpenAIModel
	}
	if current.ExaAPIKey != "" {
		stored.ExaAPIKey = current.ExaAPIKey
	}
	if current.ExaBaseURL != "" {
		stored.ExaBaseURL = current.ExaBaseURL
	}
	if current.GoshellBin != "" {
		stored.GoshellBin = current.GoshellBin
	}
	if current.MemoryDir != "" {
		stored.MemoryDir = current.MemoryDir
	}
	if current.PromptDir != "" {
		stored.PromptDir = current.PromptDir
	}
	stored.ShowToolCalls = current.ShowToolCalls
	if current.MaxTurns != 0 {
		stored.MaxTurns = current.MaxTurns
	}
	if current.SandboxPrompt != "" {
		stored.SandboxPrompt = current.SandboxPrompt
	}
	return stored
}
