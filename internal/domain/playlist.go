package domain

// Playlist is a user or subscription playlist surfaced by the music client.
// Kind is "user" or "subscription". Smart playlists, system playlists, and
// folders are excluded from the playlists feature scope (see
// docs/superpowers/specs/2026-05-03-playlists-design.md).
type Playlist struct {
	Name       string
	Kind       string
	TrackCount int
}
