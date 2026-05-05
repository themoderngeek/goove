package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		return m.handleStatus(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tickMsg:
		return m, tea.Batch(scheduleStatusTick(), fetchStatus(m.client))
	case repaintMsg:
		return m, scheduleRepaintTick()
	case actionDoneMsg:
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, tea.Batch(fetchStatus(m.client), clearErrorAfter())
		}
		return m, fetchStatus(m.client)
	case clearErrorMsg:
		m.lastError = nil
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case artworkMsg:
		currentKey := m.currentArtKey()
		if msg.key != currentKey {
			// Stale: a fetch we requested for an earlier track landed after the
			// user skipped to a different track. Discard silently — the new
			// track's status tick will trigger its own fetch.
			return m, nil
		}
		if msg.err != nil {
			slog.Debug("artwork unavailable", "track", msg.key, "err", msg.err)
		}
		m.art = artState{
			key:    msg.key,
			output: msg.output, // "" on any error path → View shows no-art layout
		}
		return m, nil

	case devicesMsg:
		m.output.loading = false
		if msg.err != nil {
			// List-fetch failure — flash in the bottom strip (auto-dissolves)
			// rather than clobbering the Output panel. User retries by
			// re-focusing the panel.
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		m.output.devices = msg.devices
		for i, d := range msg.devices {
			if d.Selected {
				m.output.cursor = i
				break
			}
		}
		return m, nil

	case deviceSetMsg:
		m.output.loading = false
		if msg.err != nil {
			// Per-action error — transient, belongs in the bottom error strip
			// (auto-dissolves) so the user can see the device list to retry.
			// Mirrors the Phase 2 fix for playlistTracksMsg errors.
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, fetchDevices(m.client)

	case playlistsMsg:
		m.playlists.loading = false
		if msg.err != nil {
			// List-fetch failure — flash in the bottom strip (auto-dissolves)
			// rather than clobbering the Playlists panel. User retries by
			// re-focusing the panel.
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		m.playlists.items = msg.playlists
		if m.playlists.cursor >= len(msg.playlists) {
			m.playlists.cursor = 0
		}
		if len(m.playlists.items) > 0 && m.main.selectedPlaylist == "" {
			name := m.playlists.items[0].Name
			m.main.selectedPlaylist = name
			return m, fetchPlaylistTracks(m.client, name)
		}
		return m, nil

	case playlistTracksDebounceMsg:
		// Stale: cursor has moved since this tick was scheduled.
		if msg.seq != m.playlists.seq {
			return m, nil
		}
		// Raced: a previous tick's result already populated the cache.
		if _, cached := m.playlists.tracksByName[msg.name]; cached {
			return m, nil
		}
		// Already in flight (paranoia — shouldn't happen because seq guards
		// concurrent debounces).
		if m.playlists.fetchingFor[msg.name] {
			return m, nil
		}
		// Clear any prior error from a previous attempt — the user is retrying
		// by revisiting this playlist. Also avoids "loading… (with stale error
		// message above)" double-render in the main pane during the new fetch.
		delete(m.playlists.trackErrByName, msg.name)
		m.playlists.fetchingFor[msg.name] = true
		return m, fetchPlaylistTracks(m.client, msg.name)

	case playlistTracksMsg:
		delete(m.playlists.fetchingFor, msg.name)
		if msg.err != nil {
			// Per-playlist error — surfaced in the main pane next to the playlist
			// it belongs to, not in the global error footer.
			m.playlists.trackErrByName[msg.name] = msg.err
		} else {
			delete(m.playlists.trackErrByName, msg.name)
			m.playlists.tracksByName[msg.name] = msg.tracks
		}
		return m, nil

	case searchPanelResultsMsg:
		if msg.seq != m.search.seq {
			return m, nil // stale
		}
		m.search.loading = false
		if msg.err != nil {
			// Preserve inputMode + query so the user can press Enter to retry.
			// lastQuery and total stay unchanged so we don't pollute the "done"
			// render with stale data from a prior successful search.
			m.search.err = msg.err
			return m, nil
		}
		m.search.err = nil
		m.search.inputMode = false
		m.search.lastQuery = msg.query
		m.search.total = msg.result.Total
		// Land results in main pane.
		m.main.mode = mainPaneSearchResults
		m.main.searchResults = domain.RankSearchResults(msg.result.Tracks, msg.query)
		m.main.cursor = 0
		// Focus jumps to main.
		m.focus = focusMain
		return m, nil

	case playTrackResultMsg:
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleStatus(msg statusMsg) (Model, tea.Cmd) {
	switch {
	case errors.Is(msg.err, music.ErrNotRunning):
		m.state = Disconnected{}
		return m, nil
	case errors.Is(msg.err, music.ErrNoTrack):
		m.state = Idle{Volume: m.lastVolume}
		return m, nil
	case errors.Is(msg.err, music.ErrPermission):
		m.permissionDenied = true
		return m, nil
	case msg.err != nil:
		m.lastError = msg.err
		m.lastErrorAt = time.Now()
		return m, clearErrorAfter()
	}
	m.state = Connected{Now: msg.now}
	m.lastVolume = msg.now.Volume

	var cmds []tea.Cmd

	// Track-change detection: fire a single fetchArtwork Cmd when:
	//   - we have a renderer (chafa is available),
	//   - the new track has a real identity (non-empty key),
	//   - the cache is for a different track,
	//   - no fetch is already in flight.
	newKey := trackKey(msg.now.Track)
	if m.renderer != nil && newKey != "" && newKey != m.art.key && !m.art.fetching {
		m.art = artState{key: newKey, fetching: true}
		cmds = append(cmds, fetchArtwork(m.client, m.renderer, newKey))
	}

	// Queue prefetch: fetch the playing playlist's tracks if we don't have
	// them cached and aren't already fetching. Fires at most once per
	// playlist-name change. Empty CurrentPlaylistName = no playlist context.
	if name := msg.now.CurrentPlaylistName; name != "" {
		if _, cached := m.playlists.tracksByName[name]; !cached && !m.playlists.fetchingFor[name] {
			m.playlists.fetchingFor[name] = true
			cmds = append(cmds, fetchPlaylistTracks(m.client, name))
		}
	}

	switch len(cmds) {
	case 0:
		return m, nil
	case 1:
		return m, cmds[0]
	default:
		return m, tea.Batch(cmds...)
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.permissionDenied {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	// Phase 2: focus-routed panel handlers run before globals.
	switch m.focus {
	case focusPlaylists:
		if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
			return mm, cmd
		}
	case focusSearch:
		if mm, cmd, handled := handleSearchPanelKey(m, msg); handled {
			return mm, cmd
		}
	case focusOutput:
		if mm, cmd, handled := handleOutputKey(m, msg); handled {
			return mm, cmd
		}
	case focusMain:
		if mm, cmd, handled := handleMainKey(m, msg); handled {
			return mm, cmd
		}
	}

	switch msg.String() {
	case "tab":
		m.focus = nextFocus(m.focus)
		return m.onFocusEntered()

	case "shift+tab":
		m.focus = prevFocus(m.focus)
		return m.onFocusEntered()

	case "1":
		m.focus = focusPlaylists
		return m.onFocusEntered()

	case "2":
		m.focus = focusSearch
		return m.onFocusEntered()

	case "3":
		m.focus = focusOutput
		return m.onFocusEntered()

	case "4":
		m.focus = focusMain
		return m.onFocusEntered()

	case "q":
		return m, tea.Quit

	case " ":
		if _, ok := m.state.(Disconnected); ok {
			return m, doAction(m.client.Launch)
		}
		return m, doAction(m.client.PlayPause)

	case "n":
		return m, doAction(m.client.Next)

	case "p":
		return m, doAction(m.client.Prev)

	case "+", "=":
		return m.applyVolumeDelta(+5)

	case "-":
		return m.applyVolumeDelta(-5)

	case "o":
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.focus = focusOutput
		mm, cmd := onFocusOutput(m)
		return mm, cmd

	case "/":
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.focus = focusSearch
		m.search.inputMode = true
		return m, nil
	}
	return m, nil
}

// onFocusEntered is called whenever m.focus has just been changed. Dispatches
// to the per-panel on-focus hook, which may return a fetch Cmd.
func (m Model) onFocusEntered() (Model, tea.Cmd) {
	switch m.focus {
	case focusPlaylists:
		return onFocusPlaylists(m)
	case focusOutput:
		return onFocusOutput(m)
	}
	return m, nil
}

func (m Model) applyVolumeDelta(delta int) (Model, tea.Cmd) {
	target := m.lastVolume + delta
	if target < 0 {
		target = 0
	}
	if target > 100 {
		target = 100
	}
	m.lastVolume = target
	if conn, ok := m.state.(Connected); ok {
		conn.Now.Volume = target
		m.state = conn
	}
	client := m.client
	return m, doAction(func(ctx context.Context) error {
		return client.SetVolume(ctx, target)
	})
}
