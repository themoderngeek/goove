package fake

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// Client is an in-memory implementation of music.Client used in tests.
// Tests script its state via Launch / SetTrack / SimulateError.
type Client struct {
	mu                 sync.Mutex
	running            bool
	hasTrack           bool
	track              domain.Track
	duration           time.Duration
	position           time.Duration
	playing            bool
	volume             int
	forcedErr          error
	artwork            []byte
	artworkErr         error
	devices            []domain.AudioDevice
	playlists          []domain.Playlist
	playlistTracks     map[string][]domain.Track
	playPlaylistRecord []PlayPlaylistCall

	// Set by SetTracks; queried by SearchTracks. Distinct from playlistTracks
	// because library search is a property of the whole library, not of any
	// one playlist.
	libraryTracks []domain.Track

	// Records of PlayTrack invocations.
	playTrackRecord []PlayTrackCall

	// Counters useful for assertions.
	PlayPauseCalls    int
	PlayCalls         int
	PauseCalls        int
	NextCalls         int
	PrevCalls         int
	SetVolumeCalls    int
	LaunchCalls       int
	PlayPlaylistCalls int
	PlayTrackCalls    int

	currentPlaylistName string
	shuffleEnabled      bool
}

// PlayPlaylistCall records one PlayPlaylist invocation.
type PlayPlaylistCall struct {
	Name    string
	FromIdx int
}

type PlayTrackCall struct {
	PersistentID string
}

func New() *Client {
	return &Client{volume: 50}
}

func (c *Client) SetTrack(t domain.Track, durationSec, positionSec int, playing bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hasTrack = true
	c.track = t
	c.duration = time.Duration(durationSec) * time.Second
	c.position = time.Duration(positionSec) * time.Second
	c.playing = playing
}

// SetCurrentPlaylistName supplies the value the next Status call returns
// as NowPlaying.CurrentPlaylistName. Used by Up Next queue tests to
// simulate "playing from playlist X" vs "no playlist context".
func (c *Client) SetCurrentPlaylistName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentPlaylistName = name
}

// SetShuffleEnabled supplies the value the next Status call returns as
// NowPlaying.ShuffleEnabled.
func (c *Client) SetShuffleEnabled(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shuffleEnabled = on
}

func (c *Client) SimulateError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forcedErr = err
}

// SetArtwork supplies bytes the next Artwork() call will return.
func (c *Client) SetArtwork(bytes []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artwork = bytes
}

// SetArtworkErr forces Artwork() to return the given error, regardless of
// whether bytes have been set. Use SetArtworkErr(nil) to clear.
func (c *Client) SetArtworkErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artworkErr = err
}

// SetDevices supplies the AirPlay device list the next AirPlayDevices call returns.
// SetAirPlayDevice mutates the Selected flag on entries in this list.
func (c *Client) SetDevices(devices []domain.AudioDevice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices = devices
}

// AirPlayDevices implements music.Client.
func (c *Client) AirPlayDevices(_ context.Context) ([]domain.AudioDevice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	// Return a copy so callers can't mutate our internal slice.
	out := make([]domain.AudioDevice, len(c.devices))
	copy(out, c.devices)
	return out, nil
}

// CurrentAirPlayDevice implements music.Client. Returns the device with
// Selected=true, or ErrDeviceNotFound if no device is selected.
func (c *Client) CurrentAirPlayDevice(_ context.Context) (domain.AudioDevice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return domain.AudioDevice{}, c.forcedErr
	}
	if !c.running {
		return domain.AudioDevice{}, music.ErrNotRunning
	}
	for _, d := range c.devices {
		if d.Selected {
			return d, nil
		}
	}
	return domain.AudioDevice{}, music.ErrDeviceNotFound
}

// SetAirPlayDevice implements music.Client. Updates the Selected flag in-place:
// the named device becomes Selected=true, all others become Selected=false.
// Returns ErrDeviceNotFound if no device with the exact name exists.
func (c *Client) SetAirPlayDevice(_ context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	found := false
	for i := range c.devices {
		if c.devices[i].Name == name {
			c.devices[i].Selected = true
			found = true
		} else {
			c.devices[i].Selected = false
		}
	}
	if !found {
		return music.ErrDeviceNotFound
	}
	return nil
}

// Artwork implements music.Client.
func (c *Client) Artwork(ctx context.Context) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if c.artworkErr != nil {
		return nil, c.artworkErr
	}
	if c.artwork == nil {
		return nil, music.ErrNoArtwork
	}
	return c.artwork, nil
}

func (c *Client) IsRunning(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return false, c.forcedErr
	}
	return c.running, nil
}

func (c *Client) Launch(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	c.LaunchCalls++
	c.running = true
	return nil
}

func (c *Client) Status(ctx context.Context) (domain.NowPlaying, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return domain.NowPlaying{}, c.forcedErr
	}
	if !c.running {
		return domain.NowPlaying{}, music.ErrNotRunning
	}
	if !c.hasTrack {
		return domain.NowPlaying{}, music.ErrNoTrack
	}
	return domain.NowPlaying{
		Track:               c.track,
		Position:            c.position,
		Duration:            c.duration,
		IsPlaying:           c.playing,
		Volume:              c.volume,
		LastSyncedAt:        time.Now(),
		CurrentPlaylistName: c.currentPlaylistName,
		ShuffleEnabled:      c.shuffleEnabled,
	}, nil
}

func (c *Client) PlayPause(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PlayPauseCalls++
	c.playing = !c.playing
	return nil
}

// Play implements music.Client.
func (c *Client) Play(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PlayCalls++
	return nil
}

// Pause implements music.Client.
func (c *Client) Pause(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PauseCalls++
	return nil
}

func (c *Client) Next(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.NextCalls++
	return nil
}

func (c *Client) Prev(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PrevCalls++
	return nil
}

func (c *Client) SetVolume(ctx context.Context, percent int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.SetVolumeCalls++
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	c.volume = percent
	return nil
}

// SetPlaylists supplies the playlist list returned by Playlists.
func (c *Client) SetPlaylists(playlists []domain.Playlist) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.playlists = playlists
}

// SetPlaylistTracks supplies the tracks returned by PlaylistTracks(name).
func (c *Client) SetPlaylistTracks(name string, tracks []domain.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.playlistTracks == nil {
		c.playlistTracks = make(map[string][]domain.Track)
	}
	c.playlistTracks[name] = tracks
}

// PlayPlaylistRecord returns a copy of the recorded PlayPlaylist invocations.
func (c *Client) PlayPlaylistRecord() []PlayPlaylistCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]PlayPlaylistCall, len(c.playPlaylistRecord))
	copy(out, c.playPlaylistRecord)
	return out
}

// Playlists implements music.Client.
func (c *Client) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	out := make([]domain.Playlist, len(c.playlists))
	copy(out, c.playlists)
	return out, nil
}

// PlaylistTracks implements music.Client.
func (c *Client) PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	tracks, ok := c.playlistTracks[playlistName]
	if !ok {
		return nil, music.ErrPlaylistNotFound
	}
	out := make([]domain.Track, len(tracks))
	copy(out, tracks)
	return out, nil
}

// PlayPlaylist implements music.Client.
func (c *Client) PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	known := false
	for _, p := range c.playlists {
		if p.Name == playlistName {
			known = true
			break
		}
	}
	if !known {
		return music.ErrPlaylistNotFound
	}
	c.PlayPlaylistCalls++
	c.playPlaylistRecord = append(c.playPlaylistRecord, PlayPlaylistCall{
		Name: playlistName, FromIdx: fromTrackIndex,
	})
	return nil
}

// SetLibraryTracks supplies the in-memory library searched by SearchTracks.
func (c *Client) SetLibraryTracks(tracks []domain.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.libraryTracks = tracks
}

// PlayTrackRecord returns a copy of the recorded PlayTrack invocations.
func (c *Client) PlayTrackRecord() []PlayTrackCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]PlayTrackCall, len(c.playTrackRecord))
	copy(out, c.playTrackRecord)
	return out
}

// SearchTracks implements music.Client. OR-matches case-insensitive substring
// across title/artist/album, caps at 100, returns Total = pre-cap match count.
func (c *Client) SearchTracks(ctx context.Context, query string) (music.SearchResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return music.SearchResult{}, c.forcedErr
	}
	if !c.running {
		return music.SearchResult{}, music.ErrNotRunning
	}
	q := strings.ToLower(query)
	var hits []domain.Track
	for _, t := range c.libraryTracks {
		if strings.Contains(strings.ToLower(t.Title), q) ||
			strings.Contains(strings.ToLower(t.Artist), q) ||
			strings.Contains(strings.ToLower(t.Album), q) {
			hits = append(hits, t)
		}
	}
	total := len(hits)
	if len(hits) > 100 {
		hits = hits[:100]
	}
	return music.SearchResult{Tracks: hits, Total: total}, nil
}

// PlayTrack implements music.Client. Sets now-playing to the matching track
// and flips IsPlaying on. ErrTrackNotFound if no library track has that ID.
func (c *Client) PlayTrack(ctx context.Context, persistentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	for _, t := range c.libraryTracks {
		if t.PersistentID == persistentID {
			c.PlayTrackCalls++
			c.playTrackRecord = append(c.playTrackRecord, PlayTrackCall{PersistentID: persistentID})
			c.hasTrack = true
			c.track = t
			c.duration = t.Duration
			c.position = 0
			c.playing = true
			return nil
		}
	}
	return music.ErrTrackNotFound
}

var _ music.Client = (*Client)(nil)
