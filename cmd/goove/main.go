//go:build darwin

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/app"
	"github.com/themoderngeek/goove/internal/art"
	"github.com/themoderngeek/goove/internal/cli"
	"github.com/themoderngeek/goove/internal/music/applescript"
)

func main() {
	if len(os.Args) > 1 && isCLIMode(os.Args[1]) {
		client := applescript.NewDefault()
		os.Exit(cli.Run(os.Args[1:], client, os.Stdout, os.Stderr))
	}

	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not initialise log file: %v\n", err)
	}
	slog.Info("goove starting")

	client := applescript.NewDefault()
	var renderer art.Renderer
	if art.Available() {
		renderer = art.NewChafaRenderer()
	} else {
		slog.Info("chafa not found in PATH; album art disabled (install with: brew install chafa)")
	}
	model := app.New(client, renderer)

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

// isCLIMode reports whether the first non-program arg should route to CLI mode.
// Help flags trigger CLI mode (so `goove --help` doesn't launch the TUI).
// Any first arg that doesn't start with a dash is treated as a subcommand.
func isCLIMode(firstArg string) bool {
	if firstArg == "-h" || firstArg == "--help" {
		return true
	}
	return !strings.HasPrefix(firstArg, "-")
}
