package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
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
