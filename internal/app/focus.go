package app

// focus identifies which panel currently owns keyboard input. Only the four
// focusable panels are listed; the now-playing panel at the top is read-only
// and is skipped by tab order.
type focus int

const (
	focusPlaylists focus = iota
	focusSearch
	focusOutput
	focusMain
)

// nextFocus cycles forward through Playlists → Search → Output → Main → Playlists.
func nextFocus(f focus) focus {
	return (f + 1) % 4
}

// prevFocus cycles backward.
func prevFocus(f focus) focus {
	return (f + 3) % 4
}
