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

// pickerState is the modal device-picker overlay state.
// nil on Model means "picker not open"; non-nil means "picker is showing."
// While loading is true, only esc/q are honoured (cancel cancels both fetch and set).
type pickerState struct {
	loading bool
	devices []domain.AudioDevice
	cursor  int
	err     error
}

// searchState is the modal search overlay state.
// nil on Model means "search not open"; non-nil means "search modal showing."
//
// seq is bumped on every keystroke; in-flight debounce ticks and result
// messages carry the seq they were issued under, so stale ones are dropped
// when seq advances. Same pattern as the artwork fetch's track-key guard.
type searchState struct {
	query   string
	seq     uint64
	loading bool
	results []domain.Track
	total   int
	cursor  int
	err     error
}

type viewMode int

const (
	modeNowPlaying viewMode = iota
	modeBrowser
)

type browserPane int

const (
	leftPane  browserPane = iota // playlists
	rightPane                    // tracks of selected playlist
)

// browserState is the modal browser-view state. nil on Model means "browser
// not open"; non-nil means "browser is showing." Loading flags suppress
// duplicate fetches while a Cmd is in flight.
type browserState struct {
	pane           browserPane
	playlists      []domain.Playlist
	playlistCursor int
	loadingLists   bool
	tracks         []domain.Track // tracks of the playlist at playlistCursor
	tracksFor      string         // name of the playlist tracks were last fetched for
	trackCursor    int
	loadingTracks  bool
	err            error
}

// playlistsPanel is the state of the Playlists panel (left, top of stack).
// items is the cached playlist list; cursor is the highlighted row;
// tracksByName caches per-playlist tracks for live-preview hits.
type playlistsPanel struct {
	items        []domain.Playlist
	cursor       int
	loading      bool
	err          error
	tracksByName map[string][]domain.Track
	fetchingFor  map[string]bool
}

// searchPanel is the state of the Search panel (left, middle of stack).
// inputMode true means typing routes into the query; outside input mode the
// panel is "idle" and shows a muted prompt.
type searchPanel struct {
	inputMode bool
	query     string
	seq       uint64
	loading   bool
	lastQuery string
	total     int
	err       error
}

// outputPanel is the state of the Output panel (left, bottom of stack).
type outputPanel struct {
	devices []domain.AudioDevice
	cursor  int
	loading bool
	err     error
}

// mainPaneMode is which "view" the main pane is showing.
type mainPaneMode int

const (
	mainPaneTracks mainPaneMode = iota
	mainPaneSearchResults
)

// mainPanel is the state of the right-hand main pane.
type mainPanel struct {
	mode             mainPaneMode
	cursor           int
	selectedPlaylist string
	searchResults    []domain.Track
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
	picker   *pickerState // nil ⇒ picker not open (modal overlay state)
	mode     viewMode
	browser  *browserState
	search   *searchState // nil ⇒ search modal not open

	// New layout state (Phase 1).
	focusZ    focus
	playlists playlistsPanel
	search2   searchPanel // temp name; renamed to `search` in Phase 6 after the modal type is retired
	output    outputPanel
	main      mainPanel
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
		playlists: playlistsPanel{
			tracksByName: make(map[string][]domain.Track),
			fetchingFor:  make(map[string]bool),
		},
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
