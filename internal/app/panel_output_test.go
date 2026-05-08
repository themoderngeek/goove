package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestFocusingOutputFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	m := New(c, nil)
	m.output.loading = false // simulate post-startup-fetch state — eager fetch finished without populating
	m.focus = focusPlaylists
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	got := updated.(Model)
	if got.focus != focusOutput {
		t.Fatalf("focusZ = %v; want focusOutput", got.focus)
	}
	if cmd == nil {
		t.Fatal("expected fetchDevices Cmd")
	}
	if _, ok := cmd().(devicesMsg); !ok {
		t.Fatalf("cmd produced %T; want devicesMsg", cmd())
	}
}

func TestFocusingOutputDoesNotRefetchWhenCached(t *testing.T) {
	m := newTestModel()
	m.output.devices = []domain.AudioDevice{{Name: "MacBook"}}
	m.focus = focusPlaylists
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when devices cached, got %T", cmd())
	}
}

func TestDevicesMsgPopulatesOutputPanel(t *testing.T) {
	m := newTestModel()
	m.output.loading = true
	updated, _ := m.Update(devicesMsg{devices: []domain.AudioDevice{
		{Name: "MacBook", Selected: true}, {Name: "Sonos"},
	}})
	got := updated.(Model)
	if len(got.output.devices) != 2 {
		t.Errorf("devices = %d; want 2", len(got.output.devices))
	}
	if got.output.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (lands on selected)", got.output.cursor)
	}
}

func TestOutputCursorMovesWithJK(t *testing.T) {
	m := newTestModel()
	m.focus = focusOutput
	m.output.devices = []domain.AudioDevice{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.output.cursor != 1 {
		t.Errorf("cursor = %d; want 1", got.output.cursor)
	}
}

func TestOutputEnterFiresSetAirPlayDevice(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) //nolint:errcheck // fake.Client.Launch cannot fail
	c.SetDevices([]domain.AudioDevice{
		{Name: "MacBook", Selected: true},
		{Name: "Sonos"},
	})
	m := New(c, nil)
	m.focus = focusOutput
	m.output.devices = []domain.AudioDevice{
		{Name: "MacBook", Selected: true},
		{Name: "Sonos"},
	}
	m.output.cursor = 1 // Sonos
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected SetAirPlayDevice Cmd on enter")
	}
	got := updated.(Model)
	if !got.output.loading {
		t.Error("expected output.loading = true while the set-device call is in flight")
	}
	out := cmd()
	if _, ok := out.(deviceSetMsg); !ok {
		t.Fatalf("cmd produced %T; want deviceSetMsg", out)
	}
	// Verify the fake actually received the SetAirPlayDevice("Sonos") call:
	// after the cmd runs, Sonos should be Selected=true in the fake's state.
	devices, err := c.AirPlayDevices(context.Background())
	if err != nil {
		t.Fatalf("AirPlayDevices: %v", err)
	}
	var sonosSelected bool
	for _, d := range devices {
		if d.Name == "Sonos" && d.Selected {
			sonosSelected = true
		}
	}
	if !sonosSelected {
		t.Error("SetAirPlayDevice was not called for Sonos (Sonos.Selected still false)")
	}
}

func TestOutputEnterIsNoOpWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.focus = focusOutput
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd with empty device list, got %T", cmd())
	}
}

func TestDeviceSetMsgErrorRoutesToLastErrorNotPanel(t *testing.T) {
	m := newTestModel()
	m.output.loading = true
	updated, cmd := m.Update(deviceSetMsg{err: errors.New("device gone")})
	got := updated.(Model)
	if got.lastError == nil {
		t.Error("lastError must be set on device-switch error")
	}
	if got.output.loading {
		t.Error("output.loading must be cleared")
	}
	if cmd == nil {
		t.Fatal("expected clearErrorAfter Cmd")
	}
}

func TestDeviceSetMsgSuccessRefreshesDevices(t *testing.T) {
	m := newTestModel()
	m.output.loading = true
	updated, cmd := m.Update(deviceSetMsg{})
	got := updated.(Model)
	if got.output.loading {
		t.Error("output.loading must be cleared on success")
	}
	if cmd == nil {
		t.Fatal("expected fetchDevices Cmd on success")
	}
	out := cmd()
	if _, ok := out.(devicesMsg); !ok {
		t.Fatalf("cmd produced %T; want devicesMsg", out)
	}
}

func TestDevicesMsgErrorRoutesToLastErrorNotPanel(t *testing.T) {
	m := newTestModel()
	m.output.loading = true
	updated, cmd := m.Update(devicesMsg{err: errors.New("backend killed")})
	got := updated.(Model)
	if got.lastError == nil {
		t.Error("expected lastError set on devices-fetch error")
	}
	if got.output.loading {
		t.Error("output.loading must be cleared")
	}
	if cmd == nil {
		t.Fatal("expected clearErrorAfter Cmd")
	}
}

func TestNewInitialisesOutputPanelLoading(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	if !m.output.loading {
		t.Error("expected output.loading = true after New so first frame shows 'loading…' instead of an empty panel")
	}
}
