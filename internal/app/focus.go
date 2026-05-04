package app

// focusKind identifies which panel currently owns keyboard input. Only the four
// focusable panels are listed; the now-playing panel at the top is read-only
// and is skipped by tab order.
type focusKind int

const (
	focusPlaylists focusKind = iota
	focusSearch
	focusOutput
	focusMain
)

// numFocusPanels is the number of focusable panels — used by nextFocus /
// prevFocus to wrap the cycle. If a panel is added here, update this too.
const numFocusPanels = 4

// nextFocus cycles forward through Playlists → Search → Output → Main → Playlists.
func nextFocus(f focusKind) focusKind {
	return (f + 1) % numFocusPanels
}

// prevFocus cycles backward through Main → Output → Search → Playlists → Main.
func prevFocus(f focusKind) focusKind {
	return (f + numFocusPanels - 1) % numFocusPanels
}
