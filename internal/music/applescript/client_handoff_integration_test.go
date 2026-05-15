//go:build darwin && integration_handoff

package applescript

import (
	"context"
	"testing"
	"time"
)

// These tests MUTATE Music.app state — they start playback, switch
// tracks, and pause. Kept in a separate file with a separate build tag
// (integration_handoff) so the default integration run stays read-only.
//
// Run with:
//   go test -tags=integration_handoff ./internal/music/applescript/
//
// Prerequisites:
//   - macOS with Music.app installed.
//   - Automation permission granted to the terminal binary you run
//     `go test` from (System Settings → Privacy & Security → Automation).
//   - A playlist named "Liked Songs" with at least 3 tracks.

func TestIntegrationQueueHandoffOverridesPlaylistNaturalNext(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	c := NewDefault()

	if err := c.Launch(ctx); err != nil {
		t.Skipf("can't launch Music.app: %v", err)
	}

	// Pick a known playlist. Liked Songs is the conventional default in
	// goove's docs. Bail out clean if absent.
	playlists, err := c.Playlists(ctx)
	if err != nil {
		t.Fatalf("Playlists: %v", err)
	}
	const playlistName = "Liked Songs"
	found := false
	for _, p := range playlists {
		if p.Name == playlistName {
			found = true
			break
		}
	}
	if !found {
		t.Skipf("playlist %q not present in this library", playlistName)
	}

	tracks, err := c.PlaylistTracks(ctx, playlistName)
	if err != nil {
		t.Fatalf("PlaylistTracks: %v", err)
	}
	if len(tracks) < 3 {
		t.Skipf("playlist %q needs >= 3 tracks; has %d", playlistName, len(tracks))
	}

	// Play from track 1. We'll queue tracks[2] so it doesn't accidentally
	// match the natural-next (tracks[1]).
	if err := c.PlayPlaylist(ctx, playlistName, 1); err != nil {
		t.Fatalf("PlayPlaylist: %v", err)
	}

	// Wait for Status to confirm playback has started.
	startPID := ""
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		np, err := c.Status(ctx)
		if err == nil && np.Track.PersistentID != "" {
			startPID = np.Track.PersistentID
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if startPID == "" {
		t.Fatal("Status never reported a non-empty PID after PlayPlaylist")
	}

	queuedPID := tracks[2].PersistentID
	if queuedPID == "" || queuedPID == startPID {
		t.Fatalf("can't pick a distinct queue target: queuedPID=%q startPID=%q", queuedPID, startPID)
	}

	// Simulate the handoff: when the current track changes, call
	// PlayTrack(queuedPID). The "natural next" (tracks[1]) may play
	// for up to ~1s before our override lands — that's the spec's
	// accepted glitch.
	lastPID := startPID
	overrideDeadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(overrideDeadline) {
		np, err := c.Status(ctx)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if np.Track.PersistentID != lastPID {
			// Track change observed — dispatch the queued PID.
			if err := c.PlayTrack(ctx, queuedPID); err != nil {
				t.Fatalf("PlayTrack(queued): %v", err)
			}
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Now wait (up to 10s) for Status to reflect the queued track.
	confirmDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(confirmDeadline) {
		np, err := c.Status(ctx)
		if err == nil && np.Track.PersistentID == queuedPID {
			// Success — queued track is now playing.
			if err := c.Pause(ctx); err != nil {
				t.Logf("Pause (cleanup) returned: %v", err)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("queued track never became the playing track within 10s of override")
}
