//go:build darwin && integration

package applescript

import (
	"context"
	"testing"
	"time"
)

// These tests touch the real Music.app on the local machine.
// Run with:  go test -tags=integration ./internal/music/applescript/...
//
// Prerequisites:
//   - macOS with Music.app installed.
//   - Automation permission granted to the terminal binary you run `go test` from
//     (System Settings → Privacy & Security → Automation → <terminal> → Music).
//
// The tests are read-only by design: they only call IsRunning and Status.
// They do not press play, change volume, or skip tracks.

func TestIntegrationIsRunning(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	t.Logf("Music.app running = %v", running)
}

func TestIntegrationStatus(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Skip("Music.app is not running; cannot exercise Status")
	}

	np, err := c.Status(ctx)
	if err != nil {
		// ErrNoTrack is acceptable — Music is open with nothing loaded.
		t.Logf("Status returned %v (acceptable if no track loaded)", err)
		return
	}
	t.Logf("Now playing: %q by %q on %q (%v / %v)",
		np.Track.Title, np.Track.Artist, np.Track.Album, np.Position, np.Duration)
}
