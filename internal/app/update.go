package app

import (
	"context"
	"errors"
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
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.permissionDenied {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
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
