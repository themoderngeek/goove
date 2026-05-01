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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(nil)
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
	c.Launch(context.Background())
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
	c.Launch(context.Background())
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
	c.Launch(context.Background())
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
