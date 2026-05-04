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

// devicesMsg is the result of a fetchDevices Cmd — populates the picker.
type devicesMsg struct {
	devices []domain.AudioDevice
	err     error
}

// deviceSetMsg is the result of a SetAirPlayDevice call from inside the picker.
// On success, the picker closes; on error, the picker stays open and shows the error.
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

// searchDebounceMsg fires 250ms after the last keystroke in the search modal.
// seq is the searchState.seq the tick was scheduled under — handlers drop
// the message if it doesn't match the current seq (stale).
type searchDebounceMsg struct {
	seq uint64
}

// searchResultsMsg carries the result of a SearchTracks call. seq + query
// guard against a result arriving for a query the user has already moved
// on from.
type searchResultsMsg struct {
	seq    uint64
	query  string
	result music.SearchResult
	err    error
}

// searchPlayedMsg carries the result of a PlayTrack call from inside search.
// On error, the modal stays open and shows the error footer.
type searchPlayedMsg struct {
	seq uint64
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

// searchPanelResultsMsg is the panel flow's analogue of searchResultsMsg.
// Carries seq + query for stale-result rejection.
type searchPanelResultsMsg struct {
	seq    uint64
	query  string
	result music.SearchResult
	err    error
}
