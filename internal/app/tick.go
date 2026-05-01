package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)

const (
	statusInterval  = time.Second
	repaintInterval = 250 * time.Millisecond
	errorVisibleFor = 3 * time.Second
)

// scheduleStatusTick produces a Cmd that emits a tickMsg after one second.
// Each tickMsg handler re-arms the next tick.
func scheduleStatusTick() tea.Cmd {
	return tea.Tick(statusInterval, func(t time.Time) tea.Msg { return tickMsg{now: t} })
}

// scheduleRepaintTick produces a Cmd that emits a repaintMsg after 250ms.
func scheduleRepaintTick() tea.Cmd {
	return tea.Tick(repaintInterval, func(t time.Time) tea.Msg { return repaintMsg{} })
}

// fetchStatus runs Status() in a goroutine and emits a statusMsg with the result.
func fetchStatus(client music.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		np, err := client.Status(ctx)
		return statusMsg{now: np, err: err}
	}
}

// doAction wraps a Music transport command in a Cmd that emits actionDoneMsg.
func doAction(action func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return actionDoneMsg{err: action(ctx)}
	}
}

// clearErrorAfter emits a clearErrorMsg once errorVisibleFor has elapsed.
func clearErrorAfter() tea.Cmd {
	return tea.Tick(errorVisibleFor, func(time.Time) tea.Msg { return clearErrorMsg{} })
}
