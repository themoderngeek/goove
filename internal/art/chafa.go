package art

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const renderTimeout = 2 * time.Second

// ChafaRenderer implements Renderer by delegating to a ChafaRunner.
type ChafaRenderer struct {
	runner ChafaRunner
}

// New constructs a ChafaRenderer with the given runner. Used by tests
// that want to inject a fake; production code uses NewChafaRenderer().
func New(runner ChafaRunner) *ChafaRenderer {
	return &ChafaRenderer{runner: runner}
}

// cursorHide and cursorShow are the DEC private mode escapes chafa wraps
// its output with. They aren't standard ANSI style codes, so lipgloss
// width/height calculations don't ignore them: the hide escape on line 1
// inflates that line's measured width, and the show escape on a trailing
// line bumps the measured height by 1. Strip them so layout math is
// consistent with the visible art block.
const (
	cursorHide = "\x1b[?25l"
	cursorShow = "\x1b[?25h"
)

func (c *ChafaRenderer) Render(ctx context.Context, image []byte, width, height int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()

	out, err := c.runner.Run(ctx, image, width, height)
	if err != nil {
		return "", fmt.Errorf("art: chafa render failed: %w", err)
	}
	return stripCursorToggles(string(out)), nil
}

// stripCursorToggles removes \x1b[?25l and \x1b[?25h from chafa output and
// trims any trailing newlines that result. The hide-cursor escape always
// prefixes line 1; the show-cursor escape always trails on its own line.
// Removing them lets lipgloss measure the visible art block correctly.
func stripCursorToggles(s string) string {
	s = strings.ReplaceAll(s, cursorHide, "")
	s = strings.ReplaceAll(s, cursorShow, "")
	return strings.TrimRight(s, "\n")
}

// execChafaRunner is the real production runner. It pipes image bytes to
// chafa(1) via stdin and captures stdout. stderr is folded into the error
// on failure so logs show what chafa complained about.
type execChafaRunner struct{}

func (execChafaRunner) Run(ctx context.Context, image []byte, width, height int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "chafa",
		"--format=symbols",
		"--symbols=block",
		"--size", fmt.Sprintf("%dx%d", width, height),
		"-",
	)
	cmd.Stdin = bytes.NewReader(image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := bytes.TrimSpace(stderr.Bytes()); len(msg) > 0 {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

// NewChafaRenderer is the production constructor — it uses the real
// execChafaRunner. Use New() in tests with a fake runner.
func NewChafaRenderer() *ChafaRenderer {
	return New(execChafaRunner{})
}
