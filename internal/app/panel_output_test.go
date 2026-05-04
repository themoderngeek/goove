package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestFocusingOutputFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.focusZ = focusPlaylists
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	got := updated.(Model)
	if got.focusZ != focusOutput {
		t.Fatalf("focusZ = %v; want focusOutput", got.focusZ)
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
	m.focusZ = focusPlaylists
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
	m.focusZ = focusOutput
	m.output.devices = []domain.AudioDevice{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.output.cursor != 1 {
		t.Errorf("cursor = %d; want 1", got.output.cursor)
	}
}
