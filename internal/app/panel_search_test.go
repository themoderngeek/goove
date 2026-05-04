package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchPanelTypingEntersInputModeAndAppendsQuery(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := updated.(Model)
	if !got.search2.inputMode {
		t.Error("expected inputMode true after typing")
	}
	if got.search2.query != "l" {
		t.Errorf("query = %q; want %q", got.search2.query, "l")
	}
}

func TestSearchPanelMultipleKeysAppend(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	keys := []rune{'l', 'e', 'd'}
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = updated.(Model)
	}
	if m.search2.query != "led" {
		t.Errorf("query = %q; want %q", m.search2.query, "led")
	}
}

func TestSearchPanelBackspaceRemovesLastRune(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := updated.(Model)
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q", got.search2.query, "le")
	}
}

func TestSearchPanelEscClearsAndExitsInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.search2.inputMode {
		t.Error("expected inputMode false after esc")
	}
	if got.search2.query != "" {
		t.Errorf("query = %q; want empty", got.search2.query)
	}
}

func TestSearchPanelSpaceGoesIntoQuery(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.search2.query != "led " {
		t.Errorf("query = %q; want %q", got.search2.query, "led ")
	}
}

func TestSearchPanelNumberKeysStillJumpFocusInInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "le"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (1 always wins)", got.focusZ)
	}
	// Query unchanged.
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q (1 should not append)", got.search2.query, "le")
	}
}
