package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestSearchPanelTypingEntersInputModeAndAppendsQuery(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := updated.(Model)
	if !got.search.inputMode {
		t.Error("expected inputMode true after typing")
	}
	if got.search.query != "l" {
		t.Errorf("query = %q; want %q", got.search.query, "l")
	}
}

func TestSearchPanelMultipleKeysAppend(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	keys := []rune{'l', 'e', 'd'}
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = updated.(Model)
	}
	if m.search.query != "led" {
		t.Errorf("query = %q; want %q", m.search.query, "led")
	}
}

func TestSearchPanelBackspaceRemovesLastRune(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.inputMode = true
	m.search.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := updated.(Model)
	if got.search.query != "le" {
		t.Errorf("query = %q; want %q", got.search.query, "le")
	}
}

func TestSearchPanelEscClearsAndExitsInputMode(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.inputMode = true
	m.search.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.search.inputMode {
		t.Error("expected inputMode false after esc")
	}
	if got.search.query != "" {
		t.Errorf("query = %q; want empty", got.search.query)
	}
}

func TestSearchPanelSpaceGoesIntoQuery(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.inputMode = true
	m.search.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.search.query != "led " {
		t.Errorf("query = %q; want %q", got.search.query, "led ")
	}
}

func TestSearchPanelNumberKeysStillJumpFocusInInputMode(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.inputMode = true
	m.search.query = "le"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focus != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (1 always wins)", got.focus)
	}
	// Query unchanged.
	if got.search.query != "le" {
		t.Errorf("query = %q; want %q (1 should not append)", got.search.query, "le")
	}
}

func TestSearchPanelEnterFiresSearch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{{Title: "Stairway", Artist: "Led Zeppelin", PersistentID: "p1"}})
	m := New(c, nil)
	m.focus = focusSearch
	m.search.inputMode = true
	m.search.query = "stair"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected fireSearchPanel Cmd")
	}
	out := cmd()
	res, ok := out.(searchPanelResultsMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want searchPanelResultsMsg", out)
	}
	if res.query != "stair" {
		t.Errorf("query = %q", res.query)
	}
}

func TestSearchPanelResultsMsgPopulatesMainPane(t *testing.T) {
	m := newTestModel()
	m.search.seq = 5
	m.search.query = "stair"
	tracks := []domain.Track{{Title: "Stairway", Artist: "Led Zeppelin"}}
	updated, _ := m.Update(searchPanelResultsMsg{seq: 5, query: "stair", result: music.SearchResult{Tracks: tracks, Total: 1}})
	got := updated.(Model)
	if got.main.mode != mainPaneSearchResults {
		t.Errorf("main.mode = %v; want mainPaneSearchResults", got.main.mode)
	}
	if len(got.main.searchResults) != 1 {
		t.Errorf("searchResults = %d; want 1", len(got.main.searchResults))
	}
	if got.focus != focusMain {
		t.Errorf("focusZ = %v; want focusMain", got.focus)
	}
}

func TestSearchPanelStaleSeqDropped(t *testing.T) {
	m := newTestModel()
	m.search.seq = 5
	updated, _ := m.Update(searchPanelResultsMsg{seq: 4, query: "old"})
	got := updated.(Model)
	if got.main.mode == mainPaneSearchResults {
		t.Error("stale seq should not have populated main pane")
	}
}

func TestSearchPanelEnterEmptyQueryNoOp(t *testing.T) {
	m := newTestModel()
	m.focus = focusSearch
	m.search.inputMode = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd on empty query, got %T", cmd())
	}
}

func TestSearchPanelResultsMsgErrorPreservesInputModeAndQuery(t *testing.T) {
	m := newTestModel()
	m.search.seq = 5
	m.search.query = "stair"
	m.search.inputMode = true
	m.search.loading = true
	updated, _ := m.Update(searchPanelResultsMsg{seq: 5, query: "stair", err: errors.New("boom")})
	got := updated.(Model)
	if !got.search.inputMode {
		t.Error("inputMode should be preserved on error so user can retry")
	}
	if got.search.query != "stair" {
		t.Errorf("query = %q; want preserved 'stair'", got.search.query)
	}
	if got.search.err == nil {
		t.Error("err should be set")
	}
	if got.search.loading {
		t.Error("loading should be cleared")
	}
	if got.search.lastQuery != "" {
		t.Errorf("lastQuery should NOT be written on error, got %q", got.search.lastQuery)
	}
	if got.main.mode == mainPaneSearchResults {
		t.Error("main pane should not flip to search-results on error")
	}
}
