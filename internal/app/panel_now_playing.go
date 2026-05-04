package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderNowPlayingPanel renders the top panel for any AppState. The shape is
// identical to the previous renderConnected / renderIdle / renderDisconnected
// trio — they're just moved here under a single entry point so view.go can
// compose this panel beside the others.
//
// Phase 1: the panel still uses the existing card/border style. Phase 5
// adds optional album art on the left.
func renderNowPlayingPanel(m Model) string {
	switch s := m.state.(type) {
	case Connected:
		// Only show art when it's for the currently-playing track. After a
		// track skip, m.art.output may still hold the old track's render
		// until the new fetchArtwork lands; suppress it during that window.
		art := ""
		if m.art.key == trackKey(s.Now.Track) {
			art = m.art.output
		}
		return renderConnectedCardOnly(s, art, m.width)
	case Idle:
		return renderIdleCard(s.Volume)
	case Disconnected:
		return renderDisconnectedCard()
	}
	return ""
}

// renderConnectedCardOnly returns just the card (no footer / no error line).
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
	return cardStyle.Render(content)
}

func renderIdleCard(volume int) string {
	body := titleStyle.Render("Music is open, nothing playing.") + "\n\n" +
		subtitleStyle.Render("press space or n to start playback") + "\n\n" +
		"volume  " + volumeBar(volume, volumeBarWidth) + fmt.Sprintf("   %d%%", volume)
	return cardStyle.Render(body)
}

func renderDisconnectedCard() string {
	body := titleStyle.Render("Apple Music isn't running.") + "\n\n" +
		subtitleStyle.Render("press space to launch it, q to quit")
	return cardStyle.Render(body)
}
