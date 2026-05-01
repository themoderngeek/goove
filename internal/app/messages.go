package app

import (
	"time"

	"github.com/themoderngeek/goove/internal/domain"
)

// tickMsg fires once per second and triggers a Status() fetch.
type tickMsg struct{ now time.Time }

// repaintMsg fires four times per second and triggers a re-render only.
type repaintMsg struct{}

// statusMsg is the result of a Status() call.
type statusMsg struct {
	now domain.NowPlaying
	err error
}

// actionDoneMsg is the result of a transport command (PlayPause/Next/Prev/SetVolume).
type actionDoneMsg struct{ err error }

// clearErrorMsg fires once a few seconds after lastError is set,
// so the error footer dissolves on its own.
type clearErrorMsg struct{}
