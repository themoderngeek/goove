package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPicker is the modal overlay shown when m.picker != nil.
// Replaces the player view entirely (no side-by-side composition).
func renderPicker(p *pickerState) string {
	var body strings.Builder

	if p.loading {
		body.WriteString("Loading devices...")
	} else if len(p.devices) == 0 {
		body.WriteString("(no AirPlay devices visible)")
	} else {
		// Compute the longest name for left-alignment.
		maxName := 0
		for _, d := range p.devices {
			if len(d.Name) > maxName {
				maxName = len(d.Name)
			}
		}
		for i, d := range p.devices {
			selectedMark := " "
			if d.Selected {
				selectedMark = "*"
			}
			cursorMark := " "
			if i == p.cursor {
				cursorMark = "▶"
			}
			line := fmt.Sprintf("  %s%s %-*s (%s)",
				selectedMark, cursorMark, maxName, d.Name, d.Kind)
			if !d.Available {
				line += "  unavailable"
			}
			body.WriteString(line)
			if i < len(p.devices)-1 {
				body.WriteString("\n")
			}
		}
	}

	if p.err != nil {
		body.WriteString("\n\n")
		body.WriteString(errorStyle.Render("error: " + p.err.Error()))
	}

	header := titleStyle.Render("Pick an output device")
	card := cardStyle.Render(header + "\n\n" + body.String())

	var footerText string
	if p.loading {
		footerText = " esc cancel"
	} else {
		footerText = " ↑/↓ navigate   enter select   esc cancel"
	}
	footer := footerStyle.Render(footerText)

	return lipgloss.NewStyle().Margin(0, 2).Render(card + "\n" + footer)
}
