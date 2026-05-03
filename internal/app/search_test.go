package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
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
