package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/art"
	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// AppState is a closed sum type implemented by Disconnected, Idle, Connected.
// The unexported isAppState() method makes it unsatisfiable from outside
// this package, giving us a sealed-class shape without language support.
type AppState interface{ isAppState() }

type Disconnected struct{}
type Idle struct{ Volume int }
type Connected struct{ Now domain.NowPlaying }

func (Disconnected) isAppState() {}
func (Idle) isAppState()         {}
func (Connected) isAppState()    {}

// artState is the single-slot in-memory cache for the current track's rendered
// album art. `key` is the trackKey the bytes were fetched for; `output` is the
// chafa-rendered ANSI string (empty on any error path); `fetching` suppresses
// duplicate fetches while a Cmd is in flight.
type artState struct {
	key      string
	output   string
	fetching bool
}

// Model holds the entire goove TUI state.
type Model struct {
	client music.Client

	state       AppState
	lastVolume  int
	lastError   error
	lastErrorAt time.Time

	// Permission failure shows a blocking screen; the value is sticky.
	permissionDenied bool

	// Latest terminal size for layout decisions.
	width  int
	height int

	art      artState
	renderer art.Renderer // nil ⇒ chafa unavailable; track-change detection skips fetches
}

// New builds an initial Model with state Disconnected and lastVolume 50.
// The renderer may be nil — in that case, album art is permanently disabled
// (the track-change detection in handleStatus skips when renderer == nil).
func New(client music.Client, renderer art.Renderer) Model {
	return Model{
		client:     client,
		renderer:   renderer,
		state:      Disconnected{},
		lastVolume: 50,
	}
}

// Init returns the first Cmd: an immediate IsRunning probe + start both ticks.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.client),
		scheduleStatusTick(),
		scheduleRepaintTick(),
	)
}
