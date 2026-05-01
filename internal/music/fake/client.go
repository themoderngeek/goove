package fake

import (
	"context"
	"sync"
	"time"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// Client is an in-memory implementation of music.Client used in tests.
// Tests script its state via Launch / SetTrack / SimulateError.
type Client struct {
	mu        sync.Mutex
	running   bool
	hasTrack  bool
	track     domain.Track
	duration  time.Duration
	position  time.Duration
	playing   bool
	volume    int
	forcedErr error

	// Counters useful for assertions.
	PlayPauseCalls int
	NextCalls      int
	PrevCalls      int
	SetVolumeCalls int
	LaunchCalls    int
}

func New() *Client {
	return &Client{volume: 50}
}

func (c *Client) SetTrack(t domain.Track, durationSec, positionSec int, playing bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hasTrack = true
	c.track = t
	c.duration = time.Duration(durationSec) * time.Second
	c.position = time.Duration(positionSec) * time.Second
	c.playing = playing
}

func (c *Client) SimulateError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forcedErr = err
}

func (c *Client) IsRunning(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return false, c.forcedErr
	}
	return c.running, nil
}

func (c *Client) Launch(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.LaunchCalls++
	c.running = true
	return nil
}

func (c *Client) Status(ctx context.Context) (domain.NowPlaying, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return domain.NowPlaying{}, c.forcedErr
	}
	if !c.running {
		return domain.NowPlaying{}, music.ErrNotRunning
	}
	if !c.hasTrack {
		return domain.NowPlaying{}, music.ErrNoTrack
	}
	return domain.NowPlaying{
		Track:        c.track,
		Position:     c.position,
		Duration:     c.duration,
		IsPlaying:    c.playing,
		Volume:       c.volume,
		LastSyncedAt: time.Now(),
	}, nil
}

func (c *Client) PlayPause(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.PlayPauseCalls++
	c.playing = !c.playing
	return nil
}

func (c *Client) Next(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.NextCalls++
	return nil
}

func (c *Client) Prev(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.PrevCalls++
	return nil
}

func (c *Client) SetVolume(ctx context.Context, percent int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.SetVolumeCalls++
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	c.volume = percent
	return nil
}

var _ music.Client = (*Client)(nil)
