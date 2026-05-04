package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

func renderSearchPanel(m Model, width, height int) string {
	title := "Search"
	body := subtitleStyle.Render("/  type to search")
	return panelBox(title, body, width, height, m.focusZ == focusSearch)
}

// handleSearchPanelKey routes keys when focusZ == focusSearch. Returns
// (model, cmd, handled). Number keys 1–4 are NOT handled here so they fall
// through to the global focus-jump cases (one of the disambiguation rules in
// the spec). Tab/Shift-Tab also fall through.
func handleSearchPanelKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	// Always pass-through: focus controls.
	switch msg.String() {
	case "tab", "shift+tab", "1", "2", "3", "4":
		return m, nil, false
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.search2.inputMode = false
		m.search2.query = ""
		m.search2.seq++
		m.search2.err = nil
		return m, nil, true
	case tea.KeyBackspace:
		if !m.search2.inputMode {
			return m, nil, true
		}
		runes := []rune(m.search2.query)
		if len(runes) > 0 {
			m.search2.query = string(runes[:len(runes)-1])
			m.search2.seq++
		}
		return m, nil, true
	case tea.KeySpace:
		m.search2.inputMode = true
		m.search2.query += " "
		m.search2.seq++
		return m, nil, true
	case tea.KeyRunes:
		m.search2.inputMode = true
		m.search2.query += string(msg.Runes)
		m.search2.seq++
		return m, nil, true
	case tea.KeyEnter:
		// Phase 3 task 20 wires this.
		return m, nil, true
	}
	return m, nil, false
}
