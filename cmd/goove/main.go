//go:build darwin

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/app"
	"github.com/themoderngeek/goove/internal/music/applescript"
)

func main() {
	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not initialise log file: %v\n", err)
	}
	slog.Info("goove starting")

	client := applescript.NewDefault()
	model := app.New(client)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "goove: %v\n", err)
		os.Exit(1)
	}
}

func setupLogging() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "Logs", "goove")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "goove.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if os.Getenv("GOOVE_LOG") == "debug" {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
	return nil
}
