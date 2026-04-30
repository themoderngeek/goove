//go:build darwin

package applescript

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/themoderngeek/goove/internal/music"
)

const callTimeout = 2 * time.Second

type Client struct {
	runner Runner
}

func New(runner Runner) *Client {
	return &Client{runner: runner}
}

// NewDefault is a convenience constructor that uses the real osascript runner.
func NewDefault() *Client {
	return New(OsascriptRunner{})
}

func (c *Client) IsRunning(ctx context.Context) (bool, error) {
	out, err := c.run(ctx, scriptIsRunning)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func (c *Client) Launch(ctx context.Context) error {
	_, err := c.run(ctx, scriptLaunch)
	return err
}

// run wraps the runner with a per-call timeout and converts runner errors
// into music sentinel errors.
func (c *Client) run(ctx context.Context, script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	out, err := c.runner.Run(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", music.ErrUnavailable, err)
	}
	return out, nil
}
