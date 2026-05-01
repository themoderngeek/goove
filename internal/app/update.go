package app

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		return m.handleStatus(msg)
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
