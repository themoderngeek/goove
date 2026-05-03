package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestRenderSearch_EmptyInput(t *testing.T) {
	got := renderSearch(&searchState{})
	if !strings.Contains(got, "type to search your library") {
		t.Errorf("missing empty-state hint:\n%s", got)
	}
}

func TestRenderSearch_Loading(t *testing.T) {
	got := renderSearch(&searchState{query: "stair", loading: true})
	if !strings.Contains(got, "searching") {
		t.Errorf("missing searching hint:\n%s", got)
	}
}

func TestRenderSearch_NoMatches(t *testing.T) {
	got := renderSearch(&searchState{query: "zzqq"})
	if !strings.Contains(got, "no matches in your library") {
		t.Errorf("missing no-matches text:\n%s", got)
	}
}

func TestRenderSearch_Results(t *testing.T) {
	s := &searchState{
		query: "stair",
		total: 3,
		results: []domain.Track{
			{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV", PersistentID: "A"},
			{Title: "Take the Stairs", Artist: "Phantogram", Album: "Three", PersistentID: "B"},
		},
		cursor: 0,
	}
	got := renderSearch(s)
	if !strings.Contains(got, "Stairway to Heaven") {
		t.Errorf("missing first track:\n%s", got)
	}
	if !strings.Contains(got, "Phantogram") {
		t.Errorf("missing second track:\n%s", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("missing cursor marker:\n%s", got)
	}
}

func TestRenderSearch_TruncationHint(t *testing.T) {
	s := &searchState{query: "the", total: 412}
	for i := 0; i < 100; i++ {
		s.results = append(s.results, domain.Track{Title: "x", PersistentID: "p"})
	}
	got := renderSearch(s)
	if !strings.Contains(got, "100 of 412") {
		t.Errorf("missing truncation hint:\n%s", got)
	}
}

func TestRenderSearch_ErrorFooter(t *testing.T) {
	s := &searchState{query: "stair", err: errSentinel("boom")}
	got := renderSearch(s)
	if !strings.Contains(got, "error: boom") {
		t.Errorf("missing error footer:\n%s", got)
	}
	if !strings.Contains(got, "r retry") {
		t.Errorf("error state should label r as retry:\n%s", got)
	}
}

// errSentinel is a tiny test-only error wrapper.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// connectedTestModel returns a Model whose fake client is running and whose
// state is Connected. Tests that need a populated library can call
// SetLibraryTracks on the returned client.
func connectedTestModel(t *testing.T) (Model, *fake.Client) {
	t.Helper()
	c := fake.New()
	if err := c.Launch(context.Background()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	m := New(c, nil)
	m.state = Connected{}
	return m, c
}

func TestSlash_OpensSearchFromConnected(t *testing.T) {
	m, _ := connectedTestModel(t)
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := out.(Model)
	if mm.search == nil {
		t.Fatalf("expected search state, got nil")
	}
	if mm.search.query != "" {
		t.Errorf("expected empty query, got %q", mm.search.query)
	}
}

func TestSlash_NoOpInDisconnected(t *testing.T) {
	// Default newTestModel is Disconnected.
	m := newTestModel()
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open when Disconnected")
	}
}

func TestSlash_NoOpWhenPickerOpen(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.picker = &pickerState{}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open while picker is open")
	}
}

func TestSlash_NoOpWhenBrowserOpen(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.mode = modeBrowser
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open while browser is open")
	}
}

func TestEsc_ClosesSearch(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair"}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if out.(Model).search != nil {
		t.Errorf("esc should close search")
	}
}

func TestTyping_StartsDebounce_BumpsSeq(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{}
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	mm := out.(Model)
	if mm.search.query != "s" {
		t.Errorf("query: got %q want %q", mm.search.query, "s")
	}
	if mm.search.seq != 1 {
		t.Errorf("seq: got %d want 1", mm.search.seq)
	}
	if cmd == nil {
		t.Errorf("expected debounce Cmd, got nil")
	}
}

func TestBackspace_RemovesLastRune(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 5}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	mm := out.(Model)
	if mm.search.query != "stai" {
		t.Errorf("query: got %q want %q", mm.search.query, "stai")
	}
	if mm.search.seq != 6 {
		t.Errorf("seq: got %d want 6", mm.search.seq)
	}
}

func TestBackspace_OnEmptyQuery_NoOp(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if out.(Model).search.query != "" {
		t.Errorf("expected query still empty")
	}
}

func TestDebounceMsg_StaleSeqDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 10}
	out, cmd := m.Update(searchDebounceMsg{seq: 7})
	if cmd != nil {
		t.Errorf("stale debounce should not fire query")
	}
	if out.(Model).search.loading {
		t.Errorf("stale debounce should not set loading")
	}
}

func TestDebounceMsg_EmptyQueryDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{seq: 1}
	_, cmd := m.Update(searchDebounceMsg{seq: 1})
	if cmd != nil {
		t.Errorf("empty-query debounce should not fire query")
	}
}

func TestDebounceMsg_FreshFiresQuery(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 2}
	out, cmd := m.Update(searchDebounceMsg{seq: 2})
	if cmd == nil {
		t.Errorf("expected SearchTracks Cmd")
	}
	if !out.(Model).search.loading {
		t.Errorf("expected loading=true")
	}
}

func TestResultsMsg_StaleSeqDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 10, loading: true}
	out, _ := m.Update(searchResultsMsg{seq: 5, query: "stair"})
	mm := out.(Model)
	if !mm.search.loading {
		t.Errorf("stale result should not clear loading")
	}
}

func TestResultsMsg_QueryMismatchDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 3, loading: true}
	out, _ := m.Update(searchResultsMsg{seq: 3, query: "different"})
	if !out.(Model).search.loading {
		t.Errorf("query-mismatch result should not clear loading")
	}
}

func TestResultsMsg_FreshPopulatesAndRanks(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 3, loading: true}
	result := music.SearchResult{
		Tracks: []domain.Track{
			{Title: "Album-only", Album: "Stair Master", PersistentID: "C"},
			{Title: "Stairway", Artist: "X", Album: "Y", PersistentID: "A"},
		},
		Total: 2,
	}
	out, _ := m.Update(searchResultsMsg{seq: 3, query: "stair", result: result})
	mm := out.(Model)
	if mm.search.loading {
		t.Errorf("loading should clear on fresh result")
	}
	if mm.search.total != 2 || len(mm.search.results) != 2 {
		t.Errorf("results not populated: %+v", mm.search)
	}
	// Title-match ranks first.
	if mm.search.results[0].PersistentID != "A" {
		t.Errorf("expected title-match first, got %+v", mm.search.results[0])
	}
	if mm.search.cursor != 0 {
		t.Errorf("cursor should reset to 0, got %d", mm.search.cursor)
	}
}
