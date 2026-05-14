package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func openWithItems(items ...domain.Track) Model {
	m := newTestModel()
	m.overlay.open = true
	for _, t := range items {
		m.queue.Add(t)
	}
	return m
}

func TestOverlayJKMovesCursor(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.overlay.cursor != 1 {
		t.Errorf("after j: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.overlay.cursor != 2 {
		t.Errorf("clamped at last: cursor = %d; want 2", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.overlay.cursor != 1 {
		t.Errorf("after k: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.overlay.cursor != 0 {
		t.Errorf("clamped at head: cursor = %d; want 0", m.overlay.cursor)
	}
}

func TestOverlayXRemovesAtCursor(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m.overlay.cursor = 1
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.queue.Len() != 2 {
		t.Errorf("after x: Len = %d; want 2", m.queue.Len())
	}
	if m.queue.Items[1].PersistentID != "C" {
		t.Errorf("after x at 1: items[1] = %s; want C", m.queue.Items[1].PersistentID)
	}
}

func TestOverlayXClampsCursorAfterTailRemoval(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
	)
	m.overlay.cursor = 1
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.overlay.cursor != 0 {
		t.Errorf("cursor not clamped after tail removal: cursor = %d; want 0", m.overlay.cursor)
	}
}

func TestOverlayKJReordersAndCursorFollows(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m.overlay.cursor = 2
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.queue.Items[1].PersistentID != "C" {
		t.Errorf("after K: items[1] = %s; want C", m.queue.Items[1].PersistentID)
	}
	if m.overlay.cursor != 1 {
		t.Errorf("cursor not following K: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.queue.Items[2].PersistentID != "C" {
		t.Errorf("after J: items[2] = %s; want C", m.queue.Items[2].PersistentID)
	}
	if m.overlay.cursor != 2 {
		t.Errorf("cursor not following J: cursor = %d; want 2", m.overlay.cursor)
	}
}

func TestOverlayCThenYClears(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !m.clearPrompt {
		t.Fatal("clearPrompt not set after c")
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.queue.Len() != 0 {
		t.Errorf("queue not cleared after y: Len = %d", m.queue.Len())
	}
	if m.clearPrompt {
		t.Error("clearPrompt not reset after y")
	}
}

func TestOverlayCThenOtherKeyCancels(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.queue.Len() != 1 {
		t.Errorf("queue cleared on non-y after c: Len = %d; want 1", m.queue.Len())
	}
	if m.clearPrompt {
		t.Error("clearPrompt not reset after cancel key")
	}
}

func TestOverlayEscClosesAndResetsClearPrompt(t *testing.T) {
	m := openWithItems(domain.Track{Title: "A", PersistentID: "A"})
	m.clearPrompt = true
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.overlay.open {
		t.Error("overlay.open not cleared on Esc")
	}
	if m.clearPrompt {
		t.Error("clearPrompt not cleared on Esc")
	}
}

func TestOverlayQClosesAsWell(t *testing.T) {
	m := openWithItems(domain.Track{Title: "A", PersistentID: "A"})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	if m.overlay.open {
		t.Error("overlay.open not cleared on Q")
	}
}

func TestOverlayEnterDispatchesPlayTrackAndRemovesAndSetsPending(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A1"},
		domain.Track{Title: "B", PersistentID: "B1"},
		domain.Track{Title: "C", PersistentID: "C1"},
	)
	m.overlay.cursor = 2
	got, cmd := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a PlayTrack cmd")
	}
	if got.pendingJumpPID != "C1" {
		t.Errorf("pendingJumpPID = %q; want C1", got.pendingJumpPID)
	}
	if got.queue.Len() != 2 {
		t.Errorf("queue.Len = %d; want 2 (selected removed)", got.queue.Len())
	}
	if got.queue.Items[0].PersistentID != "A1" || got.queue.Items[1].PersistentID != "B1" {
		t.Errorf("remaining queue = %v; want [A1 B1]", got.queue.Items)
	}
	// Verify cmd routes to fake's PlayTrack — note the fake errs because
	// C1 isn't in its library; the cmd still completes.
	_ = cmd()
}

func TestOverlayEnterClampsCursorAfterRemoval(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A1"},
		domain.Track{Title: "B", PersistentID: "B1"},
	)
	m.overlay.cursor = 1
	got, _ := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got.overlay.cursor != 0 {
		t.Errorf("cursor not clamped: cursor = %d; want 0", got.overlay.cursor)
	}
}

func TestOverlayEnterOnEmptyQueueIsNoOp(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got, cmd := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("cmd != nil on Enter with empty queue")
	}
	if got.pendingJumpPID != "" {
		t.Errorf("pendingJumpPID set on empty queue: %q", got.pendingJumpPID)
	}
}
