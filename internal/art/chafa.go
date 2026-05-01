package art

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

func (c *ChafaRenderer) Render(ctx context.Context, image []byte, width, height int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()

	out, err := c.runner.Run(ctx, image, width, height)
	if err != nil {
		return "", fmt.Errorf("art: chafa render failed: %w", err)
	}
	return string(out), nil
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
