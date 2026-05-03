package app

import (
	"context"

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
// updated model + any Cmd. Transport keys (space, n, p, +, -, q) fall through
// to the now-playing key handler (Task 23). Browser-specific keys are handled
// here.
func handleBrowserKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.browser == nil {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		return browserCursorUp(m), nil
	case "down", "j":
		return browserCursorDown(m), nil
	case "tab", "right":
		return browserFocusRight(m)
	case "shift+tab", "left":
		m.browser.pane = leftPane
		return m, nil
	case "enter":
		return handleBrowserEnter(m)
	case "r":
		return handleBrowserRefetch(m)
	}
	return m, nil
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
