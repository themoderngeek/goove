package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/themoderngeek/goove/internal/domain"
)

// renderNowPlayingPanel renders the top panel for any AppState, wrapped in a
// panelBox with "Now Playing" as the title-in-border. Width is dictated by the
// caller so the panel can line up with the bottom row's right edge.
func renderNowPlayingPanel(m Model, width int) string {
	if width <= 0 {
		width = 100
	}
	var body string
	switch s := m.state.(type) {
	case Connected:
		// Only show art when it's for the currently-playing track. After a
		// track skip, m.art.output may still hold the old track's render
		// until the new fetchArtwork lands; suppress it during that window.
		art := ""
		if m.art.key == trackKey(s.Now.Track) {
			art = m.art.output
		}
		body = renderConnectedCardOnly(s, art, width)
	case Idle:
		body = renderIdleCard(s.Volume)
	case Disconnected:
		body = renderDisconnectedCard()
	default:
		return ""
	}
	height := lipgloss.Height(body) + 2 // border top + bottom
	return panelBox("Now Playing", body, width, height, false)
}

// renderConnectedCardOnly returns just the body content (no border wrapper).
// view.go composes the footer separately. Same content as renderConnectedCard
// but no margin wrapping (the parent does that).
func renderConnectedCardOnly(s Connected, art string, width int) string {
	pos := s.Now.DisplayedPosition(time.Now())
	var b strings.Builder

	state := "▶"
	if !s.Now.IsPlaying {
		state = "⏸"
	}

	b.WriteString(titleStyle.Render(state + "  " + s.Now.Track.Title))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(s.Now.Track.Artist))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(s.Now.Track.Album))
	b.WriteString("\n\n")
	b.WriteString(progressBar(pos, s.Now.Duration, progressBarWidth))
	b.WriteString("   ")
	b.WriteString(formatDuration(pos))
	b.WriteString(" / ")
	b.WriteString(formatDuration(s.Now.Duration))
	b.WriteString("\n\n")
	b.WriteString("volume  ")
	b.WriteString(volumeBar(s.Now.Volume, volumeBarWidth))
	b.WriteString(fmt.Sprintf("   %d%%", s.Now.Volume))

	content := b.String()
	if width >= artLayoutThreshold && art != "" {
		content = lipgloss.JoinHorizontal(lipgloss.Center, art, "  ", content)
	}
	return content
}

func renderIdleCard(volume int) string {
	return titleStyle.Render("Music is open, nothing playing.") + "\n\n" +
		subtitleStyle.Render("press space or n to start playback") + "\n\n" +
		"volume  " + volumeBar(volume, volumeBarWidth) + fmt.Sprintf("   %d%%", volume)
}

func renderDisconnectedCard() string {
	return titleStyle.Render("Apple Music isn't running.") + "\n\n" +
		subtitleStyle.Render("press space to launch it, q to quit")
}

// renderUpNext renders the Up Next block for the now-playing panel:
// a "─ Up Next ─" header followed by either a list of upcoming tracks
// or a single placeholder line. Returns "" to signal "skip Up Next, fall
// back to the existing centered layout" — the caller treats that as a
// no-op.
//
// rows is the number of body rows available (after the header). width is
// the column width available for the block. Returns "" when rows < 1 or
// width < 1.
//
// State dispatch (matches spec §3.3):
//   - shuffle on              → "shuffling — next track unpredictable"
//   - no playlist context     → "no queue"
//   - cache miss (any reason) → "loading…"
//   - cached, no error, current track found, more upcoming → tracks
//   - cached, error           → "no queue"
//   - cached, current at end  → "end of playlist"
//   - cached, current missing → "no queue"
func renderUpNext(now domain.NowPlaying, panel playlistsPanel, rows, width int) string {
	if rows < 1 || width < 1 {
		return ""
	}
	headerLabel := "─ Up Next "
	var header string
	if utf8.RuneCountInString(headerLabel) >= width {
		header = subtitleStyle.Render(truncate(headerLabel, width))
	} else {
		pad := strings.Repeat("─", width-utf8.RuneCountInString(headerLabel))
		header = subtitleStyle.Render(headerLabel + pad)
	}

	body := upNextBody(now, panel, rows, width)
	if body == "" {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// upNextBody returns the rendered body of the Up Next block — either a
// placeholder line or up to `rows` track rows. Returns "" only when the
// caller should not render the block at all (currently never — every
// state has a body).
func upNextBody(now domain.NowPlaying, panel playlistsPanel, rows, width int) string {
	placeholder := func(s string) string { return subtitleStyle.Render(truncate(s, width)) }

	if now.ShuffleEnabled {
		return placeholder("shuffling — next track unpredictable")
	}
	name := now.CurrentPlaylistName
	if name == "" {
		return placeholder("no queue")
	}
	tracks, cached := panel.tracksByName[name]
	if !cached {
		return placeholder("loading…")
	}
	if panel.trackErrByName[name] != nil {
		return placeholder("no queue")
	}
	pid := now.Track.PersistentID
	if pid == "" {
		return placeholder("no queue")
	}
	idx := -1
	for i := range tracks {
		if tracks[i].PersistentID == pid {
			idx = i
			break
		}
	}
	if idx == -1 {
		return placeholder("no queue")
	}
	if idx == len(tracks)-1 {
		return placeholder("end of playlist")
	}
	upcoming := tracks[idx+1:]
	if len(upcoming) > rows {
		upcoming = upcoming[:rows]
	}
	var sb strings.Builder
	for i, t := range upcoming {
		if i > 0 {
			sb.WriteString("\n")
		}
		row := fmt.Sprintf("%d. %s — %s", idx+2+i, t.Title, t.Artist)
		sb.WriteString(truncate(row, width))
	}
	return sb.String()
}
