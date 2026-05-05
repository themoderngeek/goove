package app

import (
	"errors"
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

func newPlaylistsPanel() playlistsPanel {
	return playlistsPanel{
		tracksByName:   make(map[string][]domain.Track),
		fetchingFor:    make(map[string]bool),
		trackErrByName: make(map[string]error),
	}
}

func TestRenderUpNextReturnsEmptyWhenNoRows(t *testing.T) {
	now := domain.NowPlaying{CurrentPlaylistName: "P", Track: domain.Track{PersistentID: "PID"}}
	p := newPlaylistsPanel()
	if got := renderUpNext(now, p, 0, 30); got != "" {
		t.Errorf("rows=0: got %q; want empty", got)
	}
	if got := renderUpNext(now, p, 5, 0); got != "" {
		t.Errorf("width=0: got %q; want empty", got)
	}
}

func TestRenderUpNextShufflePlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		ShuffleEnabled:      true,
		Track:               domain.Track{PersistentID: "PID"},
	}
	p := newPlaylistsPanel()
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "Up Next") {
		t.Errorf("missing header: %q", got)
	}
	if !strings.Contains(got, "shuffling") {
		t.Errorf("missing shuffle placeholder: %q", got)
	}
}

func TestRenderUpNextNoPlaylistContextPlaceholder(t *testing.T) {
	now := domain.NowPlaying{CurrentPlaylistName: ""}
	p := newPlaylistsPanel()
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "no queue") {
		t.Errorf("missing 'no queue' placeholder: %q", got)
	}
}

func TestRenderUpNextLoadingPlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Recents",
		Track:               domain.Track{PersistentID: "PID"},
	}
	p := newPlaylistsPanel()
	p.fetchingFor["Recents"] = true
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "loading") {
		t.Errorf("missing 'loading' placeholder: %q", got)
	}
}

func TestRenderUpNextLoadingWhenCacheMissNotYetFetching(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Recents",
		Track:               domain.Track{PersistentID: "PID"},
	}
	p := newPlaylistsPanel()
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "loading") {
		t.Errorf("expected 'loading' placeholder when cache miss + not fetching: %q", got)
	}
}

func TestRenderUpNextCacheErrorPlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Recents",
		Track:               domain.Track{PersistentID: "PID"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["Recents"] = []domain.Track{{PersistentID: "X"}}
	p.trackErrByName["Recents"] = errors.New("boom")
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "no queue") {
		t.Errorf("expected 'no queue' on cache error: %q", got)
	}
}

func TestRenderUpNextHappyPath(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Liked Songs",
		Track:               domain.Track{PersistentID: "PID-3"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["Liked Songs"] = []domain.Track{
		{Title: "Black Dog", Artist: "Led Zeppelin", PersistentID: "PID-1"},
		{Title: "Rock and Roll", Artist: "Led Zeppelin", PersistentID: "PID-2"},
		{Title: "Stairway", Artist: "Led Zeppelin", PersistentID: "PID-3"},
		{Title: "Misty Mountain Hop", Artist: "Led Zeppelin", PersistentID: "PID-4"},
		{Title: "Four Sticks", Artist: "Led Zeppelin", PersistentID: "PID-5"},
	}
	got := renderUpNext(now, p, 5, 60)
	if !strings.Contains(got, "Up Next") {
		t.Errorf("missing header: %q", got)
	}
	if !strings.Contains(got, "Misty Mountain Hop") {
		t.Errorf("missing upcoming track: %q", got)
	}
	if !strings.Contains(got, "Four Sticks") {
		t.Errorf("missing upcoming track: %q", got)
	}
	if strings.Contains(got, "Black Dog") || strings.Contains(got, "Rock and Roll") || strings.Contains(got, "Stairway") {
		t.Errorf("unexpected past/current track in output: %q", got)
	}
}

func TestRenderUpNextEndOfPlaylistPlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Liked Songs",
		Track:               domain.Track{PersistentID: "PID-LAST"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["Liked Songs"] = []domain.Track{
		{Title: "First", PersistentID: "PID-1"},
		{Title: "Last", PersistentID: "PID-LAST"},
	}
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "end of playlist") {
		t.Errorf("expected 'end of playlist': %q", got)
	}
}

func TestRenderUpNextCurrentTrackNotInPlaylistPlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "Liked Songs",
		Track:               domain.Track{PersistentID: "PID-UNKNOWN"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["Liked Songs"] = []domain.Track{
		{Title: "A", PersistentID: "PID-1"},
		{Title: "B", PersistentID: "PID-2"},
	}
	got := renderUpNext(now, p, 5, 30)
	if !strings.Contains(got, "no queue") {
		t.Errorf("expected 'no queue' when current track not in playlist: %q", got)
	}
}

func TestRenderUpNextTruncatesTrackTitlesToWidth(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "PID-1"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["P"] = []domain.Track{
		{Title: "Cur", PersistentID: "PID-1"},
		{Title: "ThisTitleIsAbsurdlyLongAndShouldGetTruncatedByTheRenderer", Artist: "X", PersistentID: "PID-2"},
	}
	got := renderUpNext(now, p, 5, 20)
	if strings.Contains(got, "TruncatedByTheRenderer") {
		t.Errorf("expected truncation; got full title in: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in truncated row: %q", got)
	}
}

func TestRenderUpNextCapsAtAvailableRows(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "PID-0"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["P"] = []domain.Track{
		{Title: "T0", PersistentID: "PID-0"},
		{Title: "T1", PersistentID: "PID-1"},
		{Title: "T2", PersistentID: "PID-2"},
		{Title: "T3", PersistentID: "PID-3"},
	}
	got := renderUpNext(now, p, 2, 30) // 2 body rows = 2 tracks max
	if !strings.Contains(got, "T1") || !strings.Contains(got, "T2") {
		t.Errorf("expected first two upcoming: %q", got)
	}
	if strings.Contains(got, "T3") {
		t.Errorf("third upcoming track should be capped out: %q", got)
	}
}
