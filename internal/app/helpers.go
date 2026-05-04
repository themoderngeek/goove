package app

import "unicode/utf8"

// scrollWindow returns the top-of-window index such that cursor is visible
// within a viewport of size visible across total items. Cursor stays roughly
// centred when possible.
func scrollWindow(cursor, visible, total int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := cursor - half
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start
}

// truncate trims s to width runes, appending an ellipsis if it would exceed.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width <= 1 {
		_, size := utf8.DecodeRuneInString(s)
		return s[:size]
	}
	i, count := 0, 0
	for i < len(s) && count < width-1 {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		count++
	}
	return s[:i] + "…"
}
