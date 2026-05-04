package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)

func renderSearchPanel(m Model, width, height int) string {
	title := "Search"
	body := renderSearchBody(m)
	return panelBox(title, body, width, height, m.focusZ == focusSearch)
}

func renderSearchBody(m Model) string {
	switch {
	case m.search2.inputMode && m.search2.err != nil:
		// Error during the most recent search; query preserved so user can
		// press Enter to retry.
		return titleStyle.Render("/"+m.search2.query+"_") + "\n" + errorStyle.Render("error: "+m.search2.err.Error())
	case m.search2.inputMode && m.search2.loading:
		return titleStyle.Render("/"+m.search2.query) + "\n" + subtitleStyle.Render("searching…")
	case m.search2.inputMode:
		// Caret at end of query.
		return titleStyle.Render("/" + m.search2.query + "_")
	case m.search2.lastQuery != "":
		hits := fmt.Sprintf("%d results", m.search2.total)
		if m.search2.total > len(m.main.searchResults) {
			hits = fmt.Sprintf("%d of %d", len(m.main.searchResults), m.search2.total)
		}
		return titleStyle.Render("/"+m.search2.lastQuery) + "\n" + subtitleStyle.Render(hits)
	default:
		return subtitleStyle.Render("/  type to search")
	}
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
		if !m.search2.inputMode || m.search2.query == "" {
			return m, nil, true
		}
		m.search2.seq++
		m.search2.loading = true
		m.search2.err = nil
		return m, fireSearchPanel(m.client, m.search2.seq, m.search2.query), true
	}
	return m, nil, false
}

// fireSearchPanel dispatches a SearchTracks call. Used by the ⏎ handler.
func fireSearchPanel(c music.Client, seq uint64, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.SearchTracks(context.Background(), query)
		return searchPanelResultsMsg{seq: seq, query: query, result: res, err: err}
	}
}
