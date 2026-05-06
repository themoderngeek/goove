package domain

import "time"

type Track struct {
	Title        string
	Artist       string
	Album        string
	Duration     time.Duration // populated by playlist tracks; left zero for NowPlaying.Track
	PersistentID string        // populated by search results, playlist tracks, and the now-playing track. Apple Music's stable per-library track handle, used by PlayTrack and to locate the playing track inside its playlist for the Up Next view.
}

type NowPlaying struct {
	Track        Track
	Position     time.Duration
	Duration     time.Duration
	IsPlaying    bool
	Volume       int
	LastSyncedAt time.Time

	// Queue context — populated by Status. CurrentPlaylistName is "" when
	// there is no playlist context (e.g. a track played via PlayTrack from
	// search results). May be the localised "Library" string when Music.app
	// reports the master library; not special-cased.
	CurrentPlaylistName string
	ShuffleEnabled      bool
}

func (n NowPlaying) DisplayedPosition(now time.Time) time.Duration {
	if !n.IsPlaying {
		return n.Position
	}
	elapsed := now.Sub(n.LastSyncedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	if n.Duration == 0 {
		return n.Position
	}
	pos := n.Position + elapsed
	if pos > n.Duration {
		return n.Duration
	}
	return pos
}
