package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// statusJSON is the wire format for `goove status --json`. Pointer fields
// with omitempty get omitted from output when nil — used for Idle state where
// no track / position / duration / volume is available via music.Client.
type statusJSON struct {
	IsPlaying   bool      `json:"is_playing"`
	Track       *trackRef `json:"track"`
	PositionSec *int      `json:"position_sec,omitempty"`
	DurationSec *int      `json:"duration_sec,omitempty"`
	Volume      *int      `json:"volume,omitempty"`
}

type trackRef struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
}

func cmdStatus(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	np, err := client.Status(context.Background())
	if err != nil {
		if errors.Is(err, music.ErrNoTrack) {
			if jsonOutput {
				return printStatusIdleJSON(stdout)
			}
			fmt.Fprintln(stdout, "(no track loaded)")
			return 0
		}
		// status doesn't include the "run goove launch first" hint —
		// it's a state-query, not a state-mutator.
		return errorExit(err, stderr, false)
	}

	if jsonOutput {
		return printStatusConnectedJSON(stdout, np)
	}
	fmt.Fprintln(stdout, formatStatusPlain(np))
	return 0
}

func printStatusConnectedJSON(stdout io.Writer, np domain.NowPlaying) int {
	pos := int(np.Position.Seconds())
	dur := int(np.Duration.Seconds())
	vol := np.Volume
	out := statusJSON{
		IsPlaying:   np.IsPlaying,
		Track:       &trackRef{Title: np.Track.Title, Artist: np.Track.Artist, Album: np.Track.Album},
		PositionSec: &pos,
		DurationSec: &dur,
		Volume:      &vol,
	}
	enc := json.NewEncoder(stdout)
	if err := enc.Encode(out); err != nil {
		// json.Encode appends a newline; on encode failure (extremely rare for
		// fixed-shape structs), surface a non-zero exit so scripts notice.
		return 1
	}
	return 0
}

func printStatusIdleJSON(stdout io.Writer) int {
	out := statusJSON{
		IsPlaying: false,
		Track:     nil,
		// PositionSec/DurationSec/Volume left nil — omitempty drops them.
	}
	enc := json.NewEncoder(stdout)
	if err := enc.Encode(out); err != nil {
		return 1
	}
	return 0
}

// formatStatusPlain returns a one-line human-readable status string.
// Format:  "▶ Title — Artist (1:01 / 3:06) vol 21%"
// Artist segment is omitted when np.Track.Artist is empty.
func formatStatusPlain(np domain.NowPlaying) string {
	state := "▶"
	if !np.IsPlaying {
		state = "⏸"
	}
	titleAndArtist := np.Track.Title
	if np.Track.Artist != "" {
		titleAndArtist = fmt.Sprintf("%s — %s", np.Track.Title, np.Track.Artist)
	}
	pos := int(np.Position.Seconds())
	dur := int(np.Duration.Seconds())
	return fmt.Sprintf("%s %s (%s / %s) vol %d%%",
		state,
		titleAndArtist,
		formatDuration(pos),
		formatDuration(dur),
		np.Volume,
	)
}

// formatDuration formats a number of seconds as "M:SS".
// Local helper to avoid a cross-package dependency on internal/app's formatDuration.
func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
