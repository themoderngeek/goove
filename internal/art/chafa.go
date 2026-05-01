package art

import (
	"context"
	"fmt"
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
