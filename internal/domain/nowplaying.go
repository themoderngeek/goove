package domain

import "time"

type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration // populated by playlist tracks; left zero for NowPlaying.Track
}

type NowPlaying struct {
	Track        Track
	Position     time.Duration
	Duration     time.Duration
	IsPlaying    bool
	Volume       int
	LastSyncedAt time.Time
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
