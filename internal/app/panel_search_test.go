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
	m.focusZ = focusSearch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := updated.(Model)
	if !got.search2.inputMode {
		t.Error("expected inputMode true after typing")
	}
	if got.search2.query != "l" {
		t.Errorf("query = %q; want %q", got.search2.query, "l")
	}
}

func TestSearchPanelMultipleKeysAppend(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	keys := []rune{'l', 'e', 'd'}
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = updated.(Model)
	}
	if m.search2.query != "led" {
		t.Errorf("query = %q; want %q", m.search2.query, "led")
	}
}

func TestSearchPanelBackspaceRemovesLastRune(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := updated.(Model)
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q", got.search2.query, "le")
	}
}

func TestSearchPanelEscClearsAndExitsInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.search2.inputMode {
		t.Error("expected inputMode false after esc")
	}
	if got.search2.query != "" {
		t.Errorf("query = %q; want empty", got.search2.query)
	}
}

func TestSearchPanelSpaceGoesIntoQuery(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.search2.query != "led " {
		t.Errorf("query = %q; want %q", got.search2.query, "led ")
	}
}

func TestSearchPanelNumberKeysStillJumpFocusInInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "le"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (1 always wins)", got.focusZ)
	}
	// Query unchanged.
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q (1 should not append)", got.search2.query, "le")
	}
}

func TestSearchPanelEnterFiresSearch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{{Title: "Stairway", Artist: "Led Zeppelin", PersistentID: "p1"}})
	m := New(c, nil)
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "stair"
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
	m.search2.seq = 5
	m.search2.query = "stair"
	tracks := []domain.Track{{Title: "Stairway", Artist: "Led Zeppelin"}}
	updated, _ := m.Update(searchPanelResultsMsg{seq: 5, query: "stair", result: music.SearchResult{Tracks: tracks, Total: 1}})
	got := updated.(Model)
	if got.main.mode != mainPaneSearchResults {
		t.Errorf("main.mode = %v; want mainPaneSearchResults", got.main.mode)
	}
	if len(got.main.searchResults) != 1 {
		t.Errorf("searchResults = %d; want 1", len(got.main.searchResults))
	}
	if got.focusZ != focusMain {
		t.Errorf("focusZ = %v; want focusMain", got.focusZ)
	}
}

func TestSearchPanelStaleSeqDropped(t *testing.T) {
	m := newTestModel()
	m.search2.seq = 5
	updated, _ := m.Update(searchPanelResultsMsg{seq: 4, query: "old"})
	got := updated.(Model)
	if got.main.mode == mainPaneSearchResults {
		t.Error("stale seq should not have populated main pane")
	}
}

func TestSearchPanelEnterEmptyQueryNoOp(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd on empty query, got %T", cmd())
	}
}

func TestSearchPanelResultsMsgErrorPreservesInputModeAndQuery(t *testing.T) {
	m := newTestModel()
	m.search2.seq = 5
	m.search2.query = "stair"
	m.search2.inputMode = true
	m.search2.loading = true
	updated, _ := m.Update(searchPanelResultsMsg{seq: 5, query: "stair", err: errors.New("boom")})
	got := updated.(Model)
	if !got.search2.inputMode {
		t.Error("inputMode should be preserved on error so user can retry")
	}
	if got.search2.query != "stair" {
		t.Errorf("query = %q; want preserved 'stair'", got.search2.query)
	}
	if got.search2.err == nil {
		t.Error("err should be set")
	}
	if got.search2.loading {
		t.Error("loading should be cleared")
	}
	if got.search2.lastQuery != "" {
		t.Errorf("lastQuery should NOT be written on error, got %q", got.search2.lastQuery)
	}
	if got.main.mode == mainPaneSearchResults {
		t.Error("main pane should not flip to search-results on error")
	}
}
