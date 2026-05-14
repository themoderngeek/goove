package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestRenderOverlayEmptyState(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Queue [0]") {
		t.Errorf("missing 'Queue [0]' header: %q", got)
	}
	if !strings.Contains(got, "queue is empty") {
		t.Errorf("missing empty-state hint: %q", got)
	}
}

func TestRenderOverlayWithItemsAndCursor(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.overlay.cursor = 1
	m.queue.Add(domain.Track{Title: "HC", Artist: "Eagles", PersistentID: "HC"})
	m.queue.Add(domain.Track{Title: "WW", Artist: "Oasis", PersistentID: "WW"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Queue [2]") {
		t.Errorf("missing 'Queue [2]' header: %q", got)
	}
	if !strings.Contains(got, "HC") || !strings.Contains(got, "WW") {
		t.Errorf("missing queue items: %q", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("missing cursor glyph: %q", got)
	}
	// Find which row carries the cursor — should be the 2nd (cursor=1).
	lines := strings.Split(got, "\n")
	cursorLine := ""
	for _, ln := range lines {
		if strings.Contains(ln, "▶") {
			cursorLine = ln
			break
		}
	}
	if !strings.Contains(cursorLine, "WW") {
		t.Errorf("cursor on wrong row; cursor line = %q", cursorLine)
	}
}

func TestRenderOverlayResumeFooterShowsPlaylistContext(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	m.resume = ResumeContext{PlaylistName: "LZ", NextIndex: 4}
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{}, {}, {}, {}, {}, {}, {}, {}, // 8 tracks
	}
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "then resumes") {
		t.Errorf("missing resume footer: %q", got)
	}
	if !strings.Contains(got, "LZ") {
		t.Errorf("missing resume playlist name: %q", got)
	}
	if !strings.Contains(got, "track 4 of 8") {
		t.Errorf("missing 'track 4 of 8' resume detail: %q", got)
	}
}

func TestRenderOverlayResumeFooterShowsStopWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "then stops") {
		t.Errorf("missing 'then stops' footer when resume empty: %q", got)
	}
}

func TestRenderOverlayClearPromptOverridesHelpRow(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.clearPrompt = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Clear queue?") {
		t.Errorf("missing clear prompt: %q", got)
	}
	if strings.Contains(got, "j/k nav") {
		t.Errorf("regular help row still visible while clear prompt active: %q", got)
	}
}

func TestRenderOverlayHelpRowVisibleByDefault(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "j/k") {
		t.Errorf("missing help row: %q", got)
	}
}

func TestViewRendersOverlayWhenOpen(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}}
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := m.View()
	if !strings.Contains(got, "Queue [1]") {
		t.Errorf("View did not render overlay when open: %q", got)
	}
	// The Now Playing panel should NOT render when the overlay is open.
	if strings.Contains(got, "Now Playing") {
		t.Errorf("View should suppress normal panels when overlay open: %q", got)
	}
}
