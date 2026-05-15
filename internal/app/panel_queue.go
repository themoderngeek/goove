package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// overlayState carries the queue overlay's view-layer state. Lives on
// Model. Zero value = closed; cursor 0 = head row.
type overlayState struct {
	open   bool
	cursor int
}

var (
	overlayCursorStyle  = lipgloss.NewStyle().Reverse(true)
	overlayWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF5F"))
)

// renderOverlay renders the full-area queue overlay. Layout (top to
// bottom):
//
//  1. Header row: "Queue [N]" left, close hint right.
//  2. Divider.
//  3. Body: queue rows (cursor row prefixed ▶ and reversed) or empty
//     state when len == 0.
//  4. Divider.
//  5. Resume footer: "─ then resumes ─\n<playlist> · track N of M"
//     or "─ then stops ─" when resume is empty.
//  6. Help row (or clear prompt when m.clearPrompt).
func renderOverlay(m Model, width, height int) string {
	_ = height // height drives no truncation in V1 — body grows; if it overflows the terminal scrolls
	if width < 20 {
		width = 20
	}

	count := m.queue.Len()
	header := fmt.Sprintf("Queue [%d]", count)
	closeHint := subtitleStyle.Render("Q/esc to close")
	headerRow := padBetween(header, closeHint, width)

	var bodyLines []string
	if count == 0 {
		bodyLines = append(bodyLines, subtitleStyle.Render("(queue is empty — press a on a track to add)"))
	} else {
		for i, t := range m.queue.Items {
			marker := "  "
			if i == m.overlay.cursor {
				marker = "▶ "
			}
			row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
			row = truncate(row, width)
			if i == m.overlay.cursor {
				row = overlayCursorStyle.Render(row)
			}
			bodyLines = append(bodyLines, row)
		}
	}
	body := strings.Join(bodyLines, "\n")

	var resumeFooter string
	if m.resume.PlaylistName != "" {
		total := len(m.playlists.tracksByName[m.resume.PlaylistName])
		resumeFooter = subtitleStyle.Render("─ then resumes ─") + "\n" +
			subtitleStyle.Render(fmt.Sprintf("%s · track %d of %d", m.resume.PlaylistName, m.resume.NextIndex, total))
	} else {
		resumeFooter = subtitleStyle.Render("─ then stops ─")
	}

	var helpRow string
	if m.clearPrompt {
		helpRow = overlayWarningStyle.Render("Clear queue? press y to confirm, any other key to cancel")
	} else {
		helpRow = subtitleStyle.Render("j/k nav · enter play now · x remove · K/J reorder · c clear")
	}

	out := lipgloss.JoinVertical(lipgloss.Left,
		headerRow,
		strings.Repeat("─", width),
		body,
		strings.Repeat("─", width),
		resumeFooter,
		"",
		helpRow,
	)
	return out
}

// padBetween joins left + right with a run of spaces so the combined
// string fills exactly width columns. If left+right already exceeds
// width, returns left+" "+right (no truncation in V1).
func padBetween(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// updateOverlay handles keystrokes while the queue overlay is open. The
// overlay is fully modal — all keys land here, and globals (space, n,
// p, +/-, q-to-quit, Tab, digits, /, o) do not fire. When the user
// presses Esc or Q the overlay closes; thereafter the normal global
// key map applies.
func updateOverlay(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	// Clear-prompt mode absorbs the next keypress: y confirms, anything
	// else cancels. Esc/Q still close (handled below) — they implicitly
	// also cancel the prompt because we reset clearPrompt on close.
	if m.clearPrompt {
		s := msg.String()
		if s != "esc" && s != "Q" {
			if s == "y" {
				m.queue.Clear()
			}
			m.clearPrompt = false
			return m, nil
		}
	}

	switch msg.String() {
	case "esc", "Q":
		m.overlay.open = false
		m.clearPrompt = false
		return m, nil

	case "j", "down":
		if m.overlay.cursor < m.queue.Len()-1 {
			m.overlay.cursor++
		}
		return m, nil

	case "k", "up":
		if m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		return m, nil

	case "x":
		m.queue.RemoveAt(m.overlay.cursor)
		if m.overlay.cursor >= m.queue.Len() && m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		return m, nil

	case "K":
		m.overlay.cursor = m.queue.MoveUp(m.overlay.cursor)
		return m, nil

	case "J":
		m.overlay.cursor = m.queue.MoveDown(m.overlay.cursor)
		return m, nil

	case "c":
		m.clearPrompt = true
		return m, nil

	case "enter":
		if m.queue.Len() == 0 {
			return m, nil
		}
		if m.overlay.cursor < 0 || m.overlay.cursor >= m.queue.Len() {
			return m, nil
		}
		item := m.queue.Items[m.overlay.cursor]
		m.queue.RemoveAt(m.overlay.cursor)
		if m.overlay.cursor >= m.queue.Len() && m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		m.pendingJumpPID = item.PersistentID
		return m, playTrack(m.client, item.PersistentID)
	}
	return m, nil
}
