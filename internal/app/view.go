package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/themoderngeek/goove/internal/domain"
)

const (
	progressBarWidth = 20
	volumeBarWidth   = 10
	compactThreshold = 50
)

const connectedKeybindsText = " space: play/pause   n: next   p: prev   +/-: vol   q: quit"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	subtitleStyle = lipgloss.NewStyle().Faint(true)
	footerStyle   = lipgloss.NewStyle().Faint(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F"))
	cardStyle     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)
)

func (m Model) View() string {
	if m.permissionDenied {
		return renderPermissionDenied()
	}
	if m.width > 0 && m.width < compactThreshold {
		return renderCompact(m)
	}
	switch s := m.state.(type) {
	case Connected:
		if m.width >= artLayoutThreshold &&
			m.art.output != "" &&
			m.art.key == trackKey(s.Now.Track) {
			cardOnly := renderConnectedCard(s)
			artBlock := lipgloss.NewStyle().PaddingTop(1).Render(m.art.output)
			composite := lipgloss.JoinHorizontal(lipgloss.Center, artBlock, "  ", cardOnly)
			keybinds := footerStyle.Render(connectedKeybindsText)
			out := composite + "\n" + keybinds
			if errFooter := m.errFooter(); errFooter != "" {
				out += "\n" + errFooter
			}
			return lipgloss.NewStyle().Margin(0, 2).Render(out)
		}
		return renderConnected(s, m.errFooter())
	case Idle:
		return renderIdle(s.Volume, m.errFooter())
	case Disconnected:
		return renderDisconnected(m.errFooter())
	}
	return ""
}

func (m Model) errFooter() string {
	if m.lastError == nil {
		return ""
	}
	return errorStyle.Render("error: " + m.lastError.Error())
}

// renderConnectedCard returns just the rounded-border card box for the
// Connected state — no keybinds, no error footer. Used by View for the
// art+card composite layout where the keybinds need to span full-width.
func renderConnectedCard(s Connected) string {
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

	return cardStyle.Render(b.String())
}

func renderConnected(s Connected, footer string) string {
	card := renderConnectedCard(s)
	keybinds := footerStyle.Render(connectedKeybindsText)

	out := card + "\n" + keybinds
	if footer != "" {
		out += "\n" + footer
	}
	return out
}

func progressBar(pos, dur time.Duration, width int) string {
	if dur <= 0 {
		return strings.Repeat("▯", width)
	}
	frac := float64(pos) / float64(dur)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	return strings.Repeat("▮", filled) + strings.Repeat("▯", width-filled)
}

func volumeBar(percent, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := percent * width / 100
	return strings.Repeat("▮", filled) + strings.Repeat("▯", width-filled)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func renderIdle(volume int, footer string) string {
	body := titleStyle.Render("Music is open, nothing playing.") + "\n\n" +
		subtitleStyle.Render("press space or n to start playback") + "\n\n" +
		"volume  " + volumeBar(volume, volumeBarWidth) + fmt.Sprintf("   %d%%", volume)
	card := cardStyle.Render(body)
	keybinds := footerStyle.Render(" space: play/pause   n: next   +/-: vol   q: quit")
	out := card + "\n" + keybinds
	if footer != "" {
		out += "\n" + footer
	}
	return out
}

func renderDisconnected(footer string) string {
	body := titleStyle.Render("Apple Music isn't running.") + "\n\n" +
		subtitleStyle.Render("press space to launch it, q to quit")
	card := cardStyle.Render(body)
	out := card
	if footer != "" {
		out += "\n" + footer
	}
	return out
}

func renderPermissionDenied() string {
	body := titleStyle.Render("Apple Music has blocked goove from controlling it.") + "\n\n" +
		subtitleStyle.Render(
			"  Open  System Settings → Privacy & Security → Automation\n"+
				"  Find  goove (or your terminal app)\n"+
				"  Toggle on  Music\n\n"+
				"  Then quit and re-run goove.",
		)
	card := cardStyle.Render(body)
	keybinds := footerStyle.Render(" q: quit")
	return card + "\n" + keybinds
}

func renderCompact(m Model) string {
	switch s := m.state.(type) {
	case Connected:
		state := "▶"
		if !s.Now.IsPlaying {
			state = "⏸"
		}
		line := fmt.Sprintf("%s %s — %s   vol %d%%",
			state, s.Now.Track.Title, s.Now.Track.Artist, s.Now.Volume)
		footer := footerStyle.Render("space n p +/- q")
		out := line + "\n" + footer
		if e := m.errFooter(); e != "" {
			out += "\n" + e
		}
		return out
	case Idle:
		return "Music idle.   space:play  q:quit\n"
	case Disconnected:
		return "Music not running.   space:launch  q:quit\n"
	}
	return ""
}

// trackKey returns a stable identity for a track for cache-keying purposes.
// Returns "" for an all-zero Track so cache lookups against "no track loaded"
// never accidentally match a real entry.
func trackKey(t domain.Track) string {
	if t.Title == "" && t.Artist == "" && t.Album == "" {
		return ""
	}
	return t.Title + "|" + t.Artist + "|" + t.Album
}

// currentArtKey returns trackKey for the current Connected state, or "" otherwise.
func (m Model) currentArtKey() string {
	if c, ok := m.state.(Connected); ok {
		return trackKey(c.Now.Track)
	}
	return ""
}
