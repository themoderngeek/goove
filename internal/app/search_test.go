package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
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
