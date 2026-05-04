package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestNowPlayingRendersConnectedTrack(t *testing.T) {
	m := newTestModel()
	m.state = Connected{Now: domain.NowPlaying{
		Track:  domain.Track{Title: "Stairway", Artist: "Led Zeppelin"},
		Volume: 50,
	}}
	got := renderNowPlayingPanel(m, m.width)
	if !strings.Contains(got, "Stairway") {
		t.Errorf("missing title: %q", got)
	}
	if !strings.Contains(got, "Led Zeppelin") {
		t.Errorf("missing artist: %q", got)
	}
}

func TestNowPlayingRendersIdle(t *testing.T) {
	m := newTestModel()
	m.state = Idle{Volume: 50}
	got := renderNowPlayingPanel(m, m.width)
	if !strings.Contains(got, "nothing playing") && !strings.Contains(got, "Music is open") {
		t.Errorf("idle missing expected text: %q", got)
	}
}

func TestNowPlayingRendersDisconnected(t *testing.T) {
	m := newTestModel()
	m.state = Disconnected{}
	got := renderNowPlayingPanel(m, m.width)
	if !strings.Contains(got, "isn't running") && !strings.Contains(got, "Music") {
		t.Errorf("disconnected missing expected text: %q", got)
	}
}

func TestNowPlayingArtAppearsWhenWideAndCached(t *testing.T) {
	m := newTestModel()
	m.width = 100 // > artLayoutThreshold (70)
	track := domain.Track{Title: "T", Artist: "A", Album: "Al"}
	m.state = Connected{Now: domain.NowPlaying{Track: track, Volume: 50}}
	m.art = artState{key: trackKey(track), output: "ART_OUTPUT_HERE"}
	got := renderNowPlayingPanel(m, m.width)
	if !strings.Contains(got, "ART_OUTPUT_HERE") {
		t.Errorf("expected art content; got %q", got)
	}
}

func TestNowPlayingArtHiddenBelowThreshold(t *testing.T) {
	m := newTestModel()
	m.width = 50 // < artLayoutThreshold
	track := domain.Track{Title: "T", Artist: "A", Album: "Al"}
	m.state = Connected{Now: domain.NowPlaying{Track: track, Volume: 50}}
	m.art = artState{key: trackKey(track), output: "ART_OUTPUT_HERE"}
	got := renderNowPlayingPanel(m, m.width)
	if strings.Contains(got, "ART_OUTPUT_HERE") {
		t.Errorf("expected art hidden below threshold; got %q", got)
	}
}
