package app

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/art"
	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func newTestModel() Model {
	c := fake.New()
	return New(c, nil) // nil renderer = album art disabled in tests
}

func TestStatusMsgWithSuccessTransitionsToConnected(t *testing.T) {
	m := newTestModel()
	np := domain.NowPlaying{
		Track:        domain.Track{Title: "T"},
		Volume:       60,
		IsPlaying:    true,
		LastSyncedAt: time.Now(),
	}
	updated, _ := m.Update(statusMsg{now: np})
	got := updated.(Model)
	conn, ok := got.state.(Connected)
	if !ok {
		t.Fatalf("state = %T; want Connected", got.state)
	}
	if conn.Now.Track.Title != "T" {
		t.Errorf("Title = %q", conn.Now.Track.Title)
	}
	if got.lastVolume != 60 {
		t.Errorf("lastVolume = %d; want 60", got.lastVolume)
	}
}

func TestStatusMsgErrNotRunningTransitionsToDisconnected(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(statusMsg{err: music.ErrNotRunning})
	if _, ok := updated.(Model).state.(Disconnected); !ok {
		t.Fatalf("state = %T; want Disconnected", updated.(Model).state)
	}
}

func TestStatusMsgErrNoTrackTransitionsToIdleWithLastVolume(t *testing.T) {
	m := newTestModel()
	m.lastVolume = 73
	updated, _ := m.Update(statusMsg{err: music.ErrNoTrack})
	idle, ok := updated.(Model).state.(Idle)
	if !ok {
		t.Fatalf("state = %T; want Idle", updated.(Model).state)
	}
	if idle.Volume != 73 {
		t.Errorf("Idle.Volume = %d; want 73", idle.Volume)
	}
}

func TestStatusMsgErrPermissionSetsPermissionDenied(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(statusMsg{err: music.ErrPermission})
	got := updated.(Model)
	if !got.permissionDenied {
		t.Fatal("expected permissionDenied = true")
	}
}

func TestStatusMsgGenericErrorSetsLastError(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(statusMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.lastError == nil {
		t.Fatal("expected lastError set")
	}
	if cmd == nil {
		t.Fatal("expected a clearErrorAfter Cmd to be returned")
	}
}

func TestSpaceTriggersPlayPauseAction(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)

	// Sync model to Connected state so space triggers PlayPause, not Launch.
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50, IsPlaying: false}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	out := cmd()
	if _, ok := out.(actionDoneMsg); !ok {
		t.Fatalf("cmd returned %T; want actionDoneMsg", out)
	}
	if c.PlayPauseCalls != 1 {
		t.Errorf("PlayPause calls = %d; want 1", c.PlayPauseCalls)
	}
}

func TestNKeyTriggersNext(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.NextCalls != 1 {
		t.Errorf("Next calls = %d; want 1", c.NextCalls)
	}
}

func TestPKeyTriggersPrev(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	cmd()
	if c.PrevCalls != 1 {
		t.Errorf("Prev calls = %d; want 1", c.PrevCalls)
	}
}

func TestVolumeUpOptimisticallyUpdatesAndCallsSetVolume(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)

	// Sync once to populate Connected state with volume=50 (fake default).
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50, IsPlaying: false}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	got := updated.(Model)

	conn, ok := got.state.(Connected)
	if !ok {
		t.Fatalf("state = %T; want Connected", got.state)
	}
	if conn.Now.Volume != 55 {
		t.Errorf("optimistic Volume = %d; want 55", conn.Now.Volume)
	}
	if got.lastVolume != 55 {
		t.Errorf("lastVolume = %d; want 55", got.lastVolume)
	}
	cmd()
	if c.SetVolumeCalls != 1 {
		t.Errorf("SetVolume calls = %d; want 1", c.SetVolumeCalls)
	}
}

func TestVolumeDownClampsAtZero(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	m.lastVolume = 3
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Volume: 3, Track: domain.Track{Title: "T"}}})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	got := updated.(Model)
	if got.lastVolume != 0 {
		t.Errorf("lastVolume = %d; want 0 (clamped)", got.lastVolume)
	}
}

func TestQKeyEmitsQuit(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	out := cmd()
	if _, ok := out.(tea.QuitMsg); !ok {
		t.Fatalf("cmd returned %T; want tea.QuitMsg", out)
	}
}

func TestSpaceWhileDisconnectedTriggersLaunch(t *testing.T) {
	c := fake.New()
	m := New(c, nil) // state=Disconnected
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.LaunchCalls != 1 {
		t.Errorf("Launch calls = %d; want 1", c.LaunchCalls)
	}
}

func TestActionDoneFiresStatusRefresh(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	m := New(c, nil)

	_, cmd := m.Update(actionDoneMsg{})
	if cmd == nil {
		t.Fatal("expected a status-refresh Cmd")
	}
	out := cmd()
	if _, ok := out.(statusMsg); !ok {
		t.Fatalf("cmd returned %T; want statusMsg", out)
	}
}

func TestActionDoneWithErrorSetsLastError(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	updated, _ := m.Update(actionDoneMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.lastError == nil {
		t.Fatal("expected lastError set")
	}
}

func TestTickMsgFiresStatusFetchAndReschedules(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 5, true)
	m := New(c, nil)

	_, cmd := m.Update(tickMsg{now: time.Now()})
	if cmd == nil {
		t.Fatal("expected a Cmd from tickMsg")
	}
	// We don't introspect the Batch contents — the existence of a Cmd
	// is the contract. The status fetch is exercised by other tests.
}

func TestRepaintMsgReturnsRepaintTickCmd(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(repaintMsg{})
	if cmd == nil {
		t.Fatal("expected a Cmd from repaintMsg")
	}
}

func TestClearErrorMsgClearsLastError(t *testing.T) {
	m := newTestModel()
	m.lastError = errors.New("x")
	updated, _ := m.Update(clearErrorMsg{})
	if updated.(Model).lastError != nil {
		t.Fatal("expected lastError cleared")
	}
}

func TestWindowSizeMsgUpdatesDimensions(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(Model)
	if got.width != 80 || got.height != 24 {
		t.Fatalf("width/height = %d/%d", got.width, got.height)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"},
		{59 * time.Second, "0:59"},
		{60 * time.Second, "1:00"},
		{3*time.Minute + 42*time.Second, "3:42"},
		{8*time.Minute + 2*time.Second, "8:02"},
		{-5 * time.Second, "0:00"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrackKeyReturnsEmptyForZeroTrack(t *testing.T) {
	if got := trackKey(domain.Track{}); got != "" {
		t.Errorf("trackKey(zero) = %q; want empty", got)
	}
}

func TestTrackKeyJoinsFields(t *testing.T) {
	got := trackKey(domain.Track{Title: "T", Artist: "A", Album: "B"})
	want := "T|A|B"
	if got != want {
		t.Errorf("trackKey = %q; want %q", got, want)
	}
}

func TestTrackKeyHandlesPartialFields(t *testing.T) {
	// Real Music.app tracks may have empty Album. Single non-empty field is
	// enough to constitute "a real track" — only all-empty returns "".
	got := trackKey(domain.Track{Title: "T"})
	if got == "" {
		t.Errorf("trackKey(title-only) = %q; want non-empty", got)
	}
}

func TestCurrentArtKeyReturnsEmptyOutsideConnected(t *testing.T) {
	m := newTestModel()
	// Default state is Disconnected{}.
	if got := m.currentArtKey(); got != "" {
		t.Errorf("currentArtKey on Disconnected = %q; want empty", got)
	}
	m.state = Idle{Volume: 50}
	if got := m.currentArtKey(); got != "" {
		t.Errorf("currentArtKey on Idle = %q; want empty", got)
	}
}

func TestCurrentArtKeyMatchesTrackInConnected(t *testing.T) {
	m := newTestModel()
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T", Artist: "A", Album: "B"}}}
	if got := m.currentArtKey(); got != "T|A|B" {
		t.Errorf("currentArtKey = %q; want T|A|B", got)
	}
}

// stubRenderer is an art.Renderer that returns a fixed string. Used to verify
// fetchArtwork was invoked and to inspect what came out the other side.
type stubRenderer struct{ out string }

func (s stubRenderer) Render(ctx context.Context, image []byte, w, h int) (string, error) {
	return s.out, nil
}

// compile-time interface check
var _ art.Renderer = stubRenderer{}

func TestStatusMsgWithNewTrackFiresFetchArtwork(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T", Artist: "A", Album: "B"}, 100, 0, true)
	c.SetArtwork([]byte("PNGBYTES"))
	m := New(c, stubRenderer{out: "ANSI"})

	np := domain.NowPlaying{Track: domain.Track{Title: "T", Artist: "A", Album: "B"}, IsPlaying: true}
	updated, cmd := m.Update(statusMsg{now: np})
	got := updated.(Model)

	if got.art.key != "T|A|B" {
		t.Errorf("art.key = %q; want T|A|B", got.art.key)
	}
	if !got.art.fetching {
		t.Error("art.fetching = false; want true")
	}
	if cmd == nil {
		t.Fatal("expected a fetchArtwork Cmd")
	}
}

func TestStatusMsgWithSameTrackDoesNotRefireFetchArtwork(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	c.SetArtwork([]byte("PNGBYTES"))
	m := New(c, stubRenderer{out: "ANSI"})

	// Pre-seed the art slot with the same key, simulating a previous fetch landing.
	m.art = artState{key: "T||", output: "ANSI", fetching: false}

	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}
	updated, _ := m.Update(statusMsg{now: np})
	got := updated.(Model)

	// art slot must be unchanged — same key, no new fetch.
	if got.art.key != "T||" {
		t.Errorf("art.key changed unexpectedly to %q", got.art.key)
	}
	if got.art.fetching {
		t.Error("art.fetching = true; expected no new fetch")
	}
}

func TestStatusMsgFiresNothingWhenRendererNil(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	m := New(c, nil) // no renderer ⇒ no art fetches ever

	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}
	updated, _ := m.Update(statusMsg{now: np})
	got := updated.(Model)

	if got.art.key != "" {
		t.Errorf("art.key = %q; want empty (renderer nil)", got.art.key)
	}
	if got.art.fetching {
		t.Error("art.fetching = true; want false (renderer nil)")
	}
}

func TestStatusMsgWithEmptyTrackDoesNotFireFetchArtwork(t *testing.T) {
	c := fake.New()
	m := New(c, stubRenderer{out: "ANSI"})

	// statusMsg with all-zero Track (e.g. transitional)
	np := domain.NowPlaying{Track: domain.Track{}}
	updated, _ := m.Update(statusMsg{now: np})
	got := updated.(Model)

	if got.art.fetching {
		t.Error("art.fetching = true; want false (empty track)")
	}
}

func TestArtworkMsgStoresOutputForCurrentTrack(t *testing.T) {
	c := fake.New()
	m := New(c, stubRenderer{})
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T", Artist: "A", Album: "B"}}}
	m.art = artState{key: "T|A|B", fetching: true}

	updated, _ := m.Update(artworkMsg{key: "T|A|B", output: "ANSI"})
	got := updated.(Model)

	if got.art.output != "ANSI" {
		t.Errorf("art.output = %q; want ANSI", got.art.output)
	}
	if got.art.fetching {
		t.Error("art.fetching = true; want cleared")
	}
}

func TestArtworkMsgWithStaleKeyDiscarded(t *testing.T) {
	c := fake.New()
	m := New(c, stubRenderer{})
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "C"}}}
	m.art = artState{key: "C||", fetching: true}

	// Stale message from an older fetch on track A
	updated, _ := m.Update(artworkMsg{key: "A||", output: "STALE"})
	got := updated.(Model)

	// art slot must be unchanged
	if got.art.key != "C||" {
		t.Errorf("art.key changed to %q", got.art.key)
	}
	if got.art.output != "" {
		t.Errorf("art.output = %q; want empty (stale ignored)", got.art.output)
	}
	if !got.art.fetching {
		t.Error("art.fetching cleared by stale message; want still in flight")
	}
}

func TestArtworkMsgWithErrorClearsFetchingButLeavesOutputEmpty(t *testing.T) {
	c := fake.New()
	m := New(c, stubRenderer{})
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T"}}}
	m.art = artState{key: "T||", fetching: true}

	updated, _ := m.Update(artworkMsg{key: "T||", err: music.ErrNoArtwork})
	got := updated.(Model)

	if got.art.fetching {
		t.Error("art.fetching = true; want cleared after error")
	}
	if got.art.output != "" {
		t.Errorf("art.output = %q; want empty after error", got.art.output)
	}
	if got.art.key != "T||" {
		t.Errorf("art.key = %q; want preserved as T||", got.art.key)
	}
}

func TestOKeyFocusesOutputPanelAndDispatchesFetch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	m.output.loading = false // simulate post-startup-fetch state — eager fetch finished without populating
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)
	if got.focus != focusOutput {
		t.Errorf("focusZ = %v; want focusOutput", got.focus)
	}
	if cmd == nil {
		t.Fatal("expected fetchDevices Cmd")
	}
	if _, ok := cmd().(devicesMsg); !ok {
		t.Fatalf("cmd produced %T; want devicesMsg", cmd())
	}
}

func TestOKeyIsNoOpInDisconnected(t *testing.T) {
	m := newTestModel() // starts in Disconnected
	before := m.focus
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)
	if got.focus != before {
		t.Errorf("focusZ changed to %v; want no change (suppressed in Disconnected)", got.focus)
	}
	if cmd != nil {
		t.Errorf("expected no Cmd in Disconnected, got %T", cmd)
	}
}

func TestTabAdvancesFocusFromPlaylistsToSearch(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.focus != focusSearch {
		t.Errorf("focusZ after Tab = %v; want focusSearch", got.focus)
	}
}

func TestShiftTabReversesFocus(t *testing.T) {
	m := newTestModel()
	m.focus = focusOutput
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := updated.(Model)
	if got.focus != focusSearch {
		t.Errorf("focusZ after Shift-Tab from Output = %v; want focusSearch", got.focus)
	}
}

func TestNumberKeysJumpDirectlyToFocus(t *testing.T) {
	tests := []struct {
		key  rune
		want focusKind
	}{
		{'1', focusPlaylists},
		{'2', focusSearch},
		{'3', focusOutput},
		{'4', focusMain},
	}
	for _, tt := range tests {
		m := newTestModel()
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		got := updated.(Model)
		if got.focus != tt.want {
			t.Errorf("focusZ after '%c' = %v; want %v", tt.key, got.focus, tt.want)
		}
	}
}

func TestFocusingPlaylistsFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	m := New(c, nil)
	m.playlists.loading = false // simulate post-startup-fetch state — eager fetch finished without populating
	// focus starts at focusPlaylists by default; we force a transition to
	// trigger the on-focus fetch.
	m.focus = focusSearch
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focus != focusPlaylists {
		t.Fatalf("focusZ = %v; want focusPlaylists", got.focus)
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylists Cmd on focus")
	}
	out := cmd()
	if _, ok := out.(playlistsMsg); !ok {
		t.Fatalf("cmd produced %T; want playlistsMsg", out)
	}
}

func TestFocusingPlaylistsDoesNotRefetchWhenCached(t *testing.T) {
	m := newTestModel()
	m.playlists.items = []domain.Playlist{{Name: "Liked Songs"}}
	m.focus = focusSearch
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when playlists already cached, got %T", cmd())
	}
}

func TestPlaylistsMsgPopulatesPanelStateOnSuccess(t *testing.T) {
	m := newTestModel()
	m.playlists.loading = true
	pls := []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(playlistsMsg{playlists: pls})
	got := updated.(Model)
	if got.playlists.loading {
		t.Error("loading should be cleared on success")
	}
	if len(got.playlists.items) != 2 {
		t.Errorf("items = %d entries; want 2", len(got.playlists.items))
	}
}

func TestPlaylistsMsgClearsLoadingOnError(t *testing.T) {
	m := newTestModel()
	m.playlists.loading = true
	updated, cmd := m.Update(playlistsMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.playlists.loading {
		t.Error("loading should be cleared even on error")
	}
	if got.lastError == nil {
		t.Error("expected lastError set on list-fetch error")
	}
	if len(got.playlists.items) != 0 {
		t.Errorf("items should not be populated on error, got %d", len(got.playlists.items))
	}
	if cmd == nil {
		t.Fatal("expected clearErrorAfter Cmd")
	}
}

func TestPlaylistsMsgClampsCursorWhenResultShorter(t *testing.T) {
	m := newTestModel()
	m.playlists.cursor = 5
	pls := []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(playlistsMsg{playlists: pls})
	got := updated.(Model)
	if got.playlists.cursor != 0 {
		t.Errorf("cursor should clamp to 0 when result shorter than current cursor, got %d", got.playlists.cursor)
	}
}

func TestSlashKeyFocusesSearchAndEntersInputMode(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focus != focusSearch {
		t.Errorf("focusZ = %v; want focusSearch", got.focus)
	}
	if !got.search.inputMode {
		t.Error("expected inputMode true")
	}
}

func TestSlashIsNoOpInDisconnected(t *testing.T) {
	m := newTestModel() // starts in Disconnected
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focus != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (no change in Disconnected)", got.focus)
	}
}

func TestStatusMsgDispatchesQueuePrefetchWhenPlaylistUncached(t *testing.T) {
	m := newTestModel()
	// Pre-set lastVolume so handleStatus has a sensible state.
	m.lastVolume = 50
	np := domain.NowPlaying{
		Track:               domain.Track{Title: "T", PersistentID: "PID-CUR"},
		Volume:              50,
		IsPlaying:           true,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: "Recents",
	}
	updated, cmd := m.Update(statusMsg{now: np})
	got := updated.(Model)
	if !got.playlists.fetchingFor["Recents"] {
		t.Errorf("fetchingFor[Recents] = false; want true")
	}
	if cmd == nil {
		t.Fatal("expected a Cmd, got nil")
	}
	// Configure the fake to return tracks for Recents so the Cmd's downstream
	// PlaylistTracks call succeeds and the message is observable.
	got.client.(*fake.Client).Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	got.client.(*fake.Client).SetPlaylistTracks("Recents", []domain.Track{
		{Title: "X", PersistentID: "PID-X"},
	})
	msg := cmd()
	pmsg, ok := msg.(playlistTracksMsg)
	if !ok {
		t.Fatalf("Cmd produced %T; want playlistTracksMsg", msg)
	}
	if pmsg.name != "Recents" {
		t.Errorf("playlistTracksMsg.name = %q; want Recents", pmsg.name)
	}
}

func TestStatusMsgDoesNotDispatchWhenPlaylistAlreadyCached(t *testing.T) {
	m := newTestModel()
	m.playlists.tracksByName["Recents"] = []domain.Track{{Title: "X"}}
	np := domain.NowPlaying{
		Track:               domain.Track{Title: "T"},
		Volume:              50,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: "Recents",
	}
	_, cmd := m.Update(statusMsg{now: np})
	if cmd != nil {
		// Allow only an artwork cmd (tests use renderer == nil so artwork is skipped).
		t.Errorf("expected nil Cmd (no prefetch, no artwork), got %T", cmd)
	}
}

func TestStatusMsgDoesNotDispatchWhenAlreadyFetching(t *testing.T) {
	m := newTestModel()
	m.playlists.fetchingFor["Recents"] = true
	np := domain.NowPlaying{
		Track:               domain.Track{Title: "T"},
		Volume:              50,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: "Recents",
	}
	_, cmd := m.Update(statusMsg{now: np})
	if cmd != nil {
		t.Errorf("expected nil Cmd; got %T", cmd)
	}
}

func TestStatusMsgDoesNotDispatchWhenNoPlaylistContext(t *testing.T) {
	m := newTestModel()
	np := domain.NowPlaying{
		Track:               domain.Track{Title: "T"},
		Volume:              50,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: "",
	}
	_, cmd := m.Update(statusMsg{now: np})
	if cmd != nil {
		t.Errorf("expected nil Cmd; got %T", cmd)
	}
}

// A previous fetch failure is recorded in trackErrByName. Without this guard
// the dispatch fires every status tick (every second), kicking off osascript
// processes that all time out — pathological loop.
func TestStatusMsgDoesNotRetryAfterPreviousFetchError(t *testing.T) {
	m := newTestModel()
	m.playlists.trackErrByName["Recents"] = errors.New("signal: killed")
	np := domain.NowPlaying{
		Track:               domain.Track{Title: "T"},
		Volume:              50,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: "Recents",
	}
	_, cmd := m.Update(statusMsg{now: np})
	if cmd != nil {
		t.Errorf("expected nil Cmd; got %T (must not retry an errored playlist on every tick)", cmd)
	}
}

func TestKeyAEnqueuesFocusedMainTrack(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	// Focus Main with one search-result row that has a PID.
	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "Hotel California", PersistentID: "HC1"}}
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 1 {
		t.Fatalf("queue.Len = %d; want 1", got.queue.Len())
	}
	if got.queue.Items[0].PersistentID != "HC1" {
		t.Errorf("enqueued PID = %q; want HC1", got.queue.Items[0].PersistentID)
	}
}

func TestKeyAOnEmptyPIDRefusesAndSetsError(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "NoPID"}} // empty PID
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (refused)", got.queue.Len())
	}
	if got.lastError == nil || !errors.Is(got.lastError, ErrNoPersistentID) {
		t.Errorf("lastError = %v; want ErrNoPersistentID", got.lastError)
	}
}

func TestKeyAOnNonMainFocusIsNoOp(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusPlaylists // not Main

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (no-op when focus != Main)", got.queue.Len())
	}
}

func TestKeyAOnMainWithEmptyRowsIsNoOp(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = nil
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0", got.queue.Len())
	}
	if got.lastError != nil {
		t.Errorf("lastError = %v; want nil (no-op silently)", got.lastError)
	}
}

func TestHandleStatusInvokesHandoffAndRefreshesLastFields(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
	})
	c.SetLibraryTracks([]domain.Track{
		{Title: "Hotel California", PersistentID: "HC"},
	})
	m := New(c, nil)
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
	}
	// Prime previous-tick state to ST in LZ at index 2.
	m.lastTrackPID = "ST"
	m.lastPlaylist = "LZ"
	m.lastTrackIdx = 2
	// Queue Hotel California so the next tick triggers intercept.
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})

	// Status tick: Music.app moved to the next playlist track (BD again
	// won't do — pick a different PID so it's a real change).
	now := domain.NowPlaying{
		Track:               domain.Track{Title: "End", PersistentID: "END"},
		Volume:              50,
		IsPlaying:           true,
		CurrentPlaylistName: "LZ",
	}

	updated, cmd := m.Update(statusMsg{now: now})
	got := updated.(Model)

	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (intercept popped head)", got.queue.Len())
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3}", got.resume)
	}
	if got.lastTrackPID != "END" {
		t.Errorf("lastTrackPID = %q; want END", got.lastTrackPID)
	}
	if got.lastPlaylist != "LZ" {
		t.Errorf("lastPlaylist = %q; want LZ", got.lastPlaylist)
	}
	// END isn't in the cached LZ tracks, so lastTrackIdx is 0.
	if got.lastTrackIdx != 0 {
		t.Errorf("lastTrackIdx = %d; want 0", got.lastTrackIdx)
	}
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	// The intercept Cmd should produce a PlayTrack call when invoked.
	_ = cmd() // may be a tea.Batch wrapper — call it to flush
	rec := c.PlayTrackRecord()
	if len(rec) != 1 || rec[0].PersistentID != "HC" {
		t.Errorf("PlayTrack record = %v; want [{HC}]", rec)
	}
}

func TestHandleStatusRefreshesLastFieldsOnNormalTick(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{{Title: "Stairway", PersistentID: "ST"}})
	m := New(c, nil)
	m.playlists.tracksByName["LZ"] = []domain.Track{{Title: "Stairway", PersistentID: "ST"}}

	now := domain.NowPlaying{
		Track:               domain.Track{Title: "Stairway", PersistentID: "ST"},
		Volume:              50,
		IsPlaying:           true,
		CurrentPlaylistName: "LZ",
	}
	updated, _ := m.Update(statusMsg{now: now})
	got := updated.(Model)

	if got.lastTrackPID != "ST" {
		t.Errorf("lastTrackPID = %q; want ST", got.lastTrackPID)
	}
	if got.lastPlaylist != "LZ" {
		t.Errorf("lastPlaylist = %q; want LZ", got.lastPlaylist)
	}
	if got.lastTrackIdx != 1 {
		t.Errorf("lastTrackIdx = %d; want 1", got.lastTrackIdx)
	}
}

func TestKeyQOpensOverlay(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	got := updated.(Model)
	if !got.overlay.open {
		t.Fatal("overlay.open false after Q")
	}
	if got.overlay.cursor != 0 {
		t.Errorf("cursor = %d; want 0 on fresh open", got.overlay.cursor)
	}
}

func TestKeyQuitSuppressedWhileOverlayOpen(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Errorf("q while overlay open returned cmd %v; want nil (suppressed quit)", cmd)
	}
}

func TestKeysRouteToOverlayWhenOpen(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "A", PersistentID: "A1"})
	m.queue.Add(domain.Track{Title: "B", PersistentID: "B1"})

	// 'j' should move overlay cursor, not switch focus.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.overlay.cursor != 1 {
		t.Errorf("overlay cursor = %d; want 1", got.overlay.cursor)
	}
}

func TestKeyNWithEmptyQueueCallsNext(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.NextCalls != 1 {
		t.Errorf("Next calls = %d; want 1 (empty queue path)", c.NextCalls)
	}
	if c.PlayTrackCalls != 0 {
		t.Errorf("PlayTrack calls = %d; want 0 (empty queue)", c.PlayTrackCalls)
	}
}

func TestKeyNWithQueueCallsPlayTrackAndPops(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{
		{Title: "HC", PersistentID: "HC"},
	})
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.NextCalls != 0 {
		t.Errorf("Next calls = %d; want 0 (queue path should not call Next)", c.NextCalls)
	}
	if c.PlayTrackCalls != 1 {
		t.Errorf("PlayTrack calls = %d; want 1", c.PlayTrackCalls)
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (head popped)", got.queue.Len())
	}
	if got.pendingJumpPID != "HC" {
		t.Errorf("pendingJumpPID = %q; want HC (n is a user-driven jump)", got.pendingJumpPID)
	}
}
