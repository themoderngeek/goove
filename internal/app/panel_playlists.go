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

// handlePlaylistsKey routes keys when focusZ == focusPlaylists. Returns
// (model, cmd, handled). When handled is false, the caller falls through to
// globals.
//
// onPlaylistsCursorChanged is intentionally inside the cursor-move guards —
// when the cursor is clamped at a boundary, no side effects fire. We don't
// want to refetch when the user presses j repeatedly at the bottom of the
// list.
func handlePlaylistsKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if m.playlists.cursor > 0 {
			m.playlists.cursor--
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	case "down", "j":
		if m.playlists.cursor < len(m.playlists.items)-1 {
			m.playlists.cursor++
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	case "enter":
		if len(m.playlists.items) == 0 {
			return m, nil, true
		}
		name := m.playlists.items[m.playlists.cursor].Name
		return m, playPlaylist(m.client, name, 0), true
	}
	return m, nil, false
}

// onPlaylistsCursorChanged keeps the main pane's "selected playlist" pointer
// in sync with the Playlists cursor (live preview, Q3-C). When the main pane
// is in tracks mode, the cursor also resets to the top of the new list.
//
// We deliberately do NOT clobber main.mode or main.cursor when the pane is
// in mainPaneSearchResults mode — the user's search results are sticky until
// they dismiss them (Esc in main pane, Task 21). selectedPlaylist still
// updates so that Esc lands on the currently-cursor'd playlist's tracks.
//
// On first preview of a playlist, schedules a debounce tick. Rapid cursor
// movements bump seq; only the most recent tick's fetch survives. Cached and
// in-flight selections short-circuit before scheduling.
func onPlaylistsCursorChanged(m Model) (Model, tea.Cmd) {
	if len(m.playlists.items) == 0 {
		return m, nil
	}
	name := m.playlists.items[m.playlists.cursor].Name
	m.main.selectedPlaylist = name
	if m.main.mode == mainPaneTracks {
		m.main.cursor = 0
	}

	if _, cached := m.playlists.tracksByName[name]; cached {
		return m, nil
	}
	if m.playlists.fetchingFor[name] {
		return m, nil
	}
	m.playlists.seq++
	return m, schedulePlaylistTracksDebounce(m.playlists.seq, name)
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
