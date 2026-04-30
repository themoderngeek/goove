//go:build darwin

package applescript

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// parseStatus parses the seven-line output of scriptStatus into a NowPlaying.
// Special sentinels NOT_RUNNING and NO_TRACK are mapped to the corresponding
// sentinel errors. LastSyncedAt is left zero — the caller stamps it.
func parseStatus(raw string) (domain.NowPlaying, error) {
	trimmed := strings.TrimRight(raw, "\n")

	switch trimmed {
	case "NOT_RUNNING":
		return domain.NowPlaying{}, music.ErrNotRunning
	case "NO_TRACK":
		return domain.NowPlaying{}, music.ErrNoTrack
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) != 7 {
		return domain.NowPlaying{}, fmt.Errorf("%w: expected 7 lines, got %d", music.ErrUnavailable, len(lines))
	}

	posSec, err := strconv.ParseFloat(lines[3], 64)
	if err != nil {
		return domain.NowPlaying{}, fmt.Errorf("%w: position parse: %v", music.ErrUnavailable, err)
	}
	durSec, err := strconv.ParseFloat(lines[4], 64)
	if err != nil {
		return domain.NowPlaying{}, fmt.Errorf("%w: duration parse: %v", music.ErrUnavailable, err)
	}
	vol, err := strconv.Atoi(strings.TrimSpace(lines[6]))
	if err != nil {
		return domain.NowPlaying{}, fmt.Errorf("%w: volume parse: %v", music.ErrUnavailable, err)
	}

	return domain.NowPlaying{
		Track: domain.Track{
			Title:  lines[0],
			Artist: lines[1],
			Album:  lines[2],
		},
		Position:  time.Duration(posSec * float64(time.Second)),
		Duration:  time.Duration(durSec * float64(time.Second)),
		IsPlaying: lines[5] == "playing",
		Volume:    vol,
	}, nil
}
