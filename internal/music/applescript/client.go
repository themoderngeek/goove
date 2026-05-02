//go:build darwin

package applescript

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	if err == nil {
		return out, nil
	}
	if rErr, ok := err.(*runnerErr); ok {
		if bytes.Contains(rErr.stderr, []byte("-1743")) {
			return nil, fmt.Errorf("%w: %v", music.ErrPermission, err)
		}
	}
	return nil, fmt.Errorf("%w: %v", music.ErrUnavailable, err)
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

// artworkCachePath returns the fixed path where scriptArtwork writes the
// current track's image bytes. The directory is created on demand by Artwork().
func artworkCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Caches", "goove", "artwork.bin"), nil
}

// Artwork fetches the current track's embedded artwork bytes via AppleScript.
// Returns ErrNotRunning if Music isn't running, ErrNoArtwork if the track has
// no artwork, or wrapped ErrUnavailable / ErrPermission for other failures.
func (c *Client) Artwork(ctx context.Context) ([]byte, error) {
	cachePath, err := artworkCachePath()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", music.ErrUnavailable, err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return nil, fmt.Errorf("%w: %v", music.ErrUnavailable, err)
	}

	out, err := c.run(ctx, fmt.Sprintf(scriptArtwork, cachePath))
	if err != nil {
		return nil, err // already wrapped (ErrUnavailable or ErrPermission)
	}
	switch strings.TrimSpace(string(out)) {
	case "NOT_RUNNING":
		return nil, music.ErrNotRunning
	case "NO_ART":
		return nil, music.ErrNoArtwork
	case "OK":
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return nil, fmt.Errorf("%w: read cache: %v", music.ErrUnavailable, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("%w: unexpected scriptArtwork output: %q", music.ErrUnavailable, out)
	}
}

// AirPlayDevices lists all AirPlay output devices known to Music.app.
func (c *Client) AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error) {
	out, err := c.run(ctx, scriptAirPlayDevices)
	if err != nil {
		return nil, err
	}
	return parseAirPlayDevices(string(out))
}

// CurrentAirPlayDevice implements music.Client. Returns the device with
// Selected=true, or music.ErrDeviceNotFound if no device is selected.
func (c *Client) CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error) {
	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		return domain.AudioDevice{}, err
	}
	for _, d := range devices {
		if d.Selected {
			return d, nil
		}
	}
	return domain.AudioDevice{}, music.ErrDeviceNotFound
}

// SetAirPlayDevice implements music.Client. Resolves the user's name input
// against the AirPlay device list (exact match first, then case-insensitive
// substring), then runs scriptSetAirPlay with the matched device's exact name.
func (c *Client) SetAirPlayDevice(ctx context.Context, name string) error {
	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		return err
	}
	match, err := matchAirPlayDevice(devices, name)
	if err != nil {
		return err
	}
	out, err := c.run(ctx, fmt.Sprintf(scriptSetAirPlay, match.Name))
	if err != nil {
		return err
	}
	switch strings.TrimSpace(string(out)) {
	case "OK":
		return nil
	case "NOT_RUNNING":
		return music.ErrNotRunning
	case "NOT_FOUND":
		return music.ErrDeviceNotFound
	default:
		return fmt.Errorf("%w: unexpected scriptSetAirPlay output: %q", music.ErrUnavailable, out)
	}
}

// Compile-time check that *Client implements music.Client.
var _ music.Client = (*Client)(nil)
