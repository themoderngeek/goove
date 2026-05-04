package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
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
