package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestPlaylistsCursorDownMoves(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor = %d; want 1", got.playlists.cursor)
	}
}

func TestPlaylistsCursorUpClampsAtZero(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := updated.(Model)
	if got.playlists.cursor != 0 {
		t.Errorf("cursor = %d; want 0", got.playlists.cursor)
	}
}

func TestPlaylistsCursorDownClampsAtEnd(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor = %d; want 1 (clamped)", got.playlists.cursor)
	}
}

func TestPlaylistsCursorMoveUpdatesMainSelectedPlaylist(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.main.selectedPlaylist = "A"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.selectedPlaylist != "B" {
		t.Errorf("main.selectedPlaylist = %q; want B", got.main.selectedPlaylist)
	}
	if got.main.cursor != 0 {
		t.Errorf("main.cursor = %d; want 0 (reset on selection change)", got.main.cursor)
	}
}

func TestPlaylistsArrowsAlsoNavigate(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d; want 1", got.playlists.cursor)
	}
}

func TestPlaylistsCursorMovePreservesSearchResultsMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	// Simulate a fired search whose results are in main.
	m.main.mode = mainPaneSearchResults
	m.main.cursor = 7
	m.main.searchResults = []domain.Track{{Title: "x"}, {Title: "y"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.mode != mainPaneSearchResults {
		t.Errorf("main.mode = %v; want mainPaneSearchResults (preserved)", got.main.mode)
	}
	if got.main.cursor != 7 {
		t.Errorf("main.cursor = %d; want 7 (search-results cursor preserved)", got.main.cursor)
	}
	if got.main.selectedPlaylist != "B" {
		t.Errorf("main.selectedPlaylist = %q; want B (still updates so Esc lands here)", got.main.selectedPlaylist)
	}
}

func TestPlaylistsCursorChangeFiresTrackFetchOnFirstSelection(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected fetchPlaylistTracks Cmd on first selection of B")
	}
	if !got.playlists.fetchingFor["B"] {
		t.Errorf("expected fetchingFor[B] = true")
	}
	out := cmd()
	if _, ok := out.(playlistTracksMsg); !ok {
		t.Fatalf("cmd produced %T; want playlistTracksMsg", out)
	}
}

func TestPlaylistsCursorChangeUsesCacheOnRevisit(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.tracksByName["B"] = []domain.Track{{Title: "t1"}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		out := cmd()
		t.Errorf("expected no Cmd on cached selection, got %T", out)
	}
}

func TestPlaylistsCursorChangeNoDuplicateFetch(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.fetchingFor["B"] = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Errorf("expected no Cmd while a fetch for B is in flight")
	}
}

func TestPlaylistTracksMsgPopulatesCache(t *testing.T) {
	m := newTestModel()
	m.playlists.fetchingFor["B"] = true
	tracks := []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(playlistTracksMsg{name: "B", tracks: tracks})
	got := updated.(Model)
	if got.playlists.fetchingFor["B"] {
		t.Error("expected fetchingFor[B] cleared after result lands")
	}
	if len(got.playlists.tracksByName["B"]) != 2 {
		t.Errorf("tracksByName[B] = %v; want 2 entries", got.playlists.tracksByName["B"])
	}
}

func TestPlaylistTracksMsgClearsFetchingForOnError(t *testing.T) {
	m := newTestModel()
	m.playlists.fetchingFor["B"] = true
	updated, _ := m.Update(playlistTracksMsg{name: "B", err: errors.New("boom")})
	got := updated.(Model)
	if got.playlists.fetchingFor["B"] {
		t.Error("fetchingFor[B] must be cleared even on error")
	}
	if _, exists := got.playlists.tracksByName["B"]; exists {
		t.Error("tracksByName must not be written on error")
	}
}

func TestPlaylistsEnterPlaysHighlightedPlaylistFromTrackZero(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "A"}, {Name: "B"}})
	m := New(c, nil)
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	out := cmd()
	if _, ok := out.(playPlaylistMsg); !ok {
		t.Fatalf("cmd produced %T; want playPlaylistMsg", out)
	}
	if c.PlayPlaylistCalls != 1 {
		t.Errorf("PlayPlaylist calls = %d; want 1", c.PlayPlaylistCalls)
	}
	rec := c.PlayPlaylistRecord()
	if len(rec) == 0 || rec[0].Name != "B" {
		t.Errorf("LastPlayPlaylistName = %q; want B", func() string {
			if len(rec) > 0 {
				return rec[0].Name
			}
			return ""
		}())
	}
	if len(rec) == 0 || rec[0].FromIdx != 0 {
		t.Errorf("LastPlayPlaylistFromIdx = %d; want 0", func() int {
			if len(rec) > 0 {
				return rec[0].FromIdx
			}
			return -1
		}())
	}
}

func TestPlaylistsEnterIsNoOpWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd with empty list, got %T", cmd())
	}
}
