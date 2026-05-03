package music

import (
	"context"
	"errors"

	"github.com/themoderngeek/goove/internal/domain"
)

// SearchResult is the wire shape for SearchTracks. Tracks holds at most 100
// rows (the cap is enforced inside the AppleScript). Total carries the full
// underlying match count so callers can render a "100 of N" truncation hint.
type SearchResult struct {
	Tracks []domain.Track
	Total  int
}

type Client interface {
	IsRunning(ctx context.Context) (bool, error)
	Launch(ctx context.Context) error
	Status(ctx context.Context) (domain.NowPlaying, error)
	PlayPause(ctx context.Context) error
	Next(ctx context.Context) error
	Prev(ctx context.Context) error
	SetVolume(ctx context.Context, percent int) error
	Artwork(ctx context.Context) ([]byte, error)
	AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
	CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
	SetAirPlayDevice(ctx context.Context, name string) error
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Playlists(ctx context.Context) ([]domain.Playlist, error)
	PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error)
	PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error

	// SearchTracks returns up to 100 library tracks whose title, artist, or
	// album contains query (case-insensitive). Total in the result is the
	// full underlying match count.
	SearchTracks(ctx context.Context, query string) (SearchResult, error)

	// PlayTrack starts playback of the track with the given persistent ID.
	// Replaces the current play context. Returns ErrTrackNotFound if no
	// track in the library has that ID (e.g. it was deleted).
	PlayTrack(ctx context.Context, persistentID string) error
}

var (
	ErrNotRunning       = errors.New("music: app not running")
	ErrNoTrack          = errors.New("music: no track loaded")
	ErrUnavailable      = errors.New("music: backend call failed")
	ErrPermission       = errors.New("music: automation permission denied")
	ErrNoArtwork        = errors.New("music: track has no artwork")
	ErrDeviceNotFound   = errors.New("music: airplay device not found")
	ErrAmbiguousDevice  = errors.New("music: airplay device name matches multiple devices")
	ErrPlaylistNotFound = errors.New("music: playlist not found")
	ErrTrackNotFound    = errors.New("music: track not found")
)
