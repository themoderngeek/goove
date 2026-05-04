package app

func renderOutputPanel(m Model, width, height int) string {
	title := "Output"
	body := subtitleStyle.Render("—")
	return panelBox(title, body, width, height, m.focusZ == focusOutput)
}
