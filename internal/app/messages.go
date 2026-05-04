package app

import (
	"time"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
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

// artworkMsg is the result of a fetchArtwork Cmd. The `key` is the track
// identity it was requested for — used to discard stale results when the
// user has skipped to a different track during the fetch.
type artworkMsg struct {
	key    string
	output string
	err    error
}

// devicesMsg is the result of a fetchDevices Cmd — populates the Output panel.
type devicesMsg struct {
	devices []domain.AudioDevice
	err     error
}

// deviceSetMsg is the result of a SetAirPlayDevice call from the Output panel.
// On success the device list is refreshed to pick up the new Selected flag.
type deviceSetMsg struct {
	err error
}

// playlistsMsg carries the result of a Playlists fetch.
type playlistsMsg struct {
	playlists []domain.Playlist
	err       error
}

// playlistTracksMsg carries the result of a PlaylistTracks fetch.
// name is the playlist the tracks belong to, used to ignore stale results
// (the user may have moved the cursor and triggered another fetch before
// this one completed).
type playlistTracksMsg struct {
	name   string
	tracks []domain.Track
	err    error
}

// playPlaylistMsg carries the result of a PlayPlaylist invocation. The
// existing 1Hz status tick will surface the new now-playing in its next poll;
// this message exists only so that errors can be funnelled into the
// error footer.
type playPlaylistMsg struct {
	err error
}

// playTrackResultMsg carries the result of a PlayTrack call dispatched by
// the main pane ⏎ in mainPaneSearchResults mode. On error, the bottom error
// footer surfaces it; on success the next status tick reflects the new
// now-playing.
type playTrackResultMsg struct {
	err error
}

// playlistTracksDebounceMsg fires 250ms after a Playlists cursor change.
// seq matches the playlistsPanel.seq at scheduling time; handlers drop the
// message if it doesn't match the current seq (stale — cursor has moved
// since this tick was scheduled).
type playlistTracksDebounceMsg struct {
	seq  uint64
	name string
}

// searchPanelResultsMsg carries the result of a SearchTracks call dispatched
// by the Search panel's Enter handler. seq + query are used for stale-result
// rejection when the user has typed/fired again before this one lands.
type searchPanelResultsMsg struct {
	seq    uint64
	query  string
	result music.SearchResult
	err    error
}
