package app

import "strings"

// globalKeysHint is shown on every hint bar.
const globalKeysHint = "space:play/pause  n:next  p:prev  +/-:vol  q:quit"

// renderHintBar returns the bottom-of-screen hint string. Always includes
// the global keys; appends panel-scoped hints based on m.focus. Style is
// applied by view.go (footerStyle.Render).
func renderHintBar(m Model) string {
	var sb strings.Builder
	sb.WriteString(globalKeysHint)
	sb.WriteString("  ·  ")
	sb.WriteString(panelHint(m))
	return sb.String()
}

// panelHint returns just the focused panel's keys (no globals). Split out so
// it can be tested in isolation and so future overflow-handling can drop it
// at narrow widths.
func panelHint(m Model) string {
	switch m.focus {
	case focusPlaylists:
		return "j/k:nav  ⏎:play"
	case focusSearch:
		if m.search.inputMode {
			return "⏎:run  esc:clear"
		}
		return "type to search"
	case focusOutput:
		return "j/k:nav  ⏎:switch"
	case focusMain:
		if m.main.mode == mainPaneSearchResults {
			return "j/k:nav  ⏎:play  esc:back"
		}
		return "j/k:nav  ⏎:play"
	}
	return ""
}
