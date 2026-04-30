//go:build darwin

package applescript

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/themoderngeek/goove/internal/domain"
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

func (c *Client) Status(ctx context.Context) (domain.NowPlaying, error) {
	out, err := c.run(ctx, scriptStatus)
	if err != nil {
		return domain.NowPlaying{}, err
	}
	np, err := parseStatus(string(out))
	if err != nil {
		return domain.NowPlaying{}, err
	}
	np.LastSyncedAt = time.Now()
	return np, nil
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

func (c *Client) PlayPause(ctx context.Context) error {
	_, err := c.run(ctx, scriptPlayPause)
	return err
}

func (c *Client) Next(ctx context.Context) error {
	_, err := c.run(ctx, scriptNext)
	return err
}

func (c *Client) Prev(ctx context.Context) error {
	_, err := c.run(ctx, scriptPrev)
	return err
}

func (c *Client) SetVolume(ctx context.Context, percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	_, err := c.run(ctx, fmt.Sprintf(scriptSetVolume, percent))
	return err
}

// Compile-time check that *Client implements music.Client.
var _ music.Client = (*Client)(nil)
