package app

func renderSearchPanel(m Model, width, height int) string {
	title := "Search"
	body := subtitleStyle.Render("/  type to search")
	return panelBox(title, body, width, height, m.focusZ == focusSearch)
}
