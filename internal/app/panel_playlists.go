package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPlaylistsPanel renders the Playlists panel (left, top of stack).
// Phase 1: placeholder. Phase 2 wires real content.
func renderPlaylistsPanel(m Model, width, height int) string {
	title := "Playlists"
	body := subtitleStyle.Render("—")
	return panelBox(title, body, width, height, m.focusZ == focusPlaylists)
}

// panelBox is the shared lipgloss box used by every left-column panel.
// focused=true draws the border in the focus colour.
func panelBox(title, body string, width, height int, focused bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6b7280")).
		Width(width-2).
		Height(height-2).
		Padding(0, 1)
	if focused {
		style = style.BorderForeground(lipgloss.Color("#ebcb8b"))
	}
	header := titleStyle.Render(title)
	return style.Render(header + "\n" + strings.TrimRight(body, "\n"))
}
