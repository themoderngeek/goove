package app

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func newTestModel() Model {
	c := fake.New()
	return New(c)
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
	m := New(c)

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
	m := New(c)

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
	m := New(c)
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
	m := New(c)

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
	m := New(c)
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
	m := New(c) // state=Disconnected
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.LaunchCalls != 1 {
		t.Errorf("Launch calls = %d; want 1", c.LaunchCalls)
	}
}
