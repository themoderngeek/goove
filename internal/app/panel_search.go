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
	return panelBox(title, body, width, height, m.focus == focusSearch)
}

func renderSearchBody(m Model) string {
	switch {
	case m.search.inputMode && m.search.err != nil:
		// Error during the most recent search; query preserved so user can
		// press Enter to retry.
		return titleStyle.Render("/"+m.search.query+"_") + "\n" + errorStyle.Render("error: "+m.search.err.Error())
	case m.search.inputMode && m.search.loading:
		return titleStyle.Render("/"+m.search.query) + "\n" + subtitleStyle.Render("searching…")
	case m.search.inputMode:
		// Caret at end of query.
		return titleStyle.Render("/" + m.search.query + "_")
	case m.search.lastQuery != "":
		hits := fmt.Sprintf("%d results", m.search.total)
		if m.search.total > len(m.main.searchResults) {
			hits = fmt.Sprintf("%d of %d", len(m.main.searchResults), m.search.total)
		}
		return titleStyle.Render("/"+m.search.lastQuery) + "\n" + subtitleStyle.Render(hits)
	default:
		return subtitleStyle.Render("/  type to search")
	}
}

// handleSearchPanelKey routes keys when focus == focusSearch. Returns
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
		m.search.inputMode = false
		m.search.query = ""
		m.search.seq++
		m.search.err = nil
		return m, nil, true
	case tea.KeyBackspace:
		if !m.search.inputMode {
			return m, nil, true
		}
		runes := []rune(m.search.query)
		if len(runes) > 0 {
			m.search.query = string(runes[:len(runes)-1])
			m.search.seq++
		}
		return m, nil, true
	case tea.KeySpace:
		m.search.inputMode = true
		m.search.query += " "
		m.search.seq++
		return m, nil, true
	case tea.KeyRunes:
		m.search.inputMode = true
		m.search.query += string(msg.Runes)
		m.search.seq++
		return m, nil, true
	case tea.KeyEnter:
		if !m.search.inputMode || m.search.query == "" {
			return m, nil, true
		}
		m.search.seq++
		m.search.loading = true
		m.search.err = nil
		return m, fireSearchPanel(m.client, m.search.seq, m.search.query), true
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
