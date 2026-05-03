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
// this message is just for surfacing errors in the browser.
type playPlaylistMsg struct {
	err error
}
