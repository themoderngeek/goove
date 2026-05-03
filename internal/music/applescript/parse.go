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

// parsePlaylists parses the tab-separated output of scriptPlaylists. Each line
// has three fields: name, kind ("user" | "subscription"), track_count.
//
// NOT_RUNNING maps to music.ErrNotRunning. Empty input returns an empty slice
// (legitimate state — Music has no playlists). Rows with empty names are
// skipped (Music permits a "" playlist name but `play playlist ""` errors).
// Rows with the wrong field count are skipped defensively.
func parsePlaylists(raw string) ([]domain.Playlist, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "NOT_RUNNING" {
		return nil, music.ErrNotRunning
	}
	if trimmed == "" {
		return []domain.Playlist{}, nil
	}
	var playlists []domain.Playlist
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		if fields[0] == "" {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil {
			continue
		}
		playlists = append(playlists, domain.Playlist{
			Name:       fields[0],
			Kind:       fields[1],
			TrackCount: count,
		})
	}
	return playlists, nil
}

// parsePlaylistTracks parses the tab-separated output of scriptPlaylistTracks.
// Each line has four fields: title, artist, album, duration_seconds.
//
// NOT_RUNNING → music.ErrNotRunning. NOT_FOUND → music.ErrPlaylistNotFound.
// Empty input returns an empty slice. Malformed rows (wrong field count or
// non-numeric duration) are skipped defensively.
func parsePlaylistTracks(raw string) ([]domain.Track, error) {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "NOT_RUNNING":
		return nil, music.ErrNotRunning
	case "NOT_FOUND":
		return nil, music.ErrPlaylistNotFound
	}
	if trimmed == "" {
		return []domain.Track{}, nil
	}
	var tracks []domain.Track
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		secs, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err != nil {
			continue
		}
		tracks = append(tracks, domain.Track{
			Title:    fields[0],
			Artist:   fields[1],
			Album:    fields[2],
			Duration: time.Duration(secs * float64(time.Second)),
		})
	}
	return tracks, nil
}

// parseSearchTracks parses scriptSearchTracks output. The first line is the
// total underlying match count; following lines (up to 100) are tab-separated
// track records. NOT_RUNNING maps to ErrNotRunning. Malformed rows (wrong
// field count or non-numeric duration) are skipped defensively.
func parseSearchTracks(raw string) ([]domain.Track, int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "NOT_RUNNING" {
		return nil, 0, music.ErrNotRunning
	}
	lines := strings.Split(trimmed, "\n")
	total, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: search total parse: %v", music.ErrUnavailable, err)
	}
	var tracks []domain.Track
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			continue
		}
		secs, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		if err != nil {
			continue
		}
		tracks = append(tracks, domain.Track{
			PersistentID: fields[0],
			Title:        fields[1],
			Artist:       fields[2],
			Album:        fields[3],
			Duration:     time.Duration(secs * float64(time.Second)),
		})
	}
	return tracks, total, nil
}

// matchAirPlayDevice picks the single device whose Name matches the user's input.
// Exact match (case-sensitive) wins immediately; otherwise case-insensitive
// substring match. Returns ErrDeviceNotFound if no matches; ErrAmbiguousDevice
// if multiple substring matches.
func matchAirPlayDevice(devices []domain.AudioDevice, name string) (domain.AudioDevice, error) {
	for _, d := range devices {
		if d.Name == name {
			return d, nil
		}
	}
	lower := strings.ToLower(name)
	var matches []domain.AudioDevice
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), lower) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return domain.AudioDevice{}, music.ErrDeviceNotFound
	}
	if len(matches) > 1 {
		return domain.AudioDevice{}, music.ErrAmbiguousDevice
	}
	return matches[0], nil
}
