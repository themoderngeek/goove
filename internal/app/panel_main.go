package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

func renderMainPanel(m Model, width, height int) string {
	switch m.main.mode {
	case mainPaneSearchResults:
		return renderMainSearchResults(m, width, height)
	default:
		return renderMainTracks(m, width, height)
	}
}

func renderMainTracks(m Model, width, height int) string {
	if m.main.selectedPlaylist == "" {
		title := "—"
		body := subtitleStyle.Render("focus a panel on the left to see its content")
		return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
	}
	title := m.main.selectedPlaylist
	if isPlayingFromSelected(m) {
		title += "  (now playing)"
	}

	tracks, cached := m.playlists.tracksByName[m.main.selectedPlaylist]
	var body string
	switch {
	case !cached && m.playlists.fetchingFor[m.main.selectedPlaylist]:
		body = subtitleStyle.Render("loading…")
	case !cached:
		body = subtitleStyle.Render("(no tracks loaded)")
	case len(tracks) == 0:
		body = subtitleStyle.Render("(empty playlist)")
	default:
		body = renderTrackRows(m, tracks, width, height)
	}
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

func renderMainSearchResults(m Model, width, height int) string {
	title := fmt.Sprintf("Search: %q · %d results", "", len(m.main.searchResults))
	if m.search2.lastQuery != "" {
		title = fmt.Sprintf("Search: %q · %d results", m.search2.lastQuery, m.search2.total)
	}
	if len(m.main.searchResults) == 0 {
		body := subtitleStyle.Render("no matches")
		return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
	}
	body := renderTrackRows(m, m.main.searchResults, width, height)
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

// renderTrackRows is shared between mainPaneTracks and mainPaneSearchResults.
func renderTrackRows(m Model, tracks []domain.Track, width, height int) string {
	visibleRows := height - 4
	if visibleRows < 1 {
		visibleRows = 1
	}
	start := scrollWindow(m.main.cursor, visibleRows, len(tracks))

	var sb strings.Builder
	for i := start; i < len(tracks) && i-start < visibleRows; i++ {
		marker := "  "
		if i == m.main.cursor && m.focusZ == focusMain {
			marker = "▶ "
		}
		t := tracks[i]
		row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
		sb.WriteString(truncate(row, width-4))
		if i-start < visibleRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// isPlayingFromSelected returns true when the currently-playing track is
// known to be from the playlist that's selected in the Playlists panel. For
// v1 this is best-effort: we don't track the source playlist of a track, so
// the heuristic is "the selected playlist contains a track whose persistent
// ID matches the now-playing track's." If the now-playing track has no
// persistent ID (older code paths), this returns false.
func isPlayingFromSelected(m Model) bool {
	conn, ok := m.state.(Connected)
	if !ok || conn.Now.Track.PersistentID == "" {
		return false
	}
	tracks, cached := m.playlists.tracksByName[m.main.selectedPlaylist]
	if !cached {
		return false
	}
	for _, t := range tracks {
		if t.PersistentID == conn.Now.Track.PersistentID {
			return true
		}
	}
	return false
}

// panelBoxWide is the same as panelBox but for the wider main pane. Identical
// implementation kept separate so future tweaks (e.g. main pane padding) can
// diverge without touching left-column panels.
func panelBoxWide(title, body string, width, height int, focused bool) string {
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

// handleMainKey routes keys when focusZ == focusMain.
func handleMainKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	tracks := mainPaneRows(m)
	switch msg.String() {
	case "up", "k":
		if m.main.cursor > 0 {
			m.main.cursor--
		}
		return m, nil, true
	case "down", "j":
		if m.main.cursor < len(tracks)-1 {
			m.main.cursor++
		}
		return m, nil, true
	case "enter":
		if len(tracks) == 0 {
			return m, nil, true
		}
		switch m.main.mode {
		case mainPaneTracks:
			if m.main.selectedPlaylist == "" {
				return m, nil, true
			}
			return m, playPlaylist(m.client, m.main.selectedPlaylist, m.main.cursor), true
		case mainPaneSearchResults:
			pid := tracks[m.main.cursor].PersistentID
			return m, playTrack(m.client, pid), true
		}
	}
	return m, nil, false
}

// mainPaneRows returns whichever slice is currently visible in the main pane.
func mainPaneRows(m Model) []domain.Track {
	switch m.main.mode {
	case mainPaneSearchResults:
		return m.main.searchResults
	default:
		if m.main.selectedPlaylist == "" {
			return nil
		}
		return m.playlists.tracksByName[m.main.selectedPlaylist]
	}
}

// playTrack is the Cmd used when ⏎ is pressed on a search result. Reuses
// client.PlayTrack — same call the search modal already made.
func playTrack(c music.Client, persistentID string) tea.Cmd {
	return func() tea.Msg {
		return searchPlayedMsg{err: c.PlayTrack(context.Background(), persistentID)}
	}
}
