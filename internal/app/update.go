package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

	if m.picker != nil {
		return m.handlePickerKey(msg)
	}

	switch msg.String() {
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
