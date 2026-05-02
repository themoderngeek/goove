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
	trimmed := strings.TrimSpace(raw)

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

	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	posSec, err := strconv.ParseFloat(lines[3], 64)
	if err != nil {
		return domain.NowPlaying{}, fmt.Errorf("%w: position parse: %v", music.ErrUnavailable, err)
	}
	durSec, err := strconv.ParseFloat(lines[4], 64)
	if err != nil {
		return domain.NowPlaying{}, fmt.Errorf("%w: duration parse: %v", music.ErrUnavailable, err)
	}
	vol, err := strconv.Atoi(lines[6])
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

// parseAirPlayDevices parses the tab-separated output of scriptAirPlayDevices.
// Special sentinel NOT_RUNNING maps to music.ErrNotRunning. Empty input
// (Music shows zero AirPlay devices — legitimate state) returns an empty slice.
func parseAirPlayDevices(raw string) ([]domain.AudioDevice, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "NOT_RUNNING" {
		return nil, music.ErrNotRunning
	}
	if trimmed == "" {
		return []domain.AudioDevice{}, nil
	}
	var devices []domain.AudioDevice
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			return nil, fmt.Errorf("%w: device line has %d fields, want 5: %q",
				music.ErrUnavailable, len(fields), line)
		}
		devices = append(devices, domain.AudioDevice{
			Name:      fields[0],
			Kind:      fields[1],
			Available: fields[2] == "true",
			Active:    fields[3] == "true",
			Selected:  fields[4] == "true",
		})
	}
	return devices, nil
}
