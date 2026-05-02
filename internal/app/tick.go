package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/art"
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
		np, err := client.Status(context.Background())
		return statusMsg{now: np, err: err}
	}
}

// doAction wraps a Music transport command in a Cmd that emits actionDoneMsg.
func doAction(action func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: action(context.Background())}
	}
}

// clearErrorAfter emits a clearErrorMsg once errorVisibleFor has elapsed.
func clearErrorAfter() tea.Cmd {
	return tea.Tick(errorVisibleFor, func(time.Time) tea.Msg { return clearErrorMsg{} })
}

const (
	artWidth           = 20
	artHeight          = 10
	artLayoutThreshold = 70  // terminal width below which side-by-side layout is suppressed
)

// fetchArtwork pipelines bytes from the music client through the art renderer.
// Stale guard happens at the message handler — this Cmd just produces a result
// tagged with the requested key.
func fetchArtwork(client music.Client, renderer art.Renderer, key string) tea.Cmd {
	return func() tea.Msg {
		bytes, err := client.Artwork(context.Background())
		if err != nil {
			return artworkMsg{key: key, err: err}
		}
		out, err := renderer.Render(context.Background(), bytes, artWidth, artHeight)
		return artworkMsg{key: key, output: out, err: err}
	}
}

// fetchDevices runs AirPlayDevices in a goroutine and emits a devicesMsg.
// Used by the picker on open.
func fetchDevices(client music.Client) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.AirPlayDevices(context.Background())
		return devicesMsg{devices: devices, err: err}
	}
}
