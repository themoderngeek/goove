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

func TestOKeyOpensPickerInConnected(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	m := New(c, nil)
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T"}}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)

	if got.picker == nil {
		t.Fatal("expected picker to be open after 'o' keypress")
	}
	if !got.picker.loading {
		t.Error("expected picker.loading = true while fetch is in flight")
	}
	if cmd == nil {
		t.Error("expected a fetchDevices Cmd")
	}
}

func TestOKeyOpensPickerInIdle(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.state = Idle{Volume: 50}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker == nil {
		t.Fatal("expected picker to be open in Idle state")
	}
}

func TestOKeyIsNoOpInDisconnected(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	// Default state is Disconnected{}.

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil (suppressed in Disconnected)", updated.(Model).picker)
	}
}

func TestOKeyIsNoOpWhenPermissionDenied(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.permissionDenied = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil (suppressed when permissionDenied)", updated.(Model).picker)
	}
}

func TestPickerArrowsNavigateCursor(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{
			{Name: "A"}, {Name: "B"}, {Name: "C"},
		},
		cursor: 0,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).picker.cursor != 1 {
		t.Errorf("cursor after down = %d; want 1", updated.(Model).picker.cursor)
	}

	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor after up = %d; want 0", updated.(Model).picker.cursor)
	}
}

func TestPickerArrowsClampAtBoundaries(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}
	// Up at top — stays at 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor at top after up = %d; want 0", updated.(Model).picker.cursor)
	}
	// Down past bottom — clamps to last
	m = updated.(Model)
	for range 5 {
		tmp, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = tmp.(Model)
	}
	if m.picker.cursor != 1 {
		t.Errorf("cursor after spam-down = %d; want 1 (clamped)", m.picker.cursor)
	}
}

func TestPickerVIKeysAlsoNavigate(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.(Model).picker.cursor != 1 {
		t.Errorf("cursor after j = %d; want 1", updated.(Model).picker.cursor)
	}
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor after k = %d; want 0", updated.(Model).picker.cursor)
	}
}

func TestPickerEscClosesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after esc", updated.(Model).picker)
	}
}

func TestPickerQAlsoCloses(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after q", updated.(Model).picker)
	}
}

func TestPickerEnterTriggersSetAirPlayDevice(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{
			{Name: "Computer", Selected: true},
			{Name: "Kitchen Sonos"},
		},
		cursor: 1, // pointing at Kitchen Sonos
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.picker.loading {
		t.Error("expected loading=true after enter")
	}
	if cmd == nil {
		t.Fatal("expected a Cmd from enter")
	}
	out := cmd()
	dsm, ok := out.(deviceSetMsg)
	if !ok {
		t.Fatalf("cmd returned %T; want deviceSetMsg", out)
	}
	if dsm.err != nil {
		t.Errorf("deviceSetMsg.err = %v; want nil", dsm.err)
	}
}

func TestPickerWhileLoadingOnlyEscWorks(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}

	// Down should be ignored.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor moved while loading = %d; want 0", updated.(Model).picker.cursor)
	}
	// Esc still closes.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).picker != nil {
		t.Error("esc did not close picker while loading")
	}
}

func TestTransportKeysSuppressedWhilePickerOpen(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.PlayPauseCalls != 0 {
		t.Errorf("PlayPauseCalls = %d; want 0 (suppressed by picker)", c.PlayPauseCalls)
	}
	// Picker still open after the suppressed key.
	if updated.(Model).picker == nil {
		t.Error("picker closed unexpectedly")
	}
}

func TestDevicesMsgPopulatesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{loading: true}

	devices := []domain.AudioDevice{
		{Name: "Computer", Selected: false},
		{Name: "Kitchen Sonos", Selected: true},
	}
	updated, _ := m.Update(devicesMsg{devices: devices, err: nil})
	got := updated.(Model)

	if got.picker.loading {
		t.Error("loading still true after devicesMsg")
	}
	if len(got.picker.devices) != 2 {
		t.Errorf("len = %d; want 2", len(got.picker.devices))
	}
	// Cursor should land on the currently-selected device.
	if got.picker.cursor != 1 {
		t.Errorf("cursor = %d; want 1 (Kitchen Sonos has Selected=true)", got.picker.cursor)
	}
}

func TestDevicesMsgErrorShownInPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{loading: true}

	updated, _ := m.Update(devicesMsg{err: music.ErrUnavailable})
	got := updated.(Model)

	if got.picker.loading {
		t.Error("loading still true after error devicesMsg")
	}
	if got.picker.err == nil {
		t.Error("expected picker.err set")
	}
}

func TestDevicesMsgPopulatesOutputPanelWhenPickerClosed(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(devicesMsg{devices: []domain.AudioDevice{
		{Name: "MacBook", Selected: true}, {Name: "Sonos"},
	}})
	got := updated.(Model)
	if got.picker != nil {
		t.Errorf("picker should remain nil; got %+v", got.picker)
	}
	if len(got.output.devices) != 2 {
		t.Errorf("output.devices = %d; want 2 (panel populates even with no modal open)", len(got.output.devices))
	}
	if got.output.cursor != 0 {
		t.Errorf("output.cursor = %d; want 0 (lands on selected)", got.output.cursor)
	}
}

func TestDeviceSetMsgSuccessClosesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}},
	}

	updated, _ := m.Update(deviceSetMsg{err: nil})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after successful set", updated.(Model).picker)
	}
}

func TestDeviceSetMsgErrorKeepsPickerOpen(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  1,
	}

	updated, _ := m.Update(deviceSetMsg{err: music.ErrDeviceNotFound})
	got := updated.(Model)

	if got.picker == nil {
		t.Fatal("picker closed on error; want it to stay open")
	}
	if got.picker.loading {
		t.Error("loading still true after error deviceSetMsg")
	}
	if got.picker.err == nil {
		t.Error("expected picker.err set")
	}
	if got.picker.cursor != 1 {
		t.Errorf("cursor changed unexpectedly to %d", got.picker.cursor)
	}
}

func TestDeviceSetMsgIgnoredWhenPickerClosed(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	// picker is nil — user esc'd before set landed.

	updated, _ := m.Update(deviceSetMsg{err: nil})
	if updated.(Model).picker != nil {
		t.Error("picker should remain nil")
	}
}

func TestTabAdvancesFocusFromPlaylistsToSearch(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ after Tab = %v; want focusSearch", got.focusZ)
	}
}

func TestShiftTabReversesFocus(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusOutput
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ after Shift-Tab from Output = %v; want focusSearch", got.focusZ)
	}
}

func TestNumberKeysJumpDirectlyToFocus(t *testing.T) {
	tests := []struct {
		key  rune
		want focus
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
		if got.focusZ != tt.want {
			t.Errorf("focusZ after '%c' = %v; want %v", tt.key, got.focusZ, tt.want)
		}
	}
}

func TestFocusKeysSuppressedWhilePickerOpen(t *testing.T) {
	m := newTestModel()
	m.picker = &pickerState{}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ after Tab while picker open = %v; want focusPlaylists (no change)", got.focusZ)
	}
}

func TestFocusingPlaylistsFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(nil)
	m := New(c, nil)
	// focusZ starts at focusPlaylists by default; we force a transition to
	// trigger the on-focus fetch.
	m.focusZ = focusSearch
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Fatalf("focusZ = %v; want focusPlaylists", got.focusZ)
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
	m.focusZ = focusSearch
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
	if got.playlists.err != nil {
		t.Errorf("err should be nil on success, got %v", got.playlists.err)
	}
	if len(got.playlists.items) != 2 {
		t.Errorf("items = %d entries; want 2", len(got.playlists.items))
	}
}

func TestPlaylistsMsgClearsLoadingOnError(t *testing.T) {
	m := newTestModel()
	m.playlists.loading = true
	updated, _ := m.Update(playlistsMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.playlists.loading {
		t.Error("loading should be cleared even on error")
	}
	if got.playlists.err == nil {
		t.Error("expected err set")
	}
	if len(got.playlists.items) != 0 {
		t.Errorf("items should not be populated on error, got %d", len(got.playlists.items))
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
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ = %v; want focusSearch", got.focusZ)
	}
	if !got.search2.inputMode {
		t.Error("expected inputMode true")
	}
}

func TestSlashIsNoOpInDisconnected(t *testing.T) {
	m := newTestModel() // starts in Disconnected
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (no change in Disconnected)", got.focusZ)
	}
}
