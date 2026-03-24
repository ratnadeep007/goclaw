package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"

	"ratnadeep007/goclaw/internal/config"
	"ratnadeep007/goclaw/internal/server"
	"ratnadeep007/goclaw/internal/session"
	"ratnadeep007/goclaw/internal/tui"
)

func main() {
	loadEnvFile()
	var httpMode bool
	var addr string
	var sessionID string
	flag.BoolVar(&httpMode, "http", false, "run HTTP server")
	flag.StringVar(&addr, "addr", ":8080", "HTTP listen address")
	flag.StringVar(&sessionID, "s", "", "session id")
	flag.Parse()

	cfg := config.Load()
	store, err := session.OpenDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	sess, err := store.OpenSession(context.Background(), sessionID, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if httpMode {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		s := server.NewSession(sess)
		if err := s.Serve(ctx, addr); err != nil && err != context.Canceled {
			log.Fatal(err)
		}
		return
	}

	p := tea.NewProgram(tui.New(sess))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadEnvFile() {
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, ".env"))
		candidates = append(candidates, filepath.Join(filepath.Dir(dir), ".env"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append([]string{filepath.Join(wd, ".env")}, candidates...)
	}
	for _, path := range candidates {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}
