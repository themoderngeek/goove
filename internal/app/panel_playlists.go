package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderPlaylistsPanel renders the Playlists panel (left, top of stack).
func renderPlaylistsPanel(m Model, width, height int) string {
	title := "Playlists"
	body := renderPlaylistsBody(m, width, height)
	return panelBox(title, body, width, height, m.focusZ == focusPlaylists)
}

func renderPlaylistsBody(m Model, width, height int) string {
	if m.playlists.loading && len(m.playlists.items) == 0 {
		return subtitleStyle.Render("loading…")
	}
	if m.playlists.err != nil {
		return errorStyle.Render("error: " + m.playlists.err.Error())
	}
	if len(m.playlists.items) == 0 {
		return subtitleStyle.Render("(no playlists)")
	}
	visibleRows := height - 4 // top border + bottom border + title row + title/body separator
	if visibleRows < 1 {
		visibleRows = 1
	}
	start := scrollWindow(m.playlists.cursor, visibleRows, len(m.playlists.items))

	var sb strings.Builder
	for i := start; i < len(m.playlists.items) && i-start < visibleRows; i++ {
		marker := "  "
		if i == m.playlists.cursor && m.focusZ == focusPlaylists {
			marker = "▶ "
		}
		row := marker + m.playlists.items[i].Name
		sb.WriteString(truncate(row, width-4))
		if i-start < visibleRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// onFocusPlaylists is called by handleKey whenever focus transitions TO the
// Playlists panel. Returns a fetchPlaylists Cmd if the list isn't cached yet,
// or nil. Idempotent on repeat focuses (cache hit ⇒ no Cmd).
func onFocusPlaylists(m Model) (Model, tea.Cmd) {
	if len(m.playlists.items) > 0 || m.playlists.loading {
		return m, nil
	}
	m.playlists.loading = true
	return m, fetchPlaylists(m.client)
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
