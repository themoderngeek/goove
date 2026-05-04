package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMainPanel renders the right-hand main pane. Phase 1: placeholder.
// Phase 2 fills in mainPaneTracks; Phase 3 fills in mainPaneSearchResults.
func renderMainPanel(m Model, width, height int) string {
	title := "—"
	body := subtitleStyle.Render("focus a panel on the left to see its content")
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

// panelBoxWide is the same as panelBox but for the wider main pane. Identical
// implementation kept separate so future tweaks (e.g. main pane padding) can
// diverge without touching left-column panels.
func panelBoxWide(title, body string, width, height int, focused bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6b7280")).
		Width(width - 2).
		Height(height - 2).
		Padding(0, 1)
	if focused {
		style = style.BorderForeground(lipgloss.Color("#ebcb8b"))
	}
	header := titleStyle.Render(title)
	return style.Render(header + "\n" + strings.TrimRight(body, "\n"))
}
