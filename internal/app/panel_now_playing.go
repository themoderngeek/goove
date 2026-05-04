package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
