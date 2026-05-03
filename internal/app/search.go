package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/themoderngeek/goove/internal/music"
)

// renderSearch is the modal overlay shown when m.search != nil.
// Replaces the player view entirely (no side-by-side composition), matching
// the picker pattern.
func renderSearch(s *searchState) string {
	var body strings.Builder
	body.WriteString("> ")
	body.WriteString(s.query)
	body.WriteString("_")
	body.WriteString("\n")
	body.WriteString(strings.Repeat("─", 46))
	body.WriteString("\n\n")

	switch {
	case s.query == "":
		body.WriteString(subtitleStyle.Render("type to search your library"))
	case s.loading:
		body.WriteString(subtitleStyle.Render("searching…"))
	case len(s.results) == 0:
		body.WriteString(subtitleStyle.Render("no matches in your library"))
	default:
		for i, t := range s.results {
			cursor := " "
			if i == s.cursor {
				cursor = "▶"
			}
			body.WriteString(fmt.Sprintf("  %s %s\n", cursor, titleStyle.Render(t.Title)))
			body.WriteString("    ")
			body.WriteString(subtitleStyle.Render(t.Artist + " · " + t.Album))
			if i < len(s.results)-1 {
				body.WriteString("\n\n")
			}
		}
		body.WriteString("\n\n")
		if s.total > len(s.results) {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("… %d of %d — refine the query", len(s.results), s.total)))
		} else {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("%d results", s.total)))
		}
	}

	header := titleStyle.Render("search")
	card := cardStyle.Render(header + "\n\n" + body.String())

	footerText := " ⏎ play   esc cancel"
	if len(s.results) > 0 {
		footerText = " ↑/↓ navigate   ⏎ play   r refresh   esc cancel"
	}
	footer := footerStyle.Render(footerText)

	out := card + "\n" + footer
	if s.err != nil {
		// Override the footer label to "retry" while an error is showing.
		errFooter := errorStyle.Render("error: " + s.err.Error())
		footerText = " ⏎ play   r retry   esc cancel"
		out = card + "\n" + footerStyle.Render(footerText) + "\n" + errFooter
	}
	return lipgloss.NewStyle().Margin(0, 2).Render(out)
}

// playSearchSelection invokes PlayTrack for the highlighted result.
func playSearchSelection(client music.Client, seq uint64, persistentID string) tea.Cmd {
	return func() tea.Msg {
		return searchPlayedMsg{seq: seq, err: client.PlayTrack(context.Background(), persistentID)}
	}
}

// handleSearchKey routes keystrokes when the search modal is open. Transport
// keys do NOT fall through (unlike the browser); the modal is fully captive
// the way the picker is.
// NOTE: 'r' is treated as refresh inside the modal (not appended to the query).
// This is a documented trade-off: queries containing 'r' will trigger a refresh
// instead of appending the character.
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.search = nil
		return m, nil
	case tea.KeyBackspace:
		if len(m.search.query) == 0 {
			return m, nil
		}
		runes := []rune(m.search.query)
		m.search.query = string(runes[:len(runes)-1])
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	case tea.KeyUp:
		if m.search.cursor > 0 {
			m.search.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.search.cursor < len(m.search.results)-1 {
			m.search.cursor++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.search.results) == 0 {
			return m, nil
		}
		pid := m.search.results[m.search.cursor].PersistentID
		return m, playSearchSelection(m.client, m.search.seq, pid)
	case tea.KeySpace:
		m.search.query += " "
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	case tea.KeyRunes:
		// Single-rune special-case: 'r' is refresh; everything else appends.
		if len(msg.Runes) == 1 && msg.Runes[0] == 'r' {
			if m.search.query == "" {
				return m, nil
			}
			m.search.seq++
			m.search.loading = true
			m.search.err = nil
			return m, fetchSearch(m.client, m.search.seq, m.search.query)
		}
		m.search.query += string(msg.Runes)
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	}
	return m, nil
}

const searchDebounceDuration = 250 * time.Millisecond

// scheduleSearchDebounce returns a tea.Tick Cmd that emits a searchDebounceMsg
// stamped with the given seq.
func scheduleSearchDebounce(seq uint64) tea.Cmd {
	return tea.Tick(searchDebounceDuration, func(time.Time) tea.Msg {
		return searchDebounceMsg{seq: seq}
	})
}

// fetchSearch invokes SearchTracks in a goroutine and emits a searchResultsMsg.
func fetchSearch(client music.Client, seq uint64, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := client.SearchTracks(context.Background(), query)
		return searchResultsMsg{seq: seq, query: query, result: res, err: err}
	}
}
