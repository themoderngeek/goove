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

// playlistsPanel is the state of the Playlists panel (left, top of stack).
// items is the cached playlist list; cursor is the highlighted row;
// tracksByName caches per-playlist tracks for live-preview hits.
type playlistsPanel struct {
	items          []domain.Playlist
	cursor         int
	loading        bool
	tracksByName   map[string][]domain.Track
	fetchingFor    map[string]bool
	trackErrByName map[string]error // per-playlist track-fetch errors; surfaced in the main pane
	seq            uint64           // bumped on every cursor change; debounce drops stale ticks
}

// searchPanel is the state of the Search panel (left, middle of stack).
// inputMode true means typing routes into the query; outside input mode the
// panel is "idle" and shows a muted prompt. Result rows themselves live on
// mainPanel.searchResults — this struct holds only the query-input and
// summary state (last-fired query and total match count for the panel's
// "N results" line).
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
// Devices are fetched lazily on first focus and cached. Selection is
// two-step (Q3-C in the design): cursor moves don't switch the audio
// device — only ⏎ does.
type outputPanel struct {
	devices []domain.AudioDevice
	cursor  int
	loading bool
}

// mainPaneMode toggles what content the main pane displays. Tracks of the
// playlist currently selected on the left (default), or the rows of the
// most recent search.
type mainPaneMode int

const (
	mainPaneTracks mainPaneMode = iota
	mainPaneSearchResults
)

// mainPanel is the state of the right-hand main pane. searchResults is
// populated by the Search panel's enter handler when it fires a query;
// this is intentional cross-struct ownership — the Search panel owns
// query input, the main pane owns the rows.
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

	// New layout state (Phase 1).
	focus     focusKind
	playlists playlistsPanel
	search    searchPanel
	output    outputPanel
	main      mainPanel

	// Queue management state (spec 2026-05-13-queue-management-design.md).
	queue          QueueState
	resume         ResumeContext
	lastTrackPID   string // PID seen on previous status tick; "" at launch
	lastPlaylist   string // CurrentPlaylistName on previous tick
	lastTrackIdx   int    // 1-based index of last-seen track in lastPlaylist; 0 if unknown
	overlay        overlayState
	clearPrompt    bool   // true while awaiting y/n after `c` in overlay
	pendingJumpPID string // one-shot: overlay Enter sets this; handoff handler clears on match
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
			tracksByName:   make(map[string][]domain.Track),
			fetchingFor:    make(map[string]bool),
			trackErrByName: make(map[string]error),
			loading:        true,
		},
		output: outputPanel{
			loading: true,
		},
	}
}

// Init returns the first Cmd: an immediate IsRunning probe + start both ticks.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.client),
		scheduleStatusTick(),
		scheduleRepaintTick(),
		fetchPlaylists(m.client),
		fetchDevices(m.client),
	)
}
