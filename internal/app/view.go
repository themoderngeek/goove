package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	progressBarWidth = 20
	volumeBarWidth   = 10
	compactThreshold = 50
)

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

func renderConnected(s Connected, footer string) string {
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

	card := cardStyle.Render(b.String())
	keybinds := footerStyle.Render(" space: play/pause   n: next   p: prev   +/-: vol   q: quit")

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

// Stubs replaced in later tasks.
func renderIdle(volume int, footer string) string { return "" }
func renderDisconnected(footer string) string     { return "" }
func renderPermissionDenied() string              { return "" }
func renderCompact(m Model) string                { return "" }
