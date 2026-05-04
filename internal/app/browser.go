package app

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)

// fetchPlaylists returns a Cmd that calls client.Playlists and produces
// a playlistsMsg.
func fetchPlaylists(c music.Client) tea.Cmd {
	return func() tea.Msg {
		playlists, err := c.Playlists(context.Background())
		return playlistsMsg{playlists: playlists, err: err}
	}
}

// fetchPlaylistTracks returns a Cmd that calls client.PlaylistTracks and
// produces a playlistTracksMsg. The name is echoed in the message so the
// update handler can ignore stale results.
func fetchPlaylistTracks(c music.Client, name string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.PlaylistTracks(context.Background(), name)
		return playlistTracksMsg{name: name, tracks: tracks, err: err}
	}
}

// playPlaylist returns a Cmd that calls client.PlayPlaylist and produces
// a playPlaylistMsg.
func playPlaylist(c music.Client, name string, fromIdx int) tea.Cmd {
	return func() tea.Msg {
		err := c.PlayPlaylist(context.Background(), name, fromIdx)
		return playPlaylistMsg{err: err}
	}
}

// handleBrowserKey routes key messages while the browser is open. Returns the
// updated model, any Cmd, and a "handled" flag. When handled is false, the
// caller should fall through to the now-playing key handler (so transport keys
// like space/n/p/+/-/q still work in browser mode). Browser-specific keys
// (j/k/up/down/tab/right/shift+tab/left/enter/r/esc/l) are handled here.
func handleBrowserKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.browser == nil {
		return m, nil, false
	}
	switch msg.String() {
	case "up", "k":
		mm := browserCursorUp(m)
		return mm, nil, true
	case "down", "j":
		mm := browserCursorDown(m)
		return mm, nil, true
	case "tab", "right":
		mm, cmd := browserFocusRight(m)
		return mm, cmd, true
	case "shift+tab", "left":
		m.browser.pane = leftPane
		return m, nil, true
	case "enter":
		mm, cmd := handleBrowserEnter(m)
		return mm, cmd, true
	case "r":
		mm, cmd := handleBrowserRefetch(m)
		return mm, cmd, true
	case "esc":
		m.mode = modeNowPlaying
		m.browser = nil
		return m, nil, true
	case "l":
		// Already in browser; spec says no-op (don't toggle).
		return m, nil, true
	case "1", "2", "3", "4":
		// Browser doesn't use number keys; suppress so the new focus-cycle
		// cases in the outer handleKey don't fire underneath the overlay.
		return m, nil, true
	}
	// Not a browser-specific key — let the now-playing handler take it.
	return m, nil, false
}

// handleBrowserRefetch refetches the focused pane's data: playlists for the
// left pane, tracks for the right pane. Resets the relevant tracksFor sentinel
// so the result is not treated as stale.
func handleBrowserRefetch(m Model) (Model, tea.Cmd) {
	if m.browser.pane == leftPane {
		m.browser.loadingLists = true
		return m, fetchPlaylists(m.client)
	}
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	m.browser.loadingTracks = true
	m.browser.tracksFor = "" // force the playlistTracksMsg handler to accept the result
	return m, fetchPlaylistTracks(m.client, current)
}

// handleBrowserEnter starts playback. From the left pane, it plays the
// highlighted playlist from track 1. From the right pane, it plays the
// playlist starting at the highlighted track. Empty playlists are a no-op.
func handleBrowserEnter(m Model) (Model, tea.Cmd) {
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	if m.browser.pane == leftPane {
		return m, playPlaylist(m.client, current, 0)
	}
	if len(m.browser.tracks) == 0 {
		return m, nil
	}
	return m, playPlaylist(m.client, current, m.browser.trackCursor)
}

// browserFocusRight switches focus to the right (tracks) pane. If the tracks
// for the currently-selected playlist haven't been fetched yet (or were
// fetched for a different playlist), it dispatches a fetchPlaylistTracks Cmd
// and sets loadingTracks. Otherwise it's a pure focus change.
func browserFocusRight(m Model) (Model, tea.Cmd) {
	m.browser.pane = rightPane
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	if m.browser.tracksFor == current {
		return m, nil // already have these tracks
	}
	m.browser.loadingTracks = true
	m.browser.tracks = nil
	m.browser.trackCursor = 0
	return m, fetchPlaylistTracks(m.client, current)
}

// browserCursorUp moves the cursor up by 1, clamped to 0, in the focused pane.
func browserCursorUp(m Model) Model {
	if m.browser.pane == leftPane {
		if m.browser.playlistCursor > 0 {
			m.browser.playlistCursor--
		}
	} else {
		if m.browser.trackCursor > 0 {
			m.browser.trackCursor--
		}
	}
	return m
}

// browserCursorDown moves the cursor down by 1, clamped to the last item, in
// the focused pane.
func browserCursorDown(m Model) Model {
	if m.browser.pane == leftPane {
		if m.browser.playlistCursor < len(m.browser.playlists)-1 {
			m.browser.playlistCursor++
		}
	} else {
		if m.browser.trackCursor < len(m.browser.tracks)-1 {
			m.browser.trackCursor++
		}
	}
	return m
}

// renderBrowser returns the full-screen string for modeBrowser. Layout is two
// columns separated by a vertical bar. Long lists scroll via window-clamping
// around the cursor. Width comes from the Model.
func renderBrowser(m Model) string {
	if m.browser == nil {
		return "" // shouldn't happen, but guard anyway
	}
	// Reserve some terminal-edge padding; everything else is split.
	totalWidth := m.width
	if totalWidth < 40 {
		totalWidth = 80 // safe default
	}
	leftWidth := totalWidth/2 - 1
	rightWidth := totalWidth - leftWidth - 7 // 7 = "│ " + " │ " + " │"  (left border + inner separator + right border)
	height := m.height - 4
	if height < 5 {
		height = 20
	}

	leftLines := renderLeftPane(m.browser, leftWidth, height)
	rightLines := renderRightPane(m.browser, rightWidth, height)

	var out strings.Builder
	out.WriteString("┌─ goove · browser ")
	out.WriteString(strings.Repeat("─", max(0, totalWidth-20)))
	out.WriteString("┐\n")
	for i := 0; i < height; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		fmt.Fprintf(&out, "│ %-*s │ %-*s │\n", leftWidth, left, rightWidth, right)
	}
	out.WriteString("└")
	out.WriteString(strings.Repeat("─", totalWidth-2))
	out.WriteString("┘\n")
	out.WriteString(" ↑↓: nav   tab: pane   ⏎: play   r: refetch   esc: back   space: ⏯\n")
	return out.String()
}

func renderLeftPane(b *browserState, width, height int) []string {
	header := "Playlists"
	if b.pane == leftPane {
		header = "▸ Playlists"
	}
	out := []string{header, ""}
	if b.loadingLists {
		out = append(out, "Loading…")
		return out
	}
	if b.err != nil && b.pane == leftPane {
		out = append(out, "error: "+b.err.Error())
		return out
	}
	if len(b.playlists) == 0 {
		out = append(out, "(no playlists)")
		return out
	}
	visibleRows := height - 2
	start := scrollWindow(b.playlistCursor, visibleRows, len(b.playlists))
	for i := start; i < len(b.playlists) && i-start < visibleRows; i++ {
		marker := "  "
		if i == b.playlistCursor && b.pane == leftPane {
			marker = "▸ "
		}
		row := fmt.Sprintf("%s%s", marker, b.playlists[i].Name)
		if b.playlists[i].Kind == "subscription" {
			row += " (sub)"
		}
		out = append(out, truncate(row, width))
	}
	return out
}

func renderRightPane(b *browserState, width, height int) []string {
	title := "Tracks"
	if len(b.playlists) > 0 {
		title = "Tracks — " + b.playlists[b.playlistCursor].Name
	}
	if b.pane == rightPane {
		title = "▸ " + title
	}
	out := []string{title, ""}
	if b.pane == rightPane && b.loadingTracks {
		out = append(out, "Loading…")
		return out
	}
	if b.pane == rightPane && b.err != nil {
		out = append(out, "error: "+b.err.Error())
		return out
	}
	current := ""
	if len(b.playlists) > 0 {
		current = b.playlists[b.playlistCursor].Name
	}
	if b.tracksFor != current {
		out = append(out, "(press tab to load)")
		return out
	}
	if len(b.tracks) == 0 {
		out = append(out, "(no tracks)")
		return out
	}
	visibleRows := height - 2
	start := scrollWindow(b.trackCursor, visibleRows, len(b.tracks))
	for i := start; i < len(b.tracks) && i-start < visibleRows; i++ {
		marker := "  "
		if i == b.trackCursor && b.pane == rightPane {
			marker = "▸ "
		}
		t := b.tracks[i]
		row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
		out = append(out, truncate(row, width))
	}
	return out
}

// scrollWindow returns the top-of-window index such that cursor is visible
// within a viewport of size visible across total items. Cursor stays roughly
// centred when possible.
func scrollWindow(cursor, visible, total int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := cursor - half
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width <= 1 {
		// Return just the first rune.
		_, size := utf8.DecodeRuneInString(s)
		return s[:size]
	}
	// Walk runes up to width-1, then append the ellipsis.
	i, count := 0, 0
	for i < len(s) && count < width-1 {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		count++
	}
	return s[:i] + "…"
}
