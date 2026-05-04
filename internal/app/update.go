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
		if m.picker == nil {
			return m, nil // user esc'd before fetch returned — discard
		}
		m.picker.loading = false
		m.picker.err = msg.err
		m.picker.devices = msg.devices
		// Land cursor on currently-selected device, if any.
		for i, d := range msg.devices {
			if d.Selected {
				m.picker.cursor = i
				break
			}
		}
		return m, nil

	case deviceSetMsg:
		if m.picker == nil {
			return m, nil // user esc'd before set returned — discard
		}
		if msg.err != nil {
			m.picker.loading = false
			m.picker.err = msg.err
			return m, nil
		}
		// Success: close the picker. Next 1Hz status tick re-renders the player view.
		m.picker = nil
		return m, nil

	case playlistsMsg:
		// Phase 2: also populate the persistent panel state.
		m.playlists.loading = false
		m.playlists.err = msg.err
		if msg.err == nil {
			m.playlists.items = msg.playlists
			if m.playlists.cursor >= len(msg.playlists) {
				m.playlists.cursor = 0
			}
		}
		// Existing browser-modal write (Phase 2 still keeps the modal alive):
		if m.browser != nil {
			m.browser.loadingLists = false
			m.browser.err = msg.err
			if msg.err == nil {
				m.browser.playlists = msg.playlists
				if m.browser.playlistCursor >= len(msg.playlists) {
					m.browser.playlistCursor = 0
				}
			}
		}
		return m, nil

	case playlistTracksMsg:
		// Phase 2: populate the persistent track cache.
		delete(m.playlists.fetchingFor, msg.name)
		if msg.err != nil {
			m.playlists.err = msg.err
		} else {
			m.playlists.tracksByName[msg.name] = msg.tracks
		}
		// Existing browser-modal write (Phase 2 still keeps the modal alive):
		if m.browser != nil && len(m.browser.playlists) > 0 {
			current := m.browser.playlists[m.browser.playlistCursor].Name
			if msg.name != current {
				// Stale result for the modal — the cursor has moved since this
				// fetch was issued. Panel cache write above already happened.
				return m, nil
			}
			m.browser.loadingTracks = false
			m.browser.err = msg.err
			if msg.err == nil {
				m.browser.tracks = msg.tracks
				m.browser.tracksFor = msg.name
				m.browser.trackCursor = 0
			}
		}
		return m, nil

	case searchDebounceMsg:
		if m.search == nil || msg.seq != m.search.seq {
			return m, nil
		}
		if m.search.query == "" {
			return m, nil
		}
		m.search.loading = true
		return m, fetchSearch(m.client, m.search.seq, m.search.query)

	case searchResultsMsg:
		if m.search == nil || msg.seq != m.search.seq || msg.query != m.search.query {
			return m, nil
		}
		m.search.loading = false
		m.search.err = msg.err
		m.search.results = domain.RankSearchResults(msg.result.Tracks, msg.query)
		m.search.total = msg.result.Total
		m.search.cursor = 0
		return m, nil

	case searchPlayedMsg:
		if m.search != nil {
			if msg.seq != m.search.seq {
				return m, nil
			}
			if msg.err != nil {
				m.search.err = msg.err
				return m, nil
			}
			m.search = nil
			return m, nil
		}
		// Phase 2: result from the new main-pane enter.
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

	// Track-change detection: fire a single fetchArtwork Cmd when:
	//   - we have a renderer (chafa is available),
	//   - the new track has a real identity (non-empty key),
	//   - the cache is for a different track,
	//   - no fetch is already in flight.
	newKey := trackKey(msg.now.Track)
	if m.renderer != nil && newKey != "" && newKey != m.art.key && !m.art.fetching {
		m.art = artState{key: newKey, fetching: true}
		return m, fetchArtwork(m.client, m.renderer, newKey)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.permissionDenied {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.search != nil {
		return m.handleSearchKey(msg)
	}

	if m.picker != nil {
		return m.handlePickerKey(msg)
	}

	if m.mode == modeBrowser {
		if mm, cmd, handled := handleBrowserKey(m, msg); handled {
			return mm, cmd
		}
		// Fall through to the now-playing key handler for transport keys etc.
	}

	// Phase 2: focus-routed panel handlers run before globals.
	if m.search == nil && m.picker == nil && m.mode != modeBrowser {
		switch m.focusZ {
		case focusPlaylists:
			if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
				return mm, cmd
			}
		case focusMain:
			if mm, cmd, handled := handleMainKey(m, msg); handled {
				return mm, cmd
			}
		}
	}

	switch msg.String() {
	case "tab":
		m.focusZ = nextFocus(m.focusZ)
		return m.onFocusEntered()

	case "shift+tab":
		m.focusZ = prevFocus(m.focusZ)
		return m.onFocusEntered()

	case "1":
		m.focusZ = focusPlaylists
		return m.onFocusEntered()

	case "2":
		m.focusZ = focusSearch
		return m.onFocusEntered()

	case "3":
		m.focusZ = focusOutput
		return m.onFocusEntered()

	case "4":
		m.focusZ = focusMain
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
		// Open the device picker. Suppressed in Disconnected — the AirPlay
		// device list requires Music to be running. permissionDenied is also
		// suppressed (handled at the top of this function).
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.picker = &pickerState{loading: true}
		return m, fetchDevices(m.client)

	case "l":
		// 'l' opens the playlist browser. No-op when already in browser
		// (spec: 'l' in browser is a no-op; esc returns to now-playing).
		if m.mode == modeBrowser {
			return m, nil
		}
		m.mode = modeBrowser
		m.browser = &browserState{loadingLists: true}
		return m, fetchPlaylists(m.client)

	case "/":
		// Suppress search in Disconnected, when picker is open, or when in browser.
		// permissionDenied is already handled at the top of Update.
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		if m.picker != nil || m.mode == modeBrowser {
			return m, nil
		}
		m.search = &searchState{}
		return m, nil
	}
	return m, nil
}

// handlePickerKey routes keystrokes when the picker overlay is open.
// Transport keys are suppressed by virtue of routing through this function
// instead of the normal switch.
func (m Model) handlePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.picker.loading {
		// Only esc/q work while loading.
		if msg.String() == "esc" || msg.String() == "q" {
			m.picker = nil
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		m.picker = nil
		return m, nil

	case "up", "k":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
		return m, nil

	case "down", "j":
		if m.picker.cursor < len(m.picker.devices)-1 {
			m.picker.cursor++
		}
		return m, nil

	case "enter":
		if len(m.picker.devices) == 0 {
			return m, nil
		}
		target := m.picker.devices[m.picker.cursor].Name
		m.picker.loading = true
		client := m.client
		return m, func() tea.Msg {
			err := client.SetAirPlayDevice(context.Background(), target)
			return deviceSetMsg{err: err}
		}
	}
	return m, nil
}

// onFocusEntered is called whenever m.focusZ has just been changed. Dispatches
// to the per-panel on-focus hook, which may return a fetch Cmd.
func (m Model) onFocusEntered() (Model, tea.Cmd) {
	switch m.focusZ {
	case focusPlaylists:
		return onFocusPlaylists(m)
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
