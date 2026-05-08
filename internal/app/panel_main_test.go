package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestMainPaneShowsLoadingWhenSelectionUncached(t *testing.T) {
	m := newTestModel()
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "B"
	m.playlists.fetchingFor["B"] = true
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "loading") {
		t.Errorf("main pane did not show loading state: %q", got)
	}
}

func TestMainPaneShowsTracksWhenCached(t *testing.T) {
	m := newTestModel()
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "B"
	m.playlists.tracksByName["B"] = []domain.Track{
		{Title: "Track One", Artist: "Artist A"},
		{Title: "Track Two", Artist: "Artist B"},
	}
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "Track One") {
		t.Errorf("main pane missing 'Track One': %q", got)
	}
	if !strings.Contains(got, "Track Two") {
		t.Errorf("main pane missing 'Track Two': %q", got)
	}
}

func TestMainPaneShowsHintWhenNothingSelected(t *testing.T) {
	m := newTestModel()
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "focus") && !strings.Contains(got, "—") {
		t.Errorf("main pane hint missing: %q", got)
	}
}

func TestMainTracksCursorDownMoves(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.playlists.tracksByName["A"] = []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.cursor != 1 {
		t.Errorf("main.cursor = %d; want 1", got.main.cursor)
	}
}

func TestMainTracksCursorClampsAtEnd(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.main.cursor = 1
	m.playlists.tracksByName["A"] = []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.cursor != 1 {
		t.Errorf("main.cursor = %d; want 1 (clamped)", got.main.cursor)
	}
}

func TestMainTracksEnterPlaysFromCursor(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetPlaylists([]domain.Playlist{{Name: "A"}})
	m := New(c, nil)
	m.focus = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.main.cursor = 2
	m.playlists.tracksByName["A"] = []domain.Track{
		{Title: "t1"}, {Title: "t2"}, {Title: "t3"}, {Title: "t4"},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected playPlaylist Cmd")
	}
	cmd()
	rec := c.PlayPlaylistRecord()
	if len(rec) != 1 {
		t.Fatalf("PlayPlaylistRecord len = %d; want 1", len(rec))
	}
	if rec[0].FromIdx != 2 {
		t.Errorf("FromIdx = %d; want 2", rec[0].FromIdx)
	}
	if rec[0].Name != "A" {
		t.Errorf("Name = %q; want A", rec[0].Name)
	}
}

func TestMainTracksEnterIsNoOpWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.main.mode = mainPaneTracks
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd with empty selection, got %T", cmd())
	}
}

func TestMainPaneEscReturnsToTracksFromSearchResults(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "x"}}
	m.main.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.main.mode != mainPaneTracks {
		t.Errorf("main.mode after esc = %v; want mainPaneTracks", got.main.mode)
	}
	if got.main.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (reset)", got.main.cursor)
	}
}

func TestMainPaneEscInTracksModeIsNoOp(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.main.mode = mainPaneTracks
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.main.mode != mainPaneTracks {
		t.Errorf("main.mode after esc in tracks mode = %v; want unchanged", got.main.mode)
	}
}

func TestMainPaneShowsTrackFetchErrorForSelectedPlaylist(t *testing.T) {
	m := newTestModel()
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "B"
	m.playlists.trackErrByName["B"] = errors.New("signal: killed")
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "couldn't load tracks") {
		t.Errorf("main pane did not show track-fetch error: %q", got)
	}
	if !strings.Contains(got, "signal: killed") {
		t.Errorf("main pane did not include underlying error: %q", got)
	}
}
