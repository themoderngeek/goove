package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderSearch is the modal overlay shown when m.search != nil.
// Replaces the player view entirely (no side-by-side composition), matching
// the picker pattern.
func renderSearch(s *searchState) string {
	var body strings.Builder
	body.WriteString("> ")
	body.WriteString(s.query)
	body.WriteString("_")
	body.WriteString("\n")
	body.WriteString(strings.Repeat("─", 46))
	body.WriteString("\n\n")

	switch {
	case s.query == "":
		body.WriteString(subtitleStyle.Render("type to search your library"))
	case s.loading:
		body.WriteString(subtitleStyle.Render("searching…"))
	case len(s.results) == 0:
		body.WriteString(subtitleStyle.Render("no matches in your library"))
	default:
		for i, t := range s.results {
			cursor := " "
			if i == s.cursor {
				cursor = "▶"
			}
			body.WriteString(fmt.Sprintf("  %s %s\n", cursor, titleStyle.Render(t.Title)))
			body.WriteString("    ")
			body.WriteString(subtitleStyle.Render(t.Artist + " · " + t.Album))
			if i < len(s.results)-1 {
				body.WriteString("\n\n")
			}
		}
		body.WriteString("\n\n")
		if s.total > len(s.results) {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("… %d of %d — refine the query", len(s.results), s.total)))
		} else {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("%d results", s.total)))
		}
	}

	header := titleStyle.Render("search")
	card := cardStyle.Render(header + "\n\n" + body.String())

	footerText := " ⏎ play   esc cancel"
	if len(s.results) > 0 {
		footerText = " ↑/↓ navigate   ⏎ play   r refresh   esc cancel"
	}
	footer := footerStyle.Render(footerText)

	out := card + "\n" + footer
	if s.err != nil {
		// Override the footer label to "retry" while an error is showing.
		errFooter := errorStyle.Render("error: " + s.err.Error())
		footerText = " ⏎ play   r retry   esc cancel"
		out = card + "\n" + footerStyle.Render(footerText) + "\n" + errFooter
	}
	return lipgloss.NewStyle().Margin(0, 2).Render(out)
}

// handleSearchKey routes keystrokes when the search modal is open. Transport
// keys do NOT fall through (unlike the browser); the modal is fully captive
// the way the picker is. Future tasks will extend this with typing,
// navigation, enter, and r.
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search = nil
		return m, nil
	}
	return m, nil
}
