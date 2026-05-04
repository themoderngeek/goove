package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func renderOutputPanel(m Model, width, height int) string {
	title := "Output"
	body := renderOutputBody(m, width, height)
	return panelBox(title, body, width, height, m.focusZ == focusOutput)
}

func renderOutputBody(m Model, width, height int) string {
	if m.output.loading && len(m.output.devices) == 0 {
		return subtitleStyle.Render("loading…")
	}
	if len(m.output.devices) == 0 {
		return subtitleStyle.Render("(no devices)")
	}
	visibleRows := height - 2 // top border + bottom border (title is now in the border)
	if visibleRows < 1 {
		visibleRows = 1
	}
	start := scrollWindow(m.output.cursor, visibleRows, len(m.output.devices))

	var sb strings.Builder
	for i := start; i < len(m.output.devices) && i-start < visibleRows; i++ {
		marker := "  "
		if i == m.output.cursor && m.focusZ == focusOutput {
			marker = "▶ "
		} else if m.output.devices[i].Selected {
			marker = "● "
		}
		sb.WriteString(truncate(marker+m.output.devices[i].Name, width-4))
		if i-start < visibleRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// onFocusOutput fetches devices on first focus, no-op when cached.
func onFocusOutput(m Model) (Model, tea.Cmd) {
	if len(m.output.devices) > 0 || m.output.loading {
		return m, nil
	}
	m.output.loading = true
	return m, fetchDevices(m.client)
}

func handleOutputKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if m.output.cursor > 0 {
			m.output.cursor--
		}
		return m, nil, true
	case "down", "j":
		if m.output.cursor < len(m.output.devices)-1 {
			m.output.cursor++
		}
		return m, nil, true
	case "enter":
		if len(m.output.devices) == 0 {
			return m, nil, true
		}
		m.output.loading = true
		target := m.output.devices[m.output.cursor].Name
		client := m.client
		return m, func() tea.Msg {
			return deviceSetMsg{err: client.SetAirPlayDevice(context.Background(), target)}
		}, true
	}
	return m, nil, false
}
