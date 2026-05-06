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
		body = renderConnectedCardOnly(s, art, width, m.playlists)
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
// view.go composes the footer separately.
//
// Layout dispatch:
//   - narrow (width < artLayoutThreshold) or no art → text-only, today's behaviour.
//   - art present and tall enough to host Up Next → top-aligned join of
//     art and (text + Up Next), so Up Next anchors against the bottom of
//     the art and the text starts at the top.
//   - art present but no room for Up Next (art_height − text_height − 1 < 1)
//     → centered art+text join, today's behaviour, Up Next suppressed.
func renderConnectedCardOnly(s Connected, art string, width int, panel playlistsPanel) string {
	text := buildNowPlayingText(s)
	if width < artLayoutThreshold || art == "" {
		return text
	}

	artHeight := lipgloss.Height(art)
	textHeight := lipgloss.Height(text)
	queueRows := artHeight - textHeight - 1 // -1 for the "─ Up Next ─" header
	colWidth := rightColumnWidth(width, art)
	upNext := renderUpNext(s.Now, panel, queueRows, colWidth)
	if upNext == "" {
		// No room or no Up Next applicable — fall back to centered layout.
		return lipgloss.JoinHorizontal(lipgloss.Center, art, "  ", text)
	}
	rightCol := lipgloss.JoinVertical(lipgloss.Left, text, upNext)
	return lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", rightCol)
}

// buildNowPlayingText returns the title/artist/album/progress/volume block
// — the right-column content above any Up Next list. Identical to what
// renderConnectedCardOnly used to emit verbatim, just extracted.
func buildNowPlayingText(s Connected) string {
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

	return b.String()
}

// rightColumnWidth returns the column width available to the right of the
// album art for text + Up Next content, given the OUTER panel width.
// Accounts for the panel's chrome (border + padding = 4 cols total),
// the art's width, and the 2-col gap between art and right column.
// Without the panelChrome subtraction the Up Next header pad overflows
// the panel content area and lipgloss inside panelBox wraps the trailing
// "─" characters onto a new line, visible as a horizontal split in the
// rendered panel. Returns 0 if non-positive.
func rightColumnWidth(panelOuterWidth int, art string) int {
	const panelChrome = 4 // 1 left border + 1 left pad + 1 right pad + 1 right border
	const gap = 2         // the "  " between art and right column
	w := panelOuterWidth - panelChrome - lipgloss.Width(art) - gap
	if w < 0 {
		return 0
	}
	return w
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
		// A failed fetch leaves tracksByName empty but sets trackErrByName.
		// Surface that as "no queue" — otherwise the placeholder would be
		// stuck on "loading…" forever (handleStatus also gates on the same
		// error to stop retrying).
		if panel.trackErrByName[name] != nil {
			return placeholder("no queue")
		}
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
