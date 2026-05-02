package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

func cmdStatus(args []string, client music.Client, stdout, stderr io.Writer) int {
	_ = args // args reserved for Task 7 (--json flag)
	np, err := client.Status(context.Background())
	if err != nil {
		if errors.Is(err, music.ErrNoTrack) {
			fmt.Fprintln(stdout, "(no track loaded)")
			return 0
		}
		// status doesn't include the "run goove launch first" hint —
		// it's a state-query, not a state-mutator.
		return errorExit(err, stderr, false)
	}
	fmt.Fprintln(stdout, formatStatusPlain(np))
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
