//go:build darwin && integration

package applescript

import (
	stdbytes "bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
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

func TestIntegrationArtwork(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Skip("Music.app is not running; cannot exercise Artwork")
	}

	// Verify a track is loaded; otherwise skip — Artwork has nothing to fetch.
	if _, err := c.Status(ctx); err != nil {
		t.Skipf("Status returned %v; Artwork test needs a loaded track", err)
	}

	bytes, err := c.Artwork(ctx)
	if err != nil {
		// ErrNoArtwork is acceptable — current track may be a stream without art.
		t.Logf("Artwork returned %v (acceptable if track has no artwork)", err)
		return
	}
	if len(bytes) == 0 {
		t.Fatal("Artwork returned zero bytes with no error")
	}

	// Decode just the header to confirm it's a real image.
	cfg, format, err := image.DecodeConfig(stdbytes.NewReader(bytes))
	if err != nil {
		t.Fatalf("artwork bytes are not a decodable image: %v", err)
	}
	t.Logf("Artwork: %d bytes, format=%s, %dx%d", len(bytes), format, cfg.Width, cfg.Height)
}

func TestIntegrationAirPlayDevicesRoundtrip(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Skip("Music.app is not running; cannot exercise AirPlayDevices")
	}

	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		t.Fatalf("AirPlayDevices err = %v", err)
	}
	t.Logf("Music reports %d AirPlay device(s):", len(devices))
	for _, d := range devices {
		t.Logf("  - %s (kind=%s available=%v active=%v selected=%v)",
			d.Name, d.Kind, d.Available, d.Active, d.Selected)
	}

	current, err := c.CurrentAirPlayDevice(ctx)
	if err != nil {
		t.Logf("CurrentAirPlayDevice returned %v (acceptable if no device selected)", err)
	} else {
		t.Logf("Currently selected: %s", current.Name)
	}

	// Read-only by design — this test does NOT call SetAirPlayDevice
	// because that would disrupt the user's actual audio routing.
}

func TestIntegrationPlaylistsListsAtLeastZero(t *testing.T) {
	c := NewDefault()
	ctx := context.Background()

	if err := c.Launch(ctx); err != nil {
		t.Fatalf("Launch err = %v", err)
	}

	got, err := c.Playlists(ctx)
	if err != nil {
		t.Fatalf("Playlists err = %v", err)
	}
	// We don't assert a specific count — the user's library is unknown.
	// Assert only that we got a slice (possibly empty) and that any returned
	// rows have plausible Kind values.
	for _, p := range got {
		if p.Kind != "user" && p.Kind != "subscription" {
			t.Errorf("playlist %q has unexpected kind %q", p.Name, p.Kind)
		}
	}
	t.Logf("found %d playlists", len(got))
}

// TestIntegrationSearchTracks_Smoke runs against the real Music.app library.
// It does not assert on which tracks come back — only that the call succeeds,
// returns a non-negative total, and that any returned track has a non-empty
// persistent ID.
func TestIntegrationSearchTracks_Smoke(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Skip("Music.app is not running; cannot exercise SearchTracks")
	}

	got, err := c.SearchTracks(ctx, "a")
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
	}
	if got.Total < 0 {
		t.Errorf("Total negative: %d", got.Total)
	}
	for _, tr := range got.Tracks {
		if tr.PersistentID == "" {
			t.Errorf("track without PersistentID: %+v", tr)
		}
	}
	t.Logf("SearchTracks(\"a\"): total=%d returned=%d", got.Total, len(got.Tracks))

	// PlayTrack is intentionally not exercised here — calling it in an
	// integration test would interrupt the user's active playback.
}
