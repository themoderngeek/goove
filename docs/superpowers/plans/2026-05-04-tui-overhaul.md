# goove — TUI overhaul (LazyGit-inspired) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace goove's current single-screen now-playing view + three modal overlays (search, output picker, playlist browser) with a persistent four-zone LazyGit-inspired layout: now-playing on top, Playlists/Search/Output stacked left, main pane right.

**Architecture:** Six-phase migration inside `internal/app/` only. CLI / domain / music client / AppleScript layers are unchanged. Each phase is self-contained, leaves the app working, and is independently shippable. Spec: `docs/superpowers/specs/2026-05-04-tui-overhaul-design.md`.

**Tech Stack:** Go, Bubble Tea (Elm Architecture), lipgloss for rendering, AppleScript via `osascript`, table-driven tests, chafa for album art.

---

## File map

**New files (all in `internal/app/`)**

| Path | Phase | Purpose |
|---|---|---|
| `panel_now_playing.go` | 1 | Renders the now-playing panel (top, full width). Album art added in Phase 5. |
| `panel_now_playing_test.go` | 1 / 5 | Tests for placeholder, idle, disconnected; art tests added in Phase 5. |
| `panel_playlists.go` | 1 / 2 | Playlists panel: state struct, fetch, render, key handler. Phase 1 = placeholder; Phase 2 = real. |
| `panel_playlists_test.go` | 2 | Cursor, ⏎, live-preview wiring. |
| `panel_search.go` | 1 / 3 | Search panel: state, render, key handler. Phase 1 = placeholder; Phase 3 = real. |
| `panel_search_test.go` | 3 | Input mode, debounce, ⏎ fires search. |
| `panel_output.go` | 1 / 4 | Output panel: state, render, key handler. Phase 1 = placeholder; Phase 4 = real. |
| `panel_output_test.go` | 4 | Cursor, ⏎ switches device. |
| `panel_main.go` | 1 / 2 / 3 | Main pane: mode switch (`mainPaneTracks` / `mainPaneSearchResults`), render, key handler. |
| `panel_main_test.go` | 2 / 3 | Mode flip, Esc semantics, ⏎ plays. |
| `hints.go` | 1 | Bottom hint bar — global keys + focused panel's keys. |
| `hints_test.go` | 1 | Hint composition by focus. |
| `focus.go` | 1 | `focus` enum and a tiny `nextFocus` / `prevFocus` helper. |
| `focus_test.go` | 1 | Focus cycling tests. |

**Modified files**

| Path | Phases | Notes |
|---|---|---|
| `internal/app/model.go` | 1, 2, 3, 4, 6 | Add `focus`, panel state fields; remove modal state types in later phases. |
| `internal/app/update.go` | 1, 2, 3, 4, 6 | Add focus-routed key dispatch; remove modal handlers in later phases. |
| `internal/app/view.go` | 1, 6 | Phase 1 routes to new layout when no modal is open; Phase 6 strips out the modal short-circuits. |
| `internal/app/messages.go` | 6 | Remove `searchPlayedMsg` (search modal-only) in Phase 6 cleanup. |
| `internal/app/search.go` | 3, 6 | Phase 3 keeps it temporarily as the modal; Phase 6 deletes it. |
| `internal/app/browser.go` | 2 | Deleted at end of Phase 2. |
| `internal/app/picker.go` | 4 | Deleted at end of Phase 4. |
| `internal/app/update_test.go` | 1, 2, 3, 4, 6 | Add new tests; migrate / delete modal tests. |
| `internal/app/search_test.go` | 3, 6 | Migrate / delete in Phase 3 / 6. |
| `README.md` | 6 | New screenshot, new keys table. |
| `cmd/goove/main.go` | 6 | Update `goove help` if it has any TUI key references. |

**Deleted files (Phase 6)**
- `internal/app/browser.go` (deleted in Phase 2 actually — listed here for completeness)
- `internal/app/picker.go` (deleted in Phase 4)
- `internal/app/search.go` (deleted in Phase 3)

## Conventions

- All Go files use a single grouped `import (...)` block at the top. When tasks below say "add the `tea` import" or similar, add the line into the existing grouped block — don't introduce a second `import` statement.
- All new tests use the `package app` (white-box) style and the `newTestModel()` helper in `update_test.go`.
- Commit messages follow the existing style: lowercase scope prefix (`app:` / `applescript:` / `readme:`), one-line subject, optional body, footer with the Co-Authored-By line.

## Open questions from the spec — resolutions

The design spec (§11) flagged four questions for the plan to resolve. Resolutions:

1. **`g`/`G`/`Ctrl-d`/`Ctrl-u` in the main pane.** **Dropped for v1.** Not in any task. Add later if users miss them.
2. **Minimum terminal width.** **Reuse the existing `compactThreshold` (50 cols).** Below that, a centred "terminal too narrow" message replaces the full layout. See Task 29.
3. **Hint bar overflow at narrow widths.** **Accept overflow for v1** — the hint bar can wrap or be cut off below ~80 cols. Add explicit truncation later if it becomes a problem.
4. **Refresh keys.** **Omit for v1.** No `r` / `Ctrl-R` keybind in any panel. The status tick keeps now-playing fresh; playlist/output panels re-fetch when refocused if their cache is empty.

---

## Task 1: Feature branch + commit spec & plan

Spec is already on disk and committed. Plan needs a feature branch + commit before implementation begins.

**Files:** none modified directly.

- [ ] **Step 1: Create the feature branch**

```bash
git checkout -b feature/tui-overhaul
```

Expected output: `Switched to a new branch 'feature/tui-overhaul'`.

- [ ] **Step 2: Commit the plan**

```bash
git add docs/superpowers/plans/2026-05-04-tui-overhaul.md
git commit -m "$(cat <<'EOF'
plan: TUI overhaul implementation plan

Six-phase migration of internal/app/: layout shell → playlists+main →
search → output → album art → cleanup. Each phase ships independently.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Expected output: a commit on `feature/tui-overhaul` with the plan added.

- [ ] **Step 3: Verify**

```bash
git log --oneline -2
```

Expected output: top entry is `<hash> plan: TUI overhaul implementation plan`; previous entry is the spec commit.

---

# Phase 1 — Layout shell

Adds the persistent four-zone layout, focus model, and Tab/number focus cycling. No panel does anything useful yet — they all show muted placeholders. Existing global keys (`space`/`n`/`p`/`+/-`/`q`) and existing modals (`/`/`o`/`l`) all keep working exactly as today. The new layout shows underneath the modals when no modal is open.

**End-of-phase outcome:** `go run ./cmd/goove` renders the new layout with placeholders. Tab/Shift-Tab and `1`/`2`/`3`/`4` move a yellow border between Playlists, Search, Output, and Main panels visibly. All existing tests pass. All existing TUI features (play/pause/skip/volume/search/output/browser) work via their old modal paths.

## Task 2: Add focus enum

Smallest possible first slice — introduce the focus type and its cycling helpers in their own file, with tests, no Model changes yet.

**Files:**
- Create: `internal/app/focus.go`
- Create: `internal/app/focus_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/focus_test.go`:

```go
package app

import "testing"

func TestNextFocusCyclesForward(t *testing.T) {
	tests := []struct {
		from, want focus
	}{
		{focusPlaylists, focusSearch},
		{focusSearch, focusOutput},
		{focusOutput, focusMain},
		{focusMain, focusPlaylists},
	}
	for _, tt := range tests {
		got := nextFocus(tt.from)
		if got != tt.want {
			t.Errorf("nextFocus(%v) = %v; want %v", tt.from, got, tt.want)
		}
	}
}

func TestPrevFocusCyclesBackward(t *testing.T) {
	tests := []struct {
		from, want focus
	}{
		{focusPlaylists, focusMain},
		{focusSearch, focusPlaylists},
		{focusOutput, focusSearch},
		{focusMain, focusOutput},
	}
	for _, tt := range tests {
		got := prevFocus(tt.from)
		if got != tt.want {
			t.Errorf("prevFocus(%v) = %v; want %v", tt.from, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestNextFocus -run TestPrevFocus -v
```

Expected: compile error, `undefined: focus / focusPlaylists / focusSearch / focusOutput / focusMain / nextFocus / prevFocus`.

- [ ] **Step 3: Write the focus type and helpers**

Create `internal/app/focus.go`:

```go
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
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestNextFocus -run TestPrevFocus -v
```

Expected: PASS for both tests.

- [ ] **Step 5: Run the full test suite to confirm nothing else broke**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/focus.go internal/app/focus_test.go
git commit -m "$(cat <<'EOF'
app: add focus enum + cycling helpers

Foundation for the multi-panel layout. Four focusable zones (Playlists,
Search, Output, Main); now-playing is read-only and skipped.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add panel state structs to Model

Add the four panel state types and the `focus` field to `Model`. No behavior change yet — every field just exists as zero values.

**Files:**
- Modify: `internal/app/model.go`

- [ ] **Step 1: Add state struct types and `focus` field**

In `internal/app/model.go`, after the existing `browserState` struct (around line 89), add the new panel state types:

```go
// playlistsPanel is the state of the Playlists panel (left, top of stack).
// items is the cached playlist list; cursor is the highlighted row;
// tracksByName caches per-playlist tracks for live-preview hits.
type playlistsPanel struct {
	items        []domain.Playlist
	cursor       int
	loading      bool
	err          error
	tracksByName map[string][]domain.Track
	fetchingFor  map[string]bool
}

// searchPanel is the state of the Search panel (left, middle of stack).
// inputMode true means typing routes into the query; outside input mode the
// panel is "idle" and shows a muted prompt.
type searchPanel struct {
	inputMode bool
	query     string
	seq       uint64
	loading   bool
	lastQuery string
	total     int
	err       error
}

// outputPanel is the state of the Output panel (left, bottom of stack).
type outputPanel struct {
	devices []domain.AudioDevice
	cursor  int
	loading bool
	err     error
}

// mainPaneMode is which "view" the main pane is showing.
type mainPaneMode int

const (
	mainPaneTracks mainPaneMode = iota
	mainPaneSearchResults
)

// mainPanel is the state of the right-hand main pane.
type mainPanel struct {
	mode             mainPaneMode
	cursor           int
	selectedPlaylist string
	searchResults    []domain.Track
}
```

Then in the `Model` struct, after the existing `search *searchState` field, add:

```go
	// New layout state (Phase 1).
	focusZ    focus
	playlists playlistsPanel
	search2   searchPanel // temp name; renamed to `search` in Phase 6 after the modal type is retired
	output    outputPanel
	main      mainPanel
```

> **Naming note.** The existing `search *searchState` field is the modal's pointer. We can't reuse `search` as a field name yet because Go forbids two fields with the same name. Use the temp name `search2` in Phases 1–3 and rename to `search` in Phase 6 once `*searchState` is gone. Same for `focusZ` → `focus`: we can't shadow the type name with a field name in Go, so we use `focusZ` while the type is named `focus`. (Alternative: rename the type to `focusKind` instead. Either is fine; this plan uses `focusZ` field name.)

- [ ] **Step 2: Initialise the maps in `New`**

In `internal/app/model.go`, modify `New` (around line 118) to initialise the playlist track cache maps:

```go
func New(client music.Client, renderer art.Renderer) Model {
	return Model{
		client:     client,
		renderer:   renderer,
		state:      Disconnected{},
		lastVolume: 50,
		playlists: playlistsPanel{
			tracksByName: make(map[string][]domain.Track),
			fetchingFor:  make(map[string]bool),
		},
	}
}
```

- [ ] **Step 3: Build to confirm types check**

```bash
go build ./...
```

Expected: builds clean.

- [ ] **Step 4: Run all tests to confirm nothing else broke**

```bash
go test ./...
```

Expected: PASS — existing tests don't use the new fields.

- [ ] **Step 5: Commit**

```bash
git add internal/app/model.go
git commit -m "$(cat <<'EOF'
app: add panel state structs to Model

playlistsPanel / searchPanel / outputPanel / mainPanel + focusZ field on
Model. No behaviour change yet — Phase 1 wires them.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Wire Tab / Shift-Tab / 1-4 focus cycling

Hook the focus-cycling keys into `update.go`. They only do anything when no modal is open — modals capture input first (existing behaviour).

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/app/update_test.go`, append the following tests at the bottom of the file:

```go
func TestTabAdvancesFocusFromPlaylistsToSearch(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ after Tab = %v; want focusSearch", got.focusZ)
	}
}

func TestShiftTabReversesFocus(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusOutput
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ after Shift-Tab from Output = %v; want focusSearch", got.focusZ)
	}
}

func TestNumberKeysJumpDirectlyToFocus(t *testing.T) {
	tests := []struct {
		key  rune
		want focus
	}{
		{'1', focusPlaylists},
		{'2', focusSearch},
		{'3', focusOutput},
		{'4', focusMain},
	}
	for _, tt := range tests {
		m := newTestModel()
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		got := updated.(Model)
		if got.focusZ != tt.want {
			t.Errorf("focusZ after '%c' = %v; want %v", tt.key, got.focusZ, tt.want)
		}
	}
}

func TestFocusKeysSuppressedWhilePickerOpen(t *testing.T) {
	m := newTestModel()
	m.picker = &pickerState{}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ after Tab while picker open = %v; want focusPlaylists (no change)", got.focusZ)
	}
}

func TestFocusKeysSuppressedWhileSearchModalOpen(t *testing.T) {
	m := newTestModel()
	m.search = &searchState{}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ after '2' while search modal open = %v; want focusPlaylists (no change)", got.focusZ)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestTab -run TestShiftTab -run TestNumber -run TestFocus -v
```

Expected: tests fail because the keys aren't wired yet (they pass `focusPlaylists` as initial; Tab/etc. don't change it).

- [ ] **Step 3: Add the focus-cycling cases to `handleKey`**

In `internal/app/update.go`, in the `handleKey` function, add the new cases at the **top** of the `switch msg.String()` block (after the modal short-circuits but before the existing `case "q":`):

```go
	switch msg.String() {
	case "tab":
		m.focusZ = nextFocus(m.focusZ)
		return m, nil

	case "shift+tab":
		m.focusZ = prevFocus(m.focusZ)
		return m, nil

	case "1":
		m.focusZ = focusPlaylists
		return m, nil

	case "2":
		m.focusZ = focusSearch
		return m, nil

	case "3":
		m.focusZ = focusOutput
		return m, nil

	case "4":
		m.focusZ = focusMain
		return m, nil

	case "q":
		// ...existing code...
```

> The modal short-circuits at the top of `handleKey` (`if m.search != nil`, `if m.picker != nil`, `if m.mode == modeBrowser`) already prevent these keys from reaching the new cases when a modal is open. So suppression-while-modal-open is automatic.

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestTab -run TestShiftTab -run TestNumber -run TestFocus -v
```

Expected: PASS.

- [ ] **Step 5: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/update.go internal/app/update_test.go
git commit -m "$(cat <<'EOF'
app: wire Tab / Shift-Tab / 1-4 focus cycling

Suppressed while any modal is open (handled automatically by the existing
modal short-circuits at the top of handleKey).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Extract now-playing into `panel_now_playing.go`

Move the existing `renderConnectedCard` / `renderConnected` / `renderIdle` / `renderDisconnected` functions out of `view.go` into a new `panel_now_playing.go`. Behaviour identical; this is a pure cut-and-paste refactor that we can verify with the existing tests. (No tests to add for this task; existing tests still cover the rendering paths.)

**Files:**
- Modify: `internal/app/view.go`
- Create: `internal/app/panel_now_playing.go`

- [ ] **Step 1: Cut the now-playing render code into the new file**

Create `internal/app/panel_now_playing.go`:

```go
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderNowPlayingPanel renders the top panel for any AppState. The shape is
// identical to the previous renderConnected / renderIdle / renderDisconnected
// trio — they're just moved here under a single entry point so view.go can
// compose this panel beside the others.
//
// Phase 1: the panel still uses the existing card/border style. Phase 5
// adds optional album art on the left.
func renderNowPlayingPanel(m Model) string {
	switch s := m.state.(type) {
	case Connected:
		return renderConnectedCardOnly(s, m.art.output, m.width)
	case Idle:
		return renderIdleCard(s.Volume)
	case Disconnected:
		return renderDisconnectedCard()
	}
	return ""
}

// renderConnectedCardOnly returns just the card (no footer / no error line).
// view.go composes the footer separately. Same content as renderConnectedCard
// but no margin wrapping (the parent does that).
func renderConnectedCardOnly(s Connected, art string, width int) string {
	pos := s.Now.DisplayedPosition(time.Now())
	var b strings.Builder

	state := "▶"
	if !s.Now.IsPlaying {
		state = "⏸"
	}

	b.WriteString(titleStyle.Render(state + "  " + s.Now.Track.Title))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(s.Now.Track.Artist))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(s.Now.Track.Album))
	b.WriteString("\n\n")
	b.WriteString(progressBar(pos, s.Now.Duration, progressBarWidth))
	b.WriteString("   ")
	b.WriteString(formatDuration(pos))
	b.WriteString(" / ")
	b.WriteString(formatDuration(s.Now.Duration))
	b.WriteString("\n\n")
	b.WriteString("volume  ")
	b.WriteString(volumeBar(s.Now.Volume, volumeBarWidth))
	b.WriteString(fmt.Sprintf("   %d%%", s.Now.Volume))

	content := b.String()
	if width >= artLayoutThreshold && art != "" {
		content = lipgloss.JoinHorizontal(lipgloss.Center, art, "  ", content)
	}
	return cardStyle.Render(content)
}

func renderIdleCard(volume int) string {
	body := titleStyle.Render("Music is open, nothing playing.") + "\n\n" +
		subtitleStyle.Render("press space or n to start playback") + "\n\n" +
		"volume  " + volumeBar(volume, volumeBarWidth) + fmt.Sprintf("   %d%%", volume)
	return cardStyle.Render(body)
}

func renderDisconnectedCard() string {
	body := titleStyle.Render("Apple Music isn't running.") + "\n\n" +
		subtitleStyle.Render("press space to launch it, q to quit")
	return cardStyle.Render(body)
}
```

- [ ] **Step 2: Build and run existing tests**

```bash
go build ./...
go test ./...
```

Expected: PASS — these new functions aren't called yet, but they compile.

- [ ] **Step 3: Commit**

```bash
git add internal/app/panel_now_playing.go
git commit -m "$(cat <<'EOF'
app: extract now-playing renderers into panel_now_playing.go

Pure refactor: cuts renderConnectedCardOnly / renderIdleCard /
renderDisconnectedCard into a panel-shaped entry point. view.go still
uses the old functions for now; Phase 1 task 7 swaps it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Stub the four placeholder panels + hint bar

Each of the four left-column panels (Playlists, Search, Output, Main) gets a placeholder render function. Hint bar gets a basic implementation that varies by focus.

**Files:**
- Create: `internal/app/panel_playlists.go`
- Create: `internal/app/panel_search.go`
- Create: `internal/app/panel_output.go`
- Create: `internal/app/panel_main.go`
- Create: `internal/app/hints.go`
- Create: `internal/app/hints_test.go`

- [ ] **Step 1: Write the failing hint-bar tests**

Create `internal/app/hints_test.go`:

```go
package app

import (
	"strings"
	"testing"
)

func TestHintBarAlwaysContainsGlobals(t *testing.T) {
	for _, f := range []focus{focusPlaylists, focusSearch, focusOutput, focusMain} {
		got := renderHintBar(Model{focusZ: f})
		for _, want := range []string{"space", "n", "p", "q"} {
			if !strings.Contains(got, want) {
				t.Errorf("focus=%v: hint bar %q missing global %q", f, got, want)
			}
		}
	}
}

func TestHintBarContainsPanelKeysForPlaylists(t *testing.T) {
	got := renderHintBar(Model{focusZ: focusPlaylists})
	if !strings.Contains(got, "j/k") {
		t.Errorf("hint bar for Playlists missing j/k: %q", got)
	}
	if !strings.Contains(got, "play") {
		t.Errorf("hint bar for Playlists missing play hint: %q", got)
	}
}

func TestHintBarContainsPanelKeysForSearchInIdle(t *testing.T) {
	got := renderHintBar(Model{focusZ: focusSearch})
	if !strings.Contains(got, "type to search") {
		t.Errorf("hint bar for Search idle missing 'type to search': %q", got)
	}
}

func TestHintBarContainsPanelKeysForOutput(t *testing.T) {
	got := renderHintBar(Model{focusZ: focusOutput})
	if !strings.Contains(got, "switch") {
		t.Errorf("hint bar for Output missing 'switch': %q", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestHintBar -v
```

Expected: compile error, `undefined: renderHintBar`.

- [ ] **Step 3: Implement the hint bar**

Create `internal/app/hints.go`:

```go
package app

import "strings"

// globalKeysHint is shown on every hint bar.
const globalKeysHint = "space:play/pause  n:next  p:prev  +/-:vol  q:quit"

// renderHintBar returns the bottom-of-screen hint string. Always includes
// the global keys; appends panel-scoped hints based on m.focusZ. Style is
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
	switch m.focusZ {
	case focusPlaylists:
		return "j/k:nav  ⏎:play"
	case focusSearch:
		if m.search2.inputMode {
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
```

- [ ] **Step 4: Run hint-bar tests**

```bash
go test ./internal/app/ -run TestHintBar -v
```

Expected: PASS.

- [ ] **Step 5: Stub Playlists / Search / Output / Main panels**

Create `internal/app/panel_playlists.go`:

```go
package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPlaylistsPanel renders the Playlists panel (left, top of stack).
// Phase 1: placeholder. Phase 2 wires real content.
func renderPlaylistsPanel(m Model, width, height int) string {
	title := "Playlists"
	body := subtitleStyle.Render("—")
	return panelBox(title, body, width, height, m.focusZ == focusPlaylists)
}

// panelBox is the shared lipgloss box used by every left-column panel.
// focused=true draws the border in the focus colour.
func panelBox(title, body string, width, height int, focused bool) string {
	border := lipgloss.NormalBorder()
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("#6b7280")).
		Width(width - 2).
		Height(height - 2).
		Padding(0, 1)
	if focused {
		style = style.BorderForeground(lipgloss.Color("#ebcb8b"))
	}
	header := titleStyle.Render(title)
	return style.Render(header + "\n" + strings.TrimRight(body, "\n"))
}
```

Create `internal/app/panel_search.go`:

```go
package app

func renderSearchPanel(m Model, width, height int) string {
	title := "Search"
	body := subtitleStyle.Render("/  type to search")
	return panelBox(title, body, width, height, m.focusZ == focusSearch)
}
```

Create `internal/app/panel_output.go`:

```go
package app

func renderOutputPanel(m Model, width, height int) string {
	title := "Output"
	body := subtitleStyle.Render("—")
	return panelBox(title, body, width, height, m.focusZ == focusOutput)
}
```

Create `internal/app/panel_main.go`:

```go
package app

import (
	"github.com/charmbracelet/lipgloss"
)

// renderMainPanel renders the right-hand main pane. Phase 1: placeholder.
// Phase 2 fills in mainPaneTracks; Phase 3 fills in mainPaneSearchResults.
func renderMainPanel(m Model, width, height int) string {
	title := "—"
	body := subtitleStyle.Render("focus a panel on the left to see its content")
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

// panelBoxWide is the same as panelBox but for the wider main pane. Identical
// implementation kept separate so future tweaks (e.g. main pane padding) can
// diverge without touching left-column panels.
func panelBoxWide(title, body string, width, height int, focused bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6b7280")).
		Width(width - 2).
		Height(height - 2).
		Padding(0, 1)
	if focused {
		style = style.BorderForeground(lipgloss.Color("#ebcb8b"))
	}
	header := titleStyle.Render(title)
	return style.Render(header + "\n" + body)
}
```

- [ ] **Step 6: Build to confirm types**

```bash
go build ./...
```

Expected: builds clean.

- [ ] **Step 7: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app/panel_playlists.go internal/app/panel_search.go internal/app/panel_output.go internal/app/panel_main.go internal/app/hints.go internal/app/hints_test.go
git commit -m "$(cat <<'EOF'
app: stub four panels + hint bar

Placeholders for Playlists / Search / Output / Main panels (focused border
toggles based on m.focusZ). Hint bar composes globals + focused panel's
keys. view.go wired in next task.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Compose the new layout in `view.go`

Rewire `view.go` so that — when no modal is open — the View output is the new four-zone layout. Modals (search, picker, browser) still take precedence and render with their existing functions.

**Files:**
- Modify: `internal/app/view.go`

- [ ] **Step 1: Replace the main `View` body**

In `internal/app/view.go`, replace the `View` method (around line 31) with:

```go
func (m Model) View() string {
	if m.permissionDenied {
		return renderPermissionDenied()
	}
	// Modals (Phase 1): still render on top of everything when open.
	if m.search != nil {
		return renderSearch(m.search)
	}
	if m.picker != nil {
		return renderPicker(m.picker)
	}
	if m.mode == modeBrowser {
		return renderBrowser(m)
	}
	if m.width > 0 && m.width < compactThreshold {
		return renderCompact(m)
	}
	return renderLayout(m)
}
```

- [ ] **Step 2: Add `renderLayout` to compose the four panels + hint bar**

Append to the end of `internal/app/view.go`:

```go
// renderLayout composes the four panels + hint bar + (optional) error footer.
// Used when no modal is open and the terminal is wide enough.
func renderLayout(m Model) string {
	width := m.width
	if width <= 0 {
		width = 100 // safe default before the first WindowSizeMsg
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	// Top panel: now-playing, full width.
	now := renderNowPlayingPanel(m)

	// Geometry: left column ~25% of width, main pane gets the rest.
	leftWidth := width / 4
	if leftWidth < 18 {
		leftWidth = 18
	}
	mainWidth := width - leftWidth - 2

	// Three left-column panels share the remaining vertical space below the
	// now-playing panel. We give equal heights for v1.
	bottomHeight := height - lipgloss.Height(now) - 2
	panelHeight := bottomHeight / 3

	pl := renderPlaylistsPanel(m, leftWidth, panelHeight)
	se := renderSearchPanel(m, leftWidth, panelHeight)
	op := renderOutputPanel(m, leftWidth, panelHeight)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, pl, se, op)

	mn := renderMainPanel(m, mainWidth, bottomHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, mn)
	hint := footerStyle.Render(renderHintBar(m))

	out := lipgloss.JoinVertical(lipgloss.Left, now, body, hint)
	if errFooter := m.errFooter(); errFooter != "" {
		out += "\n" + errFooter
	}
	return out
}
```

- [ ] **Step 3: Add the `lipgloss` import if not already present**

The `view.go` file should already import `github.com/charmbracelet/lipgloss`. Verify:

```bash
grep "lipgloss" internal/app/view.go
```

Expected: at least one match in the `import` block. If not, add it.

- [ ] **Step 4: Build and run all tests**

```bash
go build ./...
go test ./...
```

Expected: PASS — existing tests don't snapshot the full View output, so adding the new layout doesn't break them.

- [ ] **Step 5: Smoke-test interactively**

Run goove against real Music.app (skip this step in CI / on Linux):

```bash
go run ./cmd/goove
```

Expected: see the new layout — now-playing on top, three placeholder panels stacked left, "—" main pane on the right. Press `Tab` and watch the focused border move: Playlists → Search → Output → Main → Playlists. Press `1`/`2`/`3`/`4` and watch the focus jump directly. `space`/`n`/`p`/`+/-` still work. `/`, `o`, `l` still open their old modals.

Press `q` to quit.

- [ ] **Step 6: Commit**

```bash
git add internal/app/view.go
git commit -m "$(cat <<'EOF'
app: compose new four-zone layout in view.go

When no modal is open, View now renders the persistent layout: now-playing
on top, three placeholder panels stacked left, main pane right, hint bar
bottom. Modals (search/picker/browser) still take precedence.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Phase 1 wrap-up — verify and tag

End-of-phase checkpoint. Confirm all old features still work.

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run integration tests too (macOS only, real Music.app)**

```bash
go test -tags=integration ./internal/music/applescript/
```

Expected: PASS (these tests don't exercise the TUI; should be unaffected).

- [ ] **Step 3: Manual smoke walk**

Run `go run ./cmd/goove`. Verify each of these still works exactly as before:
- `space` plays / pauses
- `n` next, `p` prev
- `+` / `-` adjust volume
- `q` quits
- `/` opens the search modal (overlays the new layout); typing, ⏎, esc all still work
- `o` opens the output picker modal; ↑↓⏎esc all still work
- `l` opens the playlist browser modal; tab / ↑↓ / ⏎ / esc all still work
- `Tab`, `Shift-Tab`, `1`–`4` cycle focus among the panels (visible via border)

If anything is broken, fix it before moving on. Commit fixes individually.

- [ ] **Step 4: Tag the phase end**

```bash
git tag tui-overhaul-phase-1
```

Optional but recommended — gives a fast bisect target if a later phase regresses.

---

# Phase 2 — Playlists panel + main pane (live preview)

Wires the Playlists panel and the main pane to real data. Cursor moves in Playlists live-preview the tracks in main; ⏎ in Playlists plays the playlist; ⏎ in main plays from a track. The `l` browser modal is retired at the end of the phase.

**End-of-phase outcome:** No more `l` keybind, no more browser modal. Playlists and main pane are fully functional; user navigates and plays without leaving the persistent layout.

## Task 9: Wire playlists fetch on first focus

When the user moves focus onto the Playlists panel for the first time and there's no cached list yet, fetch them.

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/panel_playlists.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/update_test.go`:

```go
func TestFocusingPlaylistsFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(nil)
	m := New(c, nil)
	// focusZ starts at focusPlaylists by default; we force a transition to
	// trigger the on-focus fetch.
	m.focusZ = focusSearch
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Fatalf("focusZ = %v; want focusPlaylists", got.focusZ)
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylists Cmd on focus")
	}
	out := cmd()
	if _, ok := out.(playlistsMsg); !ok {
		t.Fatalf("cmd produced %T; want playlistsMsg", out)
	}
}

func TestFocusingPlaylistsDoesNotRefetchWhenCached(t *testing.T) {
	m := newTestModel()
	m.playlists.items = []domain.Playlist{{Name: "Liked Songs"}}
	m.focusZ = focusSearch
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when playlists already cached, got %T", cmd())
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestFocusingPlaylists -v
```

Expected: FAIL — the focus-jump cases in `handleKey` don't fire fetch yet.

- [ ] **Step 3: Add the on-focus fetch helper**

In `internal/app/panel_playlists.go`, append:

```go
// onFocusPlaylists is called by handleKey whenever focus transitions TO the
// Playlists panel. Returns a fetchPlaylists Cmd if the list isn't cached yet,
// or nil. Idempotent on repeat focuses (cache hit ⇒ no Cmd).
func onFocusPlaylists(m Model) (Model, tea.Cmd) {
	if len(m.playlists.items) > 0 || m.playlists.loading {
		return m, nil
	}
	m.playlists.loading = true
	return m, fetchPlaylists(m.client)
}
```

You'll also need to add the `tea` import:

```go
import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)
```

- [ ] **Step 4: Hook it into the focus-cycling cases in `update.go`**

In `internal/app/update.go`, modify the focus-changing cases in `handleKey` to invoke the on-focus hook. Replace the existing `case "tab":`, `case "shift+tab":`, and `case "1":` cases with:

```go
	case "tab":
		m.focusZ = nextFocus(m.focusZ)
		return m.onFocusEntered()

	case "shift+tab":
		m.focusZ = prevFocus(m.focusZ)
		return m.onFocusEntered()

	case "1":
		m.focusZ = focusPlaylists
		return m.onFocusEntered()

	case "2":
		m.focusZ = focusSearch
		return m.onFocusEntered()

	case "3":
		m.focusZ = focusOutput
		return m.onFocusEntered()

	case "4":
		m.focusZ = focusMain
		return m.onFocusEntered()
```

Add the helper at the bottom of `update.go`:

```go
// onFocusEntered is called whenever m.focusZ has just been changed. Dispatches
// to the per-panel on-focus hook, which may return a fetch Cmd.
func (m Model) onFocusEntered() (tea.Model, tea.Cmd) {
	switch m.focusZ {
	case focusPlaylists:
		mm, cmd := onFocusPlaylists(m)
		return mm, cmd
	}
	return m, nil
}
```

- [ ] **Step 5: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestFocusingPlaylists -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/update.go internal/app/panel_playlists.go internal/app/update_test.go
git commit -m "$(cat <<'EOF'
app: fetch playlists on first Playlists-panel focus

Tab / Shift-Tab / number-key focus changes now dispatch through
onFocusEntered, which fires fetchPlaylists when the panel is unpopulated.
Cached focuses are no-ops.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Render real playlists in the Playlists panel

Populate the panel render with the items list and a cursor.

**Files:**
- Modify: `internal/app/panel_playlists.go`
- Modify: `internal/app/update.go`

- [ ] **Step 1: Update the playlists-msg handler**

In `internal/app/update.go`, find the existing `case playlistsMsg:` block (around line 85). It currently writes into `m.browser`. Add a parallel write into `m.playlists`:

```go
	case playlistsMsg:
		// Phase 2: also populate the persistent panel state.
		m.playlists.loading = false
		m.playlists.err = msg.err
		if msg.err == nil {
			m.playlists.items = msg.playlists
			if m.playlists.cursor >= len(msg.playlists) {
				m.playlists.cursor = 0
			}
		}
		// Existing browser-modal write (Phase 2 still keeps the modal alive):
		if m.browser != nil {
			m.browser.loadingLists = false
			m.browser.err = msg.err
			if msg.err == nil {
				m.browser.playlists = msg.playlists
				if m.browser.playlistCursor >= len(msg.playlists) {
					m.browser.playlistCursor = 0
				}
			}
		}
		return m, nil
```

- [ ] **Step 2: Replace the placeholder render in `panel_playlists.go`**

Replace the body of `renderPlaylistsPanel` in `internal/app/panel_playlists.go`:

```go
func renderPlaylistsPanel(m Model, width, height int) string {
	title := "Playlists"
	body := renderPlaylistsBody(m, width, height)
	return panelBox(title, body, width, height, m.focusZ == focusPlaylists)
}

func renderPlaylistsBody(m Model, width, height int) string {
	if m.playlists.loading && len(m.playlists.items) == 0 {
		return subtitleStyle.Render("loading…")
	}
	if m.playlists.err != nil {
		return errorStyle.Render("error: " + m.playlists.err.Error())
	}
	if len(m.playlists.items) == 0 {
		return subtitleStyle.Render("(no playlists)")
	}
	visibleRows := height - 4 // border + title + padding
	if visibleRows < 1 {
		visibleRows = 1
	}
	start := scrollWindow(m.playlists.cursor, visibleRows, len(m.playlists.items))

	var sb strings.Builder
	for i := start; i < len(m.playlists.items) && i-start < visibleRows; i++ {
		marker := "  "
		if i == m.playlists.cursor && m.focusZ == focusPlaylists {
			marker = "▶ "
		}
		row := marker + m.playlists.items[i].Name
		sb.WriteString(truncate(row, width-4))
		if i-start < visibleRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
```

> `scrollWindow` and `truncate` already exist in `browser.go` and stay package-visible. They'll move to a new shared file in Phase 6 cleanup.

- [ ] **Step 3: Build and run all tests**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: focus the Playlists panel (`1`), then watch — after a moment, the panel populates with your real playlist names. The first row has a `▶`. Quit with `q`.

- [ ] **Step 5: Commit**

```bash
git add internal/app/panel_playlists.go internal/app/update.go
git commit -m "$(cat <<'EOF'
app: render real playlists in Playlists panel

playlistsMsg now populates both the new panel state and the (still-alive)
browser modal state. Panel renders with cursor + scroll window + truncate.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Cursor + live preview in Playlists panel

Add j/k/↑/↓ handling that moves the cursor and updates `m.main.selectedPlaylist` for live preview. (Track fetch wired in next task.)

**Files:**
- Modify: `internal/app/panel_playlists.go`
- Modify: `internal/app/update.go`
- Create: `internal/app/panel_playlists_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/panel_playlists_test.go`:

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestPlaylistsCursorDownMoves(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor = %d; want 1", got.playlists.cursor)
	}
}

func TestPlaylistsCursorUpClampsAtZero(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := updated.(Model)
	if got.playlists.cursor != 0 {
		t.Errorf("cursor = %d; want 0", got.playlists.cursor)
	}
}

func TestPlaylistsCursorDownClampsAtEnd(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor = %d; want 1 (clamped)", got.playlists.cursor)
	}
}

func TestPlaylistsCursorMoveUpdatesMainSelectedPlaylist(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.main.selectedPlaylist = "A"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.selectedPlaylist != "B" {
		t.Errorf("main.selectedPlaylist = %q; want B", got.main.selectedPlaylist)
	}
	if got.main.cursor != 0 {
		t.Errorf("main.cursor = %d; want 0 (reset on selection change)", got.main.cursor)
	}
}

func TestPlaylistsArrowsAlsoNavigate(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(Model)
	if got.playlists.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d; want 1", got.playlists.cursor)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestPlaylists -v
```

Expected: tests fail because no key handler for the panel exists yet.

- [ ] **Step 3: Add the panel key handler**

In `internal/app/panel_playlists.go`, append:

```go
// handlePlaylistsKey routes keys when focusZ == focusPlaylists. Returns
// (model, cmd, handled). When handled is false, the caller falls through to
// globals.
func handlePlaylistsKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if m.playlists.cursor > 0 {
			m.playlists.cursor--
			m = onPlaylistsCursorChanged(m)
		}
		return m, nil, true
	case "down", "j":
		if m.playlists.cursor < len(m.playlists.items)-1 {
			m.playlists.cursor++
			m = onPlaylistsCursorChanged(m)
		}
		return m, nil, true
	}
	return m, nil, false
}

// onPlaylistsCursorChanged updates the main pane's selected playlist (live
// preview, Q3-C) and resets the main pane cursor. Track-fetch wiring is
// added in the next task.
func onPlaylistsCursorChanged(m Model) Model {
	if len(m.playlists.items) == 0 {
		return m
	}
	name := m.playlists.items[m.playlists.cursor].Name
	m.main.selectedPlaylist = name
	m.main.cursor = 0
	m.main.mode = mainPaneTracks
	return m
}
```

- [ ] **Step 4: Hook the key handler into `update.go`**

In `internal/app/update.go`, modify the modal-precedence block at the top of `handleKey` so that when focus is on Playlists and no modal is open, the panel handler runs first. After the existing `if m.mode == modeBrowser` block, before the `switch msg.String() {` line, add:

```go
	// Phase 2: focus-routed panel handlers run before globals.
	if m.search == nil && m.picker == nil && m.mode != modeBrowser {
		switch m.focusZ {
		case focusPlaylists:
			if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
				return mm, cmd
			}
		}
	}
```

- [ ] **Step 5: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestPlaylists -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/panel_playlists.go internal/app/update.go internal/app/panel_playlists_test.go
git commit -m "$(cat <<'EOF'
app: Playlists panel cursor + live-preview wiring

j/k/up/down navigate the panel. Each move updates m.main.selectedPlaylist
and resets m.main.cursor (Q3-C live preview). Track fetch added next.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Track cache + fetch on selection change

When the cursor moves to a new playlist, fetch its tracks if not cached. Cached selections are no-ops.

**Files:**
- Modify: `internal/app/panel_playlists.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/panel_playlists_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/panel_playlists_test.go`:

```go
func TestPlaylistsCursorChangeFiresTrackFetchOnFirstSelection(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected fetchPlaylistTracks Cmd on first selection of B")
	}
	if !got.playlists.fetchingFor["B"] {
		t.Errorf("expected fetchingFor[B] = true")
	}
	out := cmd()
	if _, ok := out.(playlistTracksMsg); !ok {
		t.Fatalf("cmd produced %T; want playlistTracksMsg", out)
	}
}

func TestPlaylistsCursorChangeUsesCacheOnRevisit(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.tracksByName["B"] = []domain.Track{{Title: "t1"}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		out := cmd()
		t.Errorf("expected no Cmd on cached selection, got %T", out)
	}
}

func TestPlaylistsCursorChangeNoDuplicateFetch(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.fetchingFor["B"] = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Errorf("expected no Cmd while a fetch for B is in flight")
	}
}

func TestPlaylistTracksMsgPopulatesCache(t *testing.T) {
	m := newTestModel()
	m.playlists.fetchingFor["B"] = true
	tracks := []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(playlistTracksMsg{name: "B", tracks: tracks})
	got := updated.(Model)
	if got.playlists.fetchingFor["B"] {
		t.Error("expected fetchingFor[B] cleared after result lands")
	}
	if len(got.playlists.tracksByName["B"]) != 2 {
		t.Errorf("tracksByName[B] = %v; want 2 entries", got.playlists.tracksByName["B"])
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestPlaylistsCursor -run TestPlaylistTracks -v
```

Expected: tests fail — fetch isn't wired into the cursor-change hook yet.

- [ ] **Step 3: Wire fetch into `onPlaylistsCursorChanged`**

In `internal/app/panel_playlists.go`, replace `onPlaylistsCursorChanged` and add a helper:

```go
// onPlaylistsCursorChanged updates the main pane's selected playlist (live
// preview, Q3-C), resets the main pane cursor, and returns a fetchTracks Cmd
// if the new selection isn't cached and isn't already being fetched.
func onPlaylistsCursorChanged(m Model) (Model, tea.Cmd) {
	if len(m.playlists.items) == 0 {
		return m, nil
	}
	name := m.playlists.items[m.playlists.cursor].Name
	m.main.selectedPlaylist = name
	m.main.cursor = 0
	m.main.mode = mainPaneTracks

	if _, cached := m.playlists.tracksByName[name]; cached {
		return m, nil
	}
	if m.playlists.fetchingFor[name] {
		return m, nil
	}
	m.playlists.fetchingFor[name] = true
	return m, fetchPlaylistTracks(m.client, name)
}
```

Update `handlePlaylistsKey` to thread the Cmd:

```go
func handlePlaylistsKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if m.playlists.cursor > 0 {
			m.playlists.cursor--
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	case "down", "j":
		if m.playlists.cursor < len(m.playlists.items)-1 {
			m.playlists.cursor++
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	}
	return m, nil, false
}
```

- [ ] **Step 4: Update the `playlistTracksMsg` handler**

In `internal/app/update.go`, find the existing `case playlistTracksMsg:` block (around line 98). Add a parallel write into the new panel state:

```go
	case playlistTracksMsg:
		// Phase 2: populate the persistent track cache.
		delete(m.playlists.fetchingFor, msg.name)
		if msg.err != nil {
			m.playlists.err = msg.err
		} else {
			m.playlists.tracksByName[msg.name] = msg.tracks
		}
		// Existing browser-modal write:
		if m.browser != nil && len(m.browser.playlists) > 0 {
			current := m.browser.playlists[m.browser.playlistCursor].Name
			if msg.name != current {
				return m, nil
			}
			m.browser.loadingTracks = false
			m.browser.err = msg.err
			if msg.err == nil {
				m.browser.tracks = msg.tracks
				m.browser.tracksFor = msg.name
				m.browser.trackCursor = 0
			}
		}
		return m, nil
```

- [ ] **Step 5: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestPlaylistsCursor -run TestPlaylistTracks -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/panel_playlists.go internal/app/update.go internal/app/panel_playlists_test.go
git commit -m "$(cat <<'EOF'
app: per-playlist track cache with on-demand fetch

Cursor change in Playlists fetches the new selection's tracks if not
cached. Duplicate fetches suppressed via fetchingFor map. Cache populated
on playlistTracksMsg.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Render tracks in main pane

The main pane reads `m.main.selectedPlaylist`, looks up the cached tracks, and renders them with a cursor.

**Files:**
- Modify: `internal/app/panel_main.go`
- Create: `internal/app/panel_main_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/panel_main_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestMainPaneShowsLoadingWhenSelectionUncached(t *testing.T) {
	m := newTestModel()
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "B"
	m.playlists.fetchingFor["B"] = true
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "loading") {
		t.Errorf("main pane did not show loading state: %q", got)
	}
}

func TestMainPaneShowsTracksWhenCached(t *testing.T) {
	m := newTestModel()
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "B"
	m.playlists.tracksByName["B"] = []domain.Track{
		{Title: "Track One", Artist: "Artist A"},
		{Title: "Track Two", Artist: "Artist B"},
	}
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "Track One") {
		t.Errorf("main pane missing 'Track One': %q", got)
	}
	if !strings.Contains(got, "Track Two") {
		t.Errorf("main pane missing 'Track Two': %q", got)
	}
}

func TestMainPaneShowsHintWhenNothingSelected(t *testing.T) {
	m := newTestModel()
	got := renderMainPanel(m, 60, 30)
	if !strings.Contains(got, "focus") && !strings.Contains(got, "—") {
		t.Errorf("main pane hint missing: %q", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestMainPane -v
```

Expected: tests fail — placeholder render doesn't have these branches.

- [ ] **Step 3: Implement the real `renderMainPanel`**

Replace the body of `renderMainPanel` in `internal/app/panel_main.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderMainPanel(m Model, width, height int) string {
	switch m.main.mode {
	case mainPaneSearchResults:
		return renderMainSearchResults(m, width, height)
	default:
		return renderMainTracks(m, width, height)
	}
}

func renderMainTracks(m Model, width, height int) string {
	if m.main.selectedPlaylist == "" {
		title := "—"
		body := subtitleStyle.Render("focus a panel on the left to see its content")
		return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
	}
	title := m.main.selectedPlaylist
	if isPlayingFromSelected(m) {
		title += "  (now playing)"
	}

	tracks, cached := m.playlists.tracksByName[m.main.selectedPlaylist]
	var body string
	switch {
	case !cached && m.playlists.fetchingFor[m.main.selectedPlaylist]:
		body = subtitleStyle.Render("loading…")
	case !cached:
		body = subtitleStyle.Render("(no tracks loaded)")
	case len(tracks) == 0:
		body = subtitleStyle.Render("(empty playlist)")
	default:
		body = renderTrackRows(m, tracks, width, height)
	}
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

func renderMainSearchResults(m Model, width, height int) string {
	title := fmt.Sprintf("Search: %q · %d results", "", len(m.main.searchResults))
	if m.search2.lastQuery != "" {
		title = fmt.Sprintf("Search: %q · %d results", m.search2.lastQuery, m.search2.total)
	}
	if len(m.main.searchResults) == 0 {
		body := subtitleStyle.Render("no matches")
		return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
	}
	body := renderTrackRows(m, m.main.searchResults, width, height)
	return panelBoxWide(title, body, width, height, m.focusZ == focusMain)
}

// renderTrackRows is shared between mainPaneTracks and mainPaneSearchResults.
func renderTrackRows(m Model, tracks []domain.Track, width, height int) string {
	visibleRows := height - 4
	if visibleRows < 1 {
		visibleRows = 1
	}
	start := scrollWindow(m.main.cursor, visibleRows, len(tracks))

	var sb strings.Builder
	for i := start; i < len(tracks) && i-start < visibleRows; i++ {
		marker := "  "
		if i == m.main.cursor && m.focusZ == focusMain {
			marker = "▶ "
		}
		t := tracks[i]
		row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
		sb.WriteString(truncate(row, width-4))
		if i-start < visibleRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// isPlayingFromSelected returns true when the currently-playing track is
// known to be from the playlist that's selected in the Playlists panel. For
// v1 this is best-effort: we don't track the source playlist of a track, so
// the heuristic is "the selected playlist contains a track whose persistent
// ID matches the now-playing track's." If the now-playing track has no
// persistent ID (older code paths), this returns false.
func isPlayingFromSelected(m Model) bool {
	conn, ok := m.state.(Connected)
	if !ok || conn.Now.Track.PersistentID == "" {
		return false
	}
	tracks, cached := m.playlists.tracksByName[m.main.selectedPlaylist]
	if !cached {
		return false
	}
	for _, t := range tracks {
		if t.PersistentID == conn.Now.Track.PersistentID {
			return true
		}
	}
	return false
}
```

You'll need the `domain` import:

```go
import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/themoderngeek/goove/internal/domain"
)
```

The previous `panelBoxWide` helper stays.

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestMainPane -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: focus the Playlists panel (`1`), press `j`/`k` — the main pane updates with that playlist's tracks (after a brief loading delay on first preview, instant on revisit). Quit with `q`.

- [ ] **Step 7: Commit**

```bash
git add internal/app/panel_main.go internal/app/panel_main_test.go
git commit -m "$(cat <<'EOF'
app: render tracks in main pane (live preview)

main pane now reads m.main.selectedPlaylist + cached tracks, renders rows
with cursor + 'now playing' title hint when the current track is from the
selected playlist.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: ⏎ in Playlists plays the playlist

Pressing enter on a Playlists row plays the highlighted playlist from track 1.

**Files:**
- Modify: `internal/app/panel_playlists.go`
- Modify: `internal/app/panel_playlists_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/panel_playlists_test.go`:

```go
func TestPlaylistsEnterPlaysHighlightedPlaylistFromTrackZero(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.focusZ = focusPlaylists
	m.playlists.items = []domain.Playlist{{Name: "A"}, {Name: "B"}}
	m.playlists.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	out := cmd()
	if _, ok := out.(playPlaylistMsg); !ok {
		t.Fatalf("cmd produced %T; want playPlaylistMsg", out)
	}
	if c.PlayPlaylistCalls != 1 {
		t.Errorf("PlayPlaylist calls = %d; want 1", c.PlayPlaylistCalls)
	}
	if c.LastPlayPlaylistName != "B" {
		t.Errorf("LastPlayPlaylistName = %q; want B", c.LastPlayPlaylistName)
	}
	if c.LastPlayPlaylistFromIdx != 0 {
		t.Errorf("LastPlayPlaylistFromIdx = %d; want 0", c.LastPlayPlaylistFromIdx)
	}
}

func TestPlaylistsEnterIsNoOpWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusPlaylists
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd with empty list, got %T", cmd())
	}
}
```

You'll need to add `"github.com/themoderngeek/goove/internal/music/fake"` to the import block of `panel_playlists_test.go`.

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestPlaylistsEnter -v
```

Expected: FAIL — handler doesn't dispatch enter yet.

- [ ] **Step 3: Add the enter case**

In `internal/app/panel_playlists.go`, modify `handlePlaylistsKey` to add the enter case. Replace the function with:

```go
func handlePlaylistsKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if m.playlists.cursor > 0 {
			m.playlists.cursor--
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	case "down", "j":
		if m.playlists.cursor < len(m.playlists.items)-1 {
			m.playlists.cursor++
			mm, cmd := onPlaylistsCursorChanged(m)
			return mm, cmd, true
		}
		return m, nil, true
	case "enter":
		if len(m.playlists.items) == 0 {
			return m, nil, true
		}
		name := m.playlists.items[m.playlists.cursor].Name
		return m, playPlaylist(m.client, name, 0), true
	}
	return m, nil, false
}
```

`playPlaylist` already exists in `browser.go` (visible in this package).

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/app/ -run TestPlaylistsEnter -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/panel_playlists.go internal/app/panel_playlists_test.go
git commit -m "$(cat <<'EOF'
app: ⏎ on Playlists panel plays from track zero

Reuses the existing playPlaylist Cmd. No-op on empty list.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: Cursor + ⏎ in main pane (tracks mode)

Add j/k navigation and ⏎ play-from-track to the main pane when in `mainPaneTracks` mode.

**Files:**
- Modify: `internal/app/panel_main.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/panel_main_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/panel_main_test.go`:

```go
import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestMainTracksCursorDownMoves(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.playlists.tracksByName["A"] = []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.cursor != 1 {
		t.Errorf("main.cursor = %d; want 1", got.main.cursor)
	}
}

func TestMainTracksCursorClampsAtEnd(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.main.cursor = 1
	m.playlists.tracksByName["A"] = []domain.Track{{Title: "t1"}, {Title: "t2"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.main.cursor != 1 {
		t.Errorf("main.cursor = %d; want 1 (clamped)", got.main.cursor)
	}
}

func TestMainTracksEnterPlaysFromCursor(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.focusZ = focusMain
	m.main.mode = mainPaneTracks
	m.main.selectedPlaylist = "A"
	m.main.cursor = 2
	m.playlists.tracksByName["A"] = []domain.Track{
		{Title: "t1"}, {Title: "t2"}, {Title: "t3"}, {Title: "t4"},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected playPlaylist Cmd")
	}
	cmd()
	if c.LastPlayPlaylistFromIdx != 2 {
		t.Errorf("LastPlayPlaylistFromIdx = %d; want 2", c.LastPlayPlaylistFromIdx)
	}
}

func TestMainTracksEnterIsNoOpWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusMain
	m.main.mode = mainPaneTracks
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd with empty selection, got %T", cmd())
	}
}
```

- [ ] **Step 2: Add the panel handler**

In `internal/app/panel_main.go`, append:

```go
import tea "github.com/charmbracelet/bubbletea"

// handleMainKey routes keys when focusZ == focusMain.
func handleMainKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	tracks := mainPaneRows(m)
	switch msg.String() {
	case "up", "k":
		if m.main.cursor > 0 {
			m.main.cursor--
		}
		return m, nil, true
	case "down", "j":
		if m.main.cursor < len(tracks)-1 {
			m.main.cursor++
		}
		return m, nil, true
	case "enter":
		if len(tracks) == 0 {
			return m, nil, true
		}
		switch m.main.mode {
		case mainPaneTracks:
			if m.main.selectedPlaylist == "" {
				return m, nil, true
			}
			return m, playPlaylist(m.client, m.main.selectedPlaylist, m.main.cursor), true
		case mainPaneSearchResults:
			pid := tracks[m.main.cursor].PersistentID
			return m, playTrack(m.client, pid), true
		}
	}
	return m, nil, false
}

// mainPaneRows returns whichever slice is currently visible in the main pane.
func mainPaneRows(m Model) []domain.Track {
	switch m.main.mode {
	case mainPaneSearchResults:
		return m.main.searchResults
	default:
		if m.main.selectedPlaylist == "" {
			return nil
		}
		return m.playlists.tracksByName[m.main.selectedPlaylist]
	}
}

// playTrack is the Cmd used when ⏎ is pressed on a search result. Reuses
// client.PlayTrack — same call the search modal already made.
func playTrack(c music.Client, persistentID string) tea.Cmd {
	return func() tea.Msg {
		return searchPlayedMsg{err: c.PlayTrack(context.Background(), persistentID)}
	}
}
```

You'll need imports `"context"` and `"github.com/themoderngeek/goove/internal/music"`. Replace the file's import block with:

```go
import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)
```

> The `playTrack` Cmd reuses the existing `searchPlayedMsg` purely as a transport for the error. In Phase 6 we'll rename it to a generic `playTrackMsg`. For Phase 2 we don't care about the error path since `searchPlayedMsg` is currently routed through the search modal handler (which is gated on `m.search != nil`); the message will be silently dropped. That's an acceptable transient.

Actually — to avoid the silent drop being a hidden bug, let's also add an unconditional read in `update.go`. Add this case to the top-level Update msg switch in `update.go`, before the `case statusMsg:` block:

```go
	case searchPlayedMsg:
		// Phase 2: handle a play-track result that may have come from the new
		// main-pane enter. The existing search-modal handler is below; we keep
		// it for compatibility while the modal still exists. This new case only
		// fires when the modal isn't open.
		if m.search == nil {
			if msg.err != nil {
				m.lastError = msg.err
				m.lastErrorAt = time.Now()
				return m, clearErrorAfter()
			}
			return m, nil
		}
		// Falls through to the existing case below.
		fallthrough
```

Wait — `fallthrough` doesn't work across non-adjacent cases. Instead, structure it as:

```go
	case searchPlayedMsg:
		if m.search != nil {
			if msg.seq != m.search.seq {
				return m, nil
			}
			if msg.err != nil {
				m.search.err = msg.err
				return m, nil
			}
			m.search = nil
			return m, nil
		}
		// Phase 2: result from the new main-pane enter.
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, nil
```

Find and replace the existing `case searchPlayedMsg:` block in `update.go` with the version above.

- [ ] **Step 3: Hook the main-pane handler into `handleKey`**

In `internal/app/update.go`, in the focus-routed dispatch block (added in Task 11), add the `focusMain` case:

```go
	if m.search == nil && m.picker == nil && m.mode != modeBrowser {
		switch m.focusZ {
		case focusPlaylists:
			if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
				return mm, cmd
			}
		case focusMain:
			if mm, cmd, handled := handleMainKey(m, msg); handled {
				return mm, cmd
			}
		}
	}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/app/ -run TestMain -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/panel_main.go internal/app/update.go internal/app/panel_main_test.go
git commit -m "$(cat <<'EOF'
app: main pane cursor + ⏎-to-play in tracks mode

j/k navigates rows, ⏎ plays from cursor. Also handles searchPlayedMsg
unconditionally so a stray play-track error from the main pane surfaces in
the bottom error footer.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: Retire the `l` browser modal

The Playlists panel + main pane now do everything the browser modal did. Remove it.

**Files:**
- Delete: `internal/app/browser.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Move `scrollWindow` and `truncate` to a shared helpers file**

These are needed by `panel_playlists.go` and `panel_main.go` after `browser.go` goes away.

Create `internal/app/helpers.go`:

```go
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
```

- [ ] **Step 2: Delete `internal/app/browser.go`**

```bash
rm internal/app/browser.go
```

- [ ] **Step 3: Move `fetchPlaylists`, `fetchPlaylistTracks`, `playPlaylist` Cmds out of the deleted file**

These were defined in `browser.go`. Append them to `internal/app/tick.go` (they're transport Cmds, the existing pattern):

```go
// fetchPlaylists returns a Cmd that calls client.Playlists and produces
// a playlistsMsg.
func fetchPlaylists(c music.Client) tea.Cmd {
	return func() tea.Msg {
		playlists, err := c.Playlists(context.Background())
		return playlistsMsg{playlists: playlists, err: err}
	}
}

// fetchPlaylistTracks returns a Cmd that calls client.PlaylistTracks and
// produces a playlistTracksMsg.
func fetchPlaylistTracks(c music.Client, name string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.PlaylistTracks(context.Background(), name)
		return playlistTracksMsg{name: name, tracks: tracks, err: err}
	}
}

// playPlaylist returns a Cmd that calls client.PlayPlaylist and produces
// a playPlaylistMsg.
func playPlaylist(c music.Client, name string, fromIdx int) tea.Cmd {
	return func() tea.Msg {
		err := c.PlayPlaylist(context.Background(), name, fromIdx)
		return playPlaylistMsg{err: err}
	}
}
```

- [ ] **Step 4: Strip browser state and routing**

In `internal/app/model.go`:

- Delete the `viewMode`, `browserPane`, `browserState` types (around lines 62–89).
- Delete the `mode viewMode` and `browser *browserState` fields from `Model`.

In `internal/app/update.go`:

- Delete the `if m.mode == modeBrowser { ... }` short-circuit at the top of `handleKey`.
- Delete the `case "l":` block (around line 237).
- Update the `/` and `o` blocks: remove the `m.picker != nil || m.mode == modeBrowser` and `m.mode == modeBrowser` references — they no longer compile. The `/` block becomes:

```go
	case "/":
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		if m.picker != nil {
			return m, nil
		}
		m.search = &searchState{}
		return m, nil
```

- Update the focus-routed-dispatch block: change `m.search == nil && m.picker == nil && m.mode != modeBrowser` to `m.search == nil && m.picker == nil` (since `m.mode` no longer exists).
- In the `case playlistsMsg:` and `case playlistTracksMsg:` handlers: delete the `if m.browser != nil { ... }` blocks. Only the new-panel writes remain.

In `internal/app/view.go`:

- Delete the `if m.mode == modeBrowser { return renderBrowser(m) }` short-circuit.

- [ ] **Step 5: Strip / migrate browser tests**

In `internal/app/update_test.go`, delete every test whose name starts with `TestBrowser*`, `TestKeyLOpens*`, or that touches `m.browser` / `m.mode`. Run:

```bash
grep -n "TestBrowser\|TestKeyLOpens\|m\.browser\|m\.mode" internal/app/update_test.go
```

For each match, delete the surrounding test function. The semantics those tests covered are now in `panel_playlists_test.go` and `panel_main_test.go`.

Specifically delete: `TestKeyLOpensBrowserAndDispatchesFetch`, `TestPlaylistsMsgPopulatesState`, `TestPlaylistsMsgErrorStoredInState`, `TestBrowserLeftPaneDownMovesCursor`, `TestBrowserLeftPaneUpMovesCursor`, `TestBrowserLeftPaneCursorClampsAtBounds`, `TestBrowserLeftPaneNavigationDoesNotFetchTracks`, `TestBrowserTabSwitchesToRightPaneAndFetchesTracks`, `TestBrowserRightArrowAlsoSwitchesPane`, `TestBrowserShiftTabReturnsToLeftPaneNoFetch`, `TestPlaylistTracksMsgPopulatesState`, `TestPlaylistTracksMsgIgnoresStaleResult`, `TestBrowserEnterOnLeftPlaysWholePlaylist`, `TestBrowserEnterOnRightPlaysFromTrack`, `TestBrowserEnterOnRightWithEmptyTracksIsNoOp`, `TestBrowserRRefetchesPlaylistsOnLeftPane`, `TestBrowserRRefetchesTracksOnRightPane`, `TestBrowserEscReturnsToNowPlaying`, `TestBrowserModeTransportKeysStillFire`.

Then add minimal replacement coverage for the still-relevant globalbehaviour:

Append to `internal/app/update_test.go`:

```go
func TestPlaylistsMsgPopulatesPanelState(t *testing.T) {
	m := newTestModel()
	pls := []domain.Playlist{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(playlistsMsg{playlists: pls})
	got := updated.(Model)
	if len(got.playlists.items) != 2 {
		t.Errorf("playlists.items = %d entries; want 2", len(got.playlists.items))
	}
	if got.playlists.loading {
		t.Error("expected loading cleared")
	}
}

func TestPlaylistsMsgErrorStoredInPanelState(t *testing.T) {
	m := newTestModel()
	m.playlists.loading = true
	updated, _ := m.Update(playlistsMsg{err: errors.New("boom")})
	got := updated.(Model)
	if got.playlists.err == nil {
		t.Error("expected playlists.err set")
	}
	if got.playlists.loading {
		t.Error("expected loading cleared even on error")
	}
}
```

- [ ] **Step 6: Build and run**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: pressing `l` does nothing (the binding is gone). Playlists panel + main pane work as in Task 15. `space`/`n`/`p`/`+/-`/`q`/`/`/`o` still work as before.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
app: retire 'l' browser modal

browser.go deleted. mode/viewMode/browserPane/browserState types removed.
fetchPlaylists/fetchPlaylistTracks/playPlaylist Cmds moved to tick.go.
scrollWindow + truncate moved to helpers.go. Browser-modal tests deleted;
panel-state tests preserved.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: Phase 2 wrap-up

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Manual smoke walk**

`go run ./cmd/goove`. Verify:
- Playlists panel populates on first focus.
- `j`/`k` in Playlists moves cursor and live-previews tracks in main.
- ⏎ in Playlists plays the playlist.
- `4`/`Tab` to main, `j`/`k` navigates tracks, ⏎ plays from there.
- `space`/`n`/`p`/`+/-`/`q` work.
- `/` opens search modal (still working — the `/` keybind still exists).
- `o` opens output picker modal.
- `l` does nothing (intentionally retired).

- [ ] **Step 3: Tag**

```bash
git tag tui-overhaul-phase-2
```

---

# Phase 3 — Search panel

Search lives in the persistent left-stack panel. Typing query goes inline; ⏎ fires it; results land in the main pane in `mainPaneSearchResults` mode; focus jumps to main; Esc from main returns to selected playlist tracks. The `/` modal is retired at the end.

**End-of-phase outcome:** No more `/` modal; search is fully panel-based. `searchState` (the modal pointer) is removed from `Model`.

## Task 18: Wire input mode + typing

Focusing the Search panel and pressing a printable key enters input mode and starts the query. Typing more keys appends.

**Files:**
- Modify: `internal/app/panel_search.go`
- Modify: `internal/app/update.go`
- Create: `internal/app/panel_search_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/panel_search_test.go`:

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchPanelTypingEntersInputModeAndAppendsQuery(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := updated.(Model)
	if !got.search2.inputMode {
		t.Error("expected inputMode true after typing")
	}
	if got.search2.query != "l" {
		t.Errorf("query = %q; want %q", got.search2.query, "l")
	}
}

func TestSearchPanelMultipleKeysAppend(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	keys := []rune{'l', 'e', 'd'}
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = updated.(Model)
	}
	if m.search2.query != "led" {
		t.Errorf("query = %q; want %q", m.search2.query, "led")
	}
}

func TestSearchPanelBackspaceRemovesLastRune(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := updated.(Model)
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q", got.search2.query, "le")
	}
}

func TestSearchPanelEscClearsAndExitsInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.search2.inputMode {
		t.Error("expected inputMode false after esc")
	}
	if got.search2.query != "" {
		t.Errorf("query = %q; want empty", got.search2.query)
	}
}

func TestSearchPanelSpaceGoesIntoQuery(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "led"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.search2.query != "led " {
		t.Errorf("query = %q; want %q", got.search2.query, "led ")
	}
}

func TestSearchPanelNumberKeysStillJumpFocusInInputMode(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "le"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (1 always wins)", got.focusZ)
	}
	// Query unchanged.
	if got.search2.query != "le" {
		t.Errorf("query = %q; want %q (1 should not append)", got.search2.query, "le")
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestSearchPanel -v
```

Expected: FAIL — handlers don't exist yet.

- [ ] **Step 3: Add the panel handler**

In `internal/app/panel_search.go`, append:

```go
import tea "github.com/charmbracelet/bubbletea"

// handleSearchPanelKey routes keys when focusZ == focusSearch. Returns
// (model, cmd, handled). Number keys 1–4 are NOT handled here so they fall
// through to the global focus-jump cases (one of the disambiguation rules in
// the spec). Tab/Shift-Tab also fall through.
func handleSearchPanelKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	// Always pass-through: focus controls.
	switch msg.String() {
	case "tab", "shift+tab", "1", "2", "3", "4":
		return m, nil, false
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.search2.inputMode = false
		m.search2.query = ""
		m.search2.seq++
		m.search2.err = nil
		return m, nil, true
	case tea.KeyBackspace:
		if !m.search2.inputMode {
			return m, nil, true
		}
		runes := []rune(m.search2.query)
		if len(runes) > 0 {
			m.search2.query = string(runes[:len(runes)-1])
			m.search2.seq++
		}
		return m, nil, true
	case tea.KeySpace:
		m.search2.inputMode = true
		m.search2.query += " "
		m.search2.seq++
		return m, nil, true
	case tea.KeyRunes:
		m.search2.inputMode = true
		m.search2.query += string(msg.Runes)
		m.search2.seq++
		return m, nil, true
	case tea.KeyEnter:
		// Phase 3 task 19 wires this.
		return m, nil, true
	}
	return m, nil, false
}
```

- [ ] **Step 4: Hook into `update.go`**

In `internal/app/update.go`, add the `focusSearch` case to the focus-routed dispatch block:

```go
	if m.search == nil && m.picker == nil {
		switch m.focusZ {
		case focusPlaylists:
			if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
				return mm, cmd
			}
		case focusSearch:
			if mm, cmd, handled := handleSearchPanelKey(m, msg); handled {
				return mm, cmd
			}
		case focusMain:
			if mm, cmd, handled := handleMainKey(m, msg); handled {
				return mm, cmd
			}
		}
	}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/app/ -run TestSearchPanel -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/panel_search.go internal/app/update.go internal/app/panel_search_test.go
git commit -m "$(cat <<'EOF'
app: Search panel input mode, typing, backspace, esc

Printable keys append to query (entering input mode), backspace removes
last rune, esc clears and exits input mode. Tab/Shift-Tab/1-4 fall through
to globals so focus jumps still work mid-query.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 19: Render Search panel content

The Search panel renders one of three states: idle, input mode (with query), done (after a search).

**Files:**
- Modify: `internal/app/panel_search.go`

- [ ] **Step 1: Replace the placeholder render**

In `internal/app/panel_search.go`, replace `renderSearchPanel`:

```go
func renderSearchPanel(m Model, width, height int) string {
	title := "Search"
	body := renderSearchBody(m)
	return panelBox(title, body, width, height, m.focusZ == focusSearch)
}

func renderSearchBody(m Model) string {
	switch {
	case m.search2.inputMode && m.search2.loading:
		return titleStyle.Render("/"+m.search2.query) + "\n" + subtitleStyle.Render("searching…")
	case m.search2.inputMode:
		// Caret at end of query.
		return titleStyle.Render("/" + m.search2.query + "_")
	case m.search2.lastQuery != "":
		hits := fmt.Sprintf("%d results", m.search2.total)
		if m.search2.total > len(m.main.searchResults) {
			hits = fmt.Sprintf("%d of %d", len(m.main.searchResults), m.search2.total)
		}
		return titleStyle.Render("/"+m.search2.lastQuery) + "\n" + subtitleStyle.Render(hits)
	default:
		return subtitleStyle.Render("/  type to search")
	}
}
```

Update the imports to add `fmt`:

```go
import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: builds clean.

- [ ] **Step 3: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: focus Search (`2`), type some letters, see them appear in the panel. Press Backspace, see them remove. Press Esc, see the panel return to "type to search".

- [ ] **Step 4: Commit**

```bash
git add internal/app/panel_search.go
git commit -m "$(cat <<'EOF'
app: render Search panel for idle / input / done states

idle: muted '/  type to search'
input: '/<query>_' with blinking caret at end
done: '/<query>' + 'N results' on second line

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 20: ⏎ in Search panel fires the search

Pressing enter in input mode runs `client.SearchTracks` and routes the result into `m.main.searchResults`, switching main to `mainPaneSearchResults` mode and jumping focus.

**Files:**
- Modify: `internal/app/panel_search.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/panel_search_test.go`

- [ ] **Step 1: Add a panel-scoped result message type**

The existing `searchResultsMsg` is shaped for the modal. Add a new one for the panel flow:

In `internal/app/messages.go`, append:

```go
// searchPanelResultsMsg is the panel flow's analogue of searchResultsMsg.
// Carries seq + query for stale-result rejection.
type searchPanelResultsMsg struct {
	seq    uint64
	query  string
	result music.SearchResult
	err    error
}
```

- [ ] **Step 2: Add the firing helper**

In `internal/app/panel_search.go`, append:

```go
// fireSearchPanel dispatches a SearchTracks call. Used by the ⏎ handler.
func fireSearchPanel(c music.Client, seq uint64, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.SearchTracks(context.Background(), query)
		return searchPanelResultsMsg{seq: seq, query: query, result: res, err: err}
	}
}
```

Update imports:

```go
import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)
```

- [ ] **Step 3: Wire ⏎ in the Search panel handler**

In `internal/app/panel_search.go`, replace the `case tea.KeyEnter:` block in `handleSearchPanelKey`:

```go
	case tea.KeyEnter:
		if !m.search2.inputMode || m.search2.query == "" {
			return m, nil, true
		}
		m.search2.seq++
		m.search2.loading = true
		m.search2.err = nil
		return m, fireSearchPanel(m.client, m.search2.seq, m.search2.query), true
```

- [ ] **Step 4: Handle `searchPanelResultsMsg` at the top level**

In `internal/app/update.go`, in the top-level msg switch (the `Update` method body), add a new case for the panel-flow result. Insert it after the existing `case searchResultsMsg:` block:

```go
	case searchPanelResultsMsg:
		if msg.seq != m.search2.seq {
			return m, nil // stale
		}
		m.search2.loading = false
		m.search2.inputMode = false
		m.search2.lastQuery = msg.query
		m.search2.total = msg.result.Total
		m.search2.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		// Land results in main pane.
		m.main.mode = mainPaneSearchResults
		m.main.searchResults = domain.RankSearchResults(msg.result.Tracks, msg.query)
		m.main.cursor = 0
		// Focus jumps to main.
		m.focusZ = focusMain
		return m, nil
```

The `domain` import is already present in `update.go`.

- [ ] **Step 5: Write the tests**

Update the import block at the top of `internal/app/panel_search_test.go` to include `domain`, `music`, and `music/fake`:

```go
import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)
```

Then append the test functions to the same file:

```go
func TestSearchPanelEnterFiresSearch(t *testing.T) {
	c := fake.New()
	c.AddSearchTrack(domain.Track{Title: "Stairway", Artist: "Led Zeppelin", PersistentID: "p1"})
	m := New(c, nil)
	m.focusZ = focusSearch
	m.search2.inputMode = true
	m.search2.query = "stair"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected fireSearchPanel Cmd")
	}
	out := cmd()
	res, ok := out.(searchPanelResultsMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want searchPanelResultsMsg", out)
	}
	if res.query != "stair" {
		t.Errorf("query = %q", res.query)
	}
}

func TestSearchPanelResultsMsgPopulatesMainPane(t *testing.T) {
	m := newTestModel()
	m.search2.seq = 5
	m.search2.query = "stair"
	tracks := []domain.Track{{Title: "Stairway", Artist: "Led Zeppelin"}}
	updated, _ := m.Update(searchPanelResultsMsg{seq: 5, query: "stair", result: music.SearchResult{Tracks: tracks, Total: 1}})
	got := updated.(Model)
	if got.main.mode != mainPaneSearchResults {
		t.Errorf("main.mode = %v; want mainPaneSearchResults", got.main.mode)
	}
	if len(got.main.searchResults) != 1 {
		t.Errorf("searchResults = %d; want 1", len(got.main.searchResults))
	}
	if got.focusZ != focusMain {
		t.Errorf("focusZ = %v; want focusMain", got.focusZ)
	}
}

func TestSearchPanelStaleSeqDropped(t *testing.T) {
	m := newTestModel()
	m.search2.seq = 5
	updated, _ := m.Update(searchPanelResultsMsg{seq: 4, query: "old"})
	got := updated.(Model)
	if got.main.mode == mainPaneSearchResults {
		t.Error("stale seq should not have populated main pane")
	}
}

func TestSearchPanelEnterEmptyQueryNoOp(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusSearch
	m.search2.inputMode = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd on empty query, got %T", cmd())
	}
}
```

> The `fake.AddSearchTrack` helper is assumed to exist (see Phase 3 of the search plan). If it doesn't, the test can call `c.SetTrack(...)` instead and assert the cmd's existence without checking the result content.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/app/ -run TestSearchPanel -v
```

Expected: PASS.

- [ ] **Step 7: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app/panel_search.go internal/app/update.go internal/app/messages.go internal/app/panel_search_test.go
git commit -m "$(cat <<'EOF'
app: Search panel ⏎ fires search; results land in main pane

Adds searchPanelResultsMsg (modal vs panel flows now have separate result
types). On ⏎: searchTracks runs, results rank into main.searchResults,
main pane flips to mainPaneSearchResults mode, focus jumps to main.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 21: Esc in main pane returns to selected playlist tracks

Pressing Esc when main is in `mainPaneSearchResults` mode flips it back to `mainPaneTracks`.

**Files:**
- Modify: `internal/app/panel_main.go`
- Modify: `internal/app/panel_main_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/panel_main_test.go`:

```go
func TestMainPaneEscReturnsToTracksFromSearchResults(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "x"}}
	m.main.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.main.mode != mainPaneTracks {
		t.Errorf("main.mode after esc = %v; want mainPaneTracks", got.main.mode)
	}
	if got.main.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (reset)", got.main.cursor)
	}
}

func TestMainPaneEscInTracksModeIsNoOp(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusMain
	m.main.mode = mainPaneTracks
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.main.mode != mainPaneTracks {
		t.Errorf("main.mode after esc in tracks mode = %v; want unchanged", got.main.mode)
	}
}
```

- [ ] **Step 2: Add the esc case to `handleMainKey`**

In `internal/app/panel_main.go`, add to `handleMainKey`'s switch:

```go
	case "esc":
		if m.main.mode == mainPaneSearchResults {
			m.main.mode = mainPaneTracks
			m.main.cursor = 0
		}
		return m, nil, true
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/ -run TestMainPaneEsc -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/panel_main.go internal/app/panel_main_test.go
git commit -m "$(cat <<'EOF'
app: Esc in main pane returns to tracks from search results

mainPaneSearchResults → mainPaneTracks; cursor resets. Esc in tracks
mode is a no-op.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 22: Retire the `/` search modal

The Search panel + main pane now do everything the `/` modal did. Remove it.

**Files:**
- Delete: `internal/app/search.go`
- Delete: `internal/app/search_test.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/update_test.go`
- Modify: `internal/app/panel_search.go`

- [ ] **Step 1: Delete the modal source**

```bash
rm internal/app/search.go internal/app/search_test.go
```

- [ ] **Step 2: Strip modal state**

In `internal/app/model.go`:
- Delete the `searchState` struct (around lines 46–60).
- Delete the `search *searchState` field from `Model`.

In `internal/app/messages.go`:
- Delete the `searchDebounceMsg`, `searchResultsMsg`, and `searchPlayedMsg` types if they're no longer used. Check first:

```bash
grep -n "searchDebounceMsg\|searchResultsMsg\|searchPlayedMsg" internal/app/
```

- `searchDebounceMsg` and `searchResultsMsg` are no longer used (only used in modal flow). Delete.
- `searchPlayedMsg` IS still used by `playTrack` in `panel_main.go` for search-result enter. Keep it; we'll rename it to `playTrackResultMsg` in Phase 6 cleanup.

In `internal/app/update.go`:
- Delete the `if m.search != nil { return m.handleSearchKey(msg) }` short-circuit in `handleKey`.
- Delete the `case searchDebounceMsg:` and `case searchResultsMsg:` blocks.
- Update the `case searchPlayedMsg:` to remove the `if m.search != nil { ... }` branch (only the `lastError` path remains):

```go
	case searchPlayedMsg:
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, nil
```

- Delete the `case "/":` block in the `switch msg.String()`. Replace it with the new global `/` handler that focuses Search and starts input mode:

```go
	case "/":
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.focusZ = focusSearch
		m.search2.inputMode = true
		return m, nil
```

- Update the focus-routed dispatch guard: change `if m.search == nil && m.picker == nil { ... }` to `if m.picker == nil { ... }` (since `m.search` no longer exists).

In `internal/app/view.go`:
- Delete the `if m.search != nil { return renderSearch(m.search) }` short-circuit.

- [ ] **Step 3: Strip / migrate search-modal tests**

In `internal/app/update_test.go`, delete every test whose name contains "SearchModal" or that references `m.search` (the modal pointer) or `searchState`. Run:

```bash
grep -n "m\.search \|searchState\|searchDebounceMsg" internal/app/update_test.go
```

Delete each surrounding test function. Most of those behaviours are now covered by `panel_search_test.go` (typing, debounce-equivalent, ⏎ fires, esc clears).

Add this small replacement test for the global `/` keybind:

```go
func TestSlashKeyFocusesSearchAndEntersInputMode(t *testing.T) {
	c := fake.New()
	c.Launch(nil)
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focusZ != focusSearch {
		t.Errorf("focusZ = %v; want focusSearch", got.focusZ)
	}
	if !got.search2.inputMode {
		t.Error("expected inputMode true")
	}
}

func TestSlashIsNoOpInDisconnected(t *testing.T) {
	m := newTestModel() // starts in Disconnected
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := updated.(Model)
	if got.focusZ != focusPlaylists {
		t.Errorf("focusZ = %v; want focusPlaylists (no change in Disconnected)", got.focusZ)
	}
}
```

- [ ] **Step 4: Build and run**

```bash
go build ./...
go test ./...
```

Expected: PASS. If you see "undefined" errors in `update.go` related to `searchDebounceMsg`/`searchResultsMsg`, ensure the corresponding case blocks were deleted in Step 2.

- [ ] **Step 5: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: pressing `/` from anywhere focuses the Search panel and is ready for typing. Type a query, press ⏎, watch results land in main pane and focus jump there. Press Esc to return main to selected playlist's tracks. The old modal is gone.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
app: retire '/' search modal

search.go + search_test.go deleted. searchState type, searchDebounceMsg,
searchResultsMsg removed. '/' now focuses the Search panel and enters
input mode. Modal-flow update-tests deleted; global '/' coverage added.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 23: Phase 3 wrap-up

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Manual smoke walk**

`go run ./cmd/goove`. Verify:
- `/` from any panel jumps to Search and starts input mode.
- Typing appears inline. Backspace works. Esc clears.
- ⏎ fires search; main pane shows results; focus on main.
- `j`/`k` in main navigates; ⏎ plays a result.
- Esc in main returns to selected playlist tracks.
- All globals (`space`/`n`/`p`/`+/-`/`q`) work everywhere except mid-input.

- [ ] **Step 3: Tag**

```bash
git tag tui-overhaul-phase-3
```

---

# Phase 4 — Output panel

The Output panel replaces the `o` picker modal. Two-step semantics: cursor moves don't switch device, ⏎ does.

**End-of-phase outcome:** `o` jumps focus to the Output panel; `j`/`k` navigate devices; ⏎ switches. The picker modal is retired.

## Task 24: Wire device fetch on first focus + render

**Files:**
- Modify: `internal/app/panel_output.go`
- Modify: `internal/app/update.go`
- Create: `internal/app/panel_output_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/panel_output_test.go`:

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestFocusingOutputFiresFetchWhenEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(nil)
	m := New(c, nil)
	m.focusZ = focusPlaylists
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	got := updated.(Model)
	if got.focusZ != focusOutput {
		t.Fatalf("focusZ = %v; want focusOutput", got.focusZ)
	}
	if cmd == nil {
		t.Fatal("expected fetchDevices Cmd")
	}
	if _, ok := cmd().(devicesMsg); !ok {
		t.Fatalf("cmd produced %T; want devicesMsg", cmd())
	}
}

func TestFocusingOutputDoesNotRefetchWhenCached(t *testing.T) {
	m := newTestModel()
	m.output.devices = []domain.AudioDevice{{Name: "MacBook"}}
	m.focusZ = focusPlaylists
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when devices cached, got %T", cmd())
	}
}

func TestDevicesMsgPopulatesOutputPanel(t *testing.T) {
	m := newTestModel()
	m.output.loading = true
	updated, _ := m.Update(devicesMsg{devices: []domain.AudioDevice{
		{Name: "MacBook", Selected: true}, {Name: "Sonos"},
	}})
	got := updated.(Model)
	if len(got.output.devices) != 2 {
		t.Errorf("devices = %d; want 2", len(got.output.devices))
	}
	if got.output.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (lands on selected)", got.output.cursor)
	}
}

func TestOutputCursorMovesWithJK(t *testing.T) {
	m := newTestModel()
	m.focusZ = focusOutput
	m.output.devices = []domain.AudioDevice{{Name: "A"}, {Name: "B"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.output.cursor != 1 {
		t.Errorf("cursor = %d; want 1", got.output.cursor)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/app/ -run TestFocusingOutput -run TestDevicesMsg -run TestOutputCursor -v
```

Expected: FAIL.

- [ ] **Step 3: Add the on-focus hook + key handler**

In `internal/app/panel_output.go`, replace the file with:

```go
package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
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
	if m.output.err != nil {
		return errorStyle.Render("error: " + m.output.err.Error())
	}
	if len(m.output.devices) == 0 {
		return subtitleStyle.Render("(no devices)")
	}
	visibleRows := height - 4
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

// _ = music ensures the music import is referenced even if other helpers move.
var _ = music.ErrNotRunning
```

- [ ] **Step 4: Wire `onFocusOutput` into `update.go`**

In `internal/app/update.go`, modify `onFocusEntered`:

```go
func (m Model) onFocusEntered() (tea.Model, tea.Cmd) {
	switch m.focusZ {
	case focusPlaylists:
		mm, cmd := onFocusPlaylists(m)
		return mm, cmd
	case focusOutput:
		mm, cmd := onFocusOutput(m)
		return mm, cmd
	}
	return m, nil
}
```

Add the `focusOutput` case to the focus-routed dispatch:

```go
	if m.picker == nil {
		switch m.focusZ {
		case focusPlaylists:
			if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
				return mm, cmd
			}
		case focusSearch:
			if mm, cmd, handled := handleSearchPanelKey(m, msg); handled {
				return mm, cmd
			}
		case focusOutput:
			if mm, cmd, handled := handleOutputKey(m, msg); handled {
				return mm, cmd
			}
		case focusMain:
			if mm, cmd, handled := handleMainKey(m, msg); handled {
				return mm, cmd
			}
		}
	}
```

Update the existing `case devicesMsg:` block in `update.go` (around line 60) to also populate `m.output`:

```go
	case devicesMsg:
		// Phase 4: populate the persistent panel state.
		m.output.loading = false
		m.output.err = msg.err
		if msg.err == nil {
			m.output.devices = msg.devices
			for i, d := range msg.devices {
				if d.Selected {
					m.output.cursor = i
					break
				}
			}
		}
		// Existing picker-modal write (Phase 4 still keeps the modal alive):
		if m.picker != nil {
			m.picker.loading = false
			m.picker.err = msg.err
			m.picker.devices = msg.devices
			for i, d := range msg.devices {
				if d.Selected {
					m.picker.cursor = i
					break
				}
			}
		}
		return m, nil
```

Update the existing `case deviceSetMsg:` to also clear `m.output.loading` on success:

```go
	case deviceSetMsg:
		// Phase 4: panel flow.
		m.output.loading = false
		if msg.err != nil {
			m.output.err = msg.err
		} else {
			// Refresh device list to pick up the new Selected flag.
			return m, fetchDevices(m.client)
		}
		// Existing picker-modal write:
		if m.picker != nil {
			if msg.err != nil {
				m.picker.loading = false
				m.picker.err = msg.err
				return m, nil
			}
			m.picker = nil
		}
		return m, nil
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/app/ -run TestFocusingOutput -run TestDevicesMsg -run TestOutputCursor -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/panel_output.go internal/app/update.go internal/app/panel_output_test.go
git commit -m "$(cat <<'EOF'
app: Output panel — fetch on first focus, j/k cursor, ⏎ switches

Two-step semantics (Q3-C): cursor moves don't switch device. devicesMsg
now populates both the new panel state and the (still-alive) picker
modal state. Same for deviceSetMsg.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 25: Retire the `o` picker modal

**Files:**
- Delete: `internal/app/picker.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/messages.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Delete the modal source**

```bash
rm internal/app/picker.go
```

- [ ] **Step 2: Strip modal state**

In `internal/app/model.go`:
- Delete the `pickerState` struct (around lines 36–44).
- Delete the `picker *pickerState` field from `Model`.

In `internal/app/update.go`:
- Delete the `if m.picker != nil { return m.handlePickerKey(msg) }` short-circuit.
- Delete the entire `handlePickerKey` method.
- Delete the `if m.picker != nil` branch of the existing `case "o":` block. Replace with:

```go
	case "o":
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.focusZ = focusOutput
		mm, cmd := onFocusOutput(m)
		return mm, cmd
```

- Update the focus-routed dispatch guard: now that there are no modals, drop the `if m.picker == nil` wrapper. The block becomes:

```go
	switch m.focusZ {
	case focusPlaylists:
		if mm, cmd, handled := handlePlaylistsKey(m, msg); handled {
			return mm, cmd
		}
	case focusSearch:
		if mm, cmd, handled := handleSearchPanelKey(m, msg); handled {
			return mm, cmd
		}
	case focusOutput:
		if mm, cmd, handled := handleOutputKey(m, msg); handled {
			return mm, cmd
		}
	case focusMain:
		if mm, cmd, handled := handleMainKey(m, msg); handled {
			return mm, cmd
		}
	}
```

- In `case devicesMsg:` and `case deviceSetMsg:`: delete the `if m.picker != nil { ... }` branches; only the new-panel writes remain.

In `internal/app/view.go`:
- Delete the `if m.picker != nil { return renderPicker(m.picker) }` short-circuit.

- [ ] **Step 3: Strip / migrate picker tests**

In `internal/app/update_test.go`, delete every test whose name contains `Picker` or that references `m.picker`. The grep:

```bash
grep -n "Picker\|m\.picker" internal/app/update_test.go
```

Delete the surrounding functions: `TestOKeyOpensPickerInConnected`, `TestOKeyOpensPickerInIdle`, `TestOKeyIsNoOpInDisconnected`, `TestOKeyIsNoOpWhenPermissionDenied`, `TestPickerArrowsNavigateCursor`, `TestPickerArrowsClampAtBoundaries`, `TestPickerVIKeysAlsoNavigate`, `TestPickerEscClosesPicker`, `TestPickerQAlsoCloses`, `TestPickerEnterTriggersSetAirPlayDevice`, `TestPickerWhileLoadingOnlyEscWorks`, `TestTransportKeysSuppressedWhilePickerOpen`, `TestDevicesMsgPopulatesPicker`, `TestDevicesMsgErrorShownInPicker`, `TestDevicesMsgIgnoredWhenPickerClosed`, `TestDeviceSetMsgSuccessClosesPicker`, `TestDeviceSetMsgErrorKeepsPickerOpen`, `TestDeviceSetMsgIgnoredWhenPickerClosed`.

Add this replacement test for the global `o` keybind:

```go
func TestOKeyFocusesOutputPanelAndDispatchesFetch(t *testing.T) {
	c := fake.New()
	c.Launch(nil)
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, false)
	m := New(c, nil)
	np := domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}
	tmp, _ := m.Update(statusMsg{now: np})
	m = tmp.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)
	if got.focusZ != focusOutput {
		t.Errorf("focusZ = %v; want focusOutput", got.focusZ)
	}
	if cmd == nil {
		t.Fatal("expected fetchDevices Cmd")
	}
	if _, ok := cmd().(devicesMsg); !ok {
		t.Fatalf("cmd produced %T; want devicesMsg", cmd())
	}
}
```

- [ ] **Step 4: Build and run**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Smoke-test**

```bash
go run ./cmd/goove
```

Expected: `o` jumps focus to Output panel and triggers a device fetch. `j`/`k`/⏎ navigate and switch. The old modal is gone.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
app: retire 'o' picker modal

picker.go deleted. pickerState type + handlePickerKey removed. 'o' now
focuses the Output panel and dispatches fetchDevices. Picker-modal tests
deleted; global 'o' coverage added.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 26: Phase 4 wrap-up

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Manual smoke walk**

`go run ./cmd/goove`. Verify:
- `o` jumps to Output and fetches devices.
- `3` does the same.
- `j`/`k` navigates (no device change).
- ⏎ switches device.
- All globals + Playlists + Search + Main still work.

- [ ] **Step 3: Tag**

```bash
git tag tui-overhaul-phase-4
```

---

# Phase 5 — Album art in the now-playing panel

Move the chafa rendering into the panel.

**End-of-phase outcome:** Album art appears in the now-playing panel when terminal width ≥ `artLayoutThreshold`. Auto-hides on narrower terminals. Track-change cache invalidation is preserved.

## Task 27: Test panel renders with art when available and width is sufficient

The art rendering already works in the existing `renderConnectedCard`. The Phase 1 task 5 extraction already preserved that behaviour. This phase mostly confirms it still works in the new layout and adds explicit tests.

**Files:**
- Create: `internal/app/panel_now_playing_test.go`

- [ ] **Step 1: Write the tests**

Create `internal/app/panel_now_playing_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestNowPlayingRendersConnectedTrack(t *testing.T) {
	m := newTestModel()
	m.state = Connected{Now: domain.NowPlaying{
		Track:  domain.Track{Title: "Stairway", Artist: "Led Zeppelin"},
		Volume: 50,
	}}
	got := renderNowPlayingPanel(m)
	if !strings.Contains(got, "Stairway") {
		t.Errorf("missing title: %q", got)
	}
	if !strings.Contains(got, "Led Zeppelin") {
		t.Errorf("missing artist: %q", got)
	}
}

func TestNowPlayingRendersIdle(t *testing.T) {
	m := newTestModel()
	m.state = Idle{Volume: 50}
	got := renderNowPlayingPanel(m)
	if !strings.Contains(got, "nothing playing") && !strings.Contains(got, "Music is open") {
		t.Errorf("idle missing expected text: %q", got)
	}
}

func TestNowPlayingRendersDisconnected(t *testing.T) {
	m := newTestModel()
	m.state = Disconnected{}
	got := renderNowPlayingPanel(m)
	if !strings.Contains(got, "isn't running") && !strings.Contains(got, "Music") {
		t.Errorf("disconnected missing expected text: %q", got)
	}
}

func TestNowPlayingArtAppearsWhenWideAndCached(t *testing.T) {
	m := newTestModel()
	m.width = 100 // > artLayoutThreshold (70)
	track := domain.Track{Title: "T", Artist: "A", Album: "Al"}
	m.state = Connected{Now: domain.NowPlaying{Track: track, Volume: 50}}
	m.art = artState{key: trackKey(track), output: "ART_OUTPUT_HERE"}
	got := renderNowPlayingPanel(m)
	if !strings.Contains(got, "ART_OUTPUT_HERE") {
		t.Errorf("expected art content; got %q", got)
	}
}

func TestNowPlayingArtHiddenBelowThreshold(t *testing.T) {
	m := newTestModel()
	m.width = 50 // < artLayoutThreshold
	track := domain.Track{Title: "T", Artist: "A", Album: "Al"}
	m.state = Connected{Now: domain.NowPlaying{Track: track, Volume: 50}}
	m.art = artState{key: trackKey(track), output: "ART_OUTPUT_HERE"}
	got := renderNowPlayingPanel(m)
	if strings.Contains(got, "ART_OUTPUT_HERE") {
		t.Errorf("expected art hidden below threshold; got %q", got)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/app/ -run TestNowPlaying -v
```

Expected: PASS — `renderConnectedCardOnly` already gates art rendering on `width >= artLayoutThreshold` from Task 5.

- [ ] **Step 3: Commit**

```bash
git add internal/app/panel_now_playing_test.go
git commit -m "$(cat <<'EOF'
app: explicit tests for now-playing panel art behaviour

Confirms art renders when wide + cached, hides below threshold, and the
Idle/Disconnected states still produce sensible content.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 28: Phase 5 wrap-up

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Manual smoke walk**

`go run ./cmd/goove` in a wide terminal (≥ 80 cols). Verify:
- Album art renders left of the track info in the now-playing panel.
- Resize terminal smaller than `artLayoutThreshold` (70 cols): art disappears, layout still readable.
- Skip to a different track (`n`): art updates after the next status tick + chafa render.

- [ ] **Step 3: Tag**

```bash
git tag tui-overhaul-phase-5
```

---

# Phase 6 — Cleanup + docs

Remove transitional naming (`focusZ` → `focus`, `search2` → `search`), strip dead code, update README and `goove help`.

**End-of-phase outcome:** Codebase is clean — no temp names, no dead state types, no references to retired modals. README screenshot and Keys table reflect the new layout.

## Task 29: Drop `compactThreshold` / `renderCompact`

The compact path (single-line "▶ Title — Artist  vol N%" for terminals < 50 cols) doesn't fit the new layout's panel-rich shape. Replace with a "terminal too narrow" hint at the same threshold.

**Files:**
- Modify: `internal/app/view.go`

- [ ] **Step 1: Replace `renderCompact` invocation**

In `internal/app/view.go`, replace the `if m.width > 0 && m.width < compactThreshold { return renderCompact(m) }` line with:

```go
	if m.width > 0 && m.width < compactThreshold {
		return renderTooNarrow()
	}
```

Add the new helper at the bottom of `view.go`:

```go
func renderTooNarrow() string {
	return errorStyle.Render("terminal too narrow — make the window wider (≥ 50 cols)")
}
```

Delete the existing `renderCompact` function entirely.

- [ ] **Step 2: Build and run**

```bash
go build ./...
go test ./...
```

Expected: PASS. (If a test references `renderCompact`, delete that test — there shouldn't be any, but check.)

- [ ] **Step 3: Commit**

```bash
git add internal/app/view.go
git commit -m "$(cat <<'EOF'
app: replace compact layout with too-narrow hint

The single-line compact path (< 50 cols) doesn't fit the multi-panel
layout. Replace with a centred 'terminal too narrow' message at the same
threshold.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 30: Rename `focusZ` → `focus` and `search2` → `search`

Now that no struct field collides with these names (modal types are gone), rename for clarity. Three renames in dependency order:

1. Type `focus` → `focusKind` (frees up the `focus` identifier).
2. Field `focusZ` → `focus`.
3. Field `search2` → `search`.

**Files:** `model.go`, `focus.go`, `focus_test.go`, `panel_*.go`, `update.go`, `view.go`, `hints.go`, `hints_test.go`, `update_test.go`, `panel_*_test.go`.

> Don't use `sed` for these renames. Per-file edits are safer because (1) `focus` is a substring of `focusPlaylists` etc. (which must NOT be renamed), and (2) macOS BSD `sed` doesn't support `\b` word boundaries.

- [ ] **Step 1: Rename the type `focus` → `focusKind` (constants keep their names)**

In `internal/app/focus.go`:

```go
type focusKind int

const (
	focusPlaylists focusKind = iota
	focusSearch
	focusOutput
	focusMain
)

func nextFocus(f focusKind) focusKind {
	return (f + 1) % 4
}

func prevFocus(f focusKind) focusKind {
	return (f + 3) % 4
}
```

In `internal/app/focus_test.go`, change the test struct's `from, want focus` to `from, want focusKind` (two occurrences).

In `internal/app/model.go`, change the `Model` field `focusZ focus` to `focusZ focusKind`.

In `internal/app/hints.go`, in `panelHint`, the parameter list of any helper that takes `focus` becomes `focusKind`. Search the file for ` focus` (with a leading space) and update.

In `internal/app/hints_test.go`, the loop variable in `TestHintBarAlwaysContainsGlobals` is currently `for _, f := range []focus{...}`. Change to `[]focusKind{...}`.

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: builds clean. Only `type focusKind int` exists; constants unchanged; field still named `focusZ`.

If you get errors mentioning `focus` as an undeclared type, search for any remaining `var x focus` / `f focus`-style declarations and update them.

- [ ] **Step 3: Rename the field `focusZ` → `focus`**

In `internal/app/model.go`, change:

```go
	focusZ    focusKind
```

to:

```go
	focus    focusKind
```

Then in EVERY `.go` file in `internal/app/` (source and tests), replace every occurrence of `m.focusZ` with `m.focus` and `.focusZ` with `.focus`. Use grep to find them:

```bash
grep -rn 'focusZ' internal/app/
```

For each match, open the file in your editor and rename. Don't use sed — `focusZ` is a unique-enough token that find-replace in your editor is safe and fast.

After all edits:

```bash
grep -rn 'focusZ' internal/app/
```

Expected: no output.

- [ ] **Step 4: Rename `search2` → `search`**

In `internal/app/model.go`, change:

```go
	search2   searchPanel
```

to:

```go
	search    searchPanel
```

Then in EVERY `.go` file in `internal/app/`, replace `m.search2` with `m.search` and `.search2` with `.search`. Use grep:

```bash
grep -rn 'search2' internal/app/
```

For each match, rename in your editor. After all edits:

```bash
grep -rn 'search2' internal/app/
```

Expected: no output.

- [ ] **Step 5: Build and test**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/
git commit -m "$(cat <<'EOF'
app: drop transitional names

type focus → type focusKind (avoids field/type shadowing).
m.focusZ → m.focus.
m.search2 → m.search (now that *searchState is gone).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 31: Rename `searchPlayedMsg` → `playTrackResultMsg`

`searchPlayedMsg` is a naming holdover from the modal flow. Rename for clarity.

**Files:**
- Modify: `internal/app/messages.go`
- Modify: `internal/app/panel_main.go`
- Modify: `internal/app/update.go`

- [ ] **Step 1: Bulk rename**

```bash
cd internal/app
grep -rln 'searchPlayedMsg' . | xargs sed -i.bak 's/searchPlayedMsg/playTrackResultMsg/g'
rm *.bak
cd ../..
```

- [ ] **Step 2: Update the `messages.go` doc comment**

In `internal/app/messages.go`, update the comment above `playTrackResultMsg` to reflect that it's the result of any `client.PlayTrack` call (no more search-modal context).

```go
// playTrackResultMsg carries the result of a PlayTrack call (used when ⏎ is
// pressed on a search result in the main pane). On error, the error footer
// surfaces it; on success, the next status tick reflects the new now-playing.
type playTrackResultMsg struct {
	err error
}
```

(Note: the `seq` field referenced by the modal-era version is dropped — the panel flow doesn't need it.)

If the type was using `seq`, also update the `playTrack` Cmd in `panel_main.go` to drop the seq:

```go
func playTrack(c music.Client, persistentID string) tea.Cmd {
	return func() tea.Msg {
		return playTrackResultMsg{err: c.PlayTrack(context.Background(), persistentID)}
	}
}
```

And the case in `update.go`:

```go
	case playTrackResultMsg:
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, nil
```

- [ ] **Step 3: Build and test**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/
git commit -m "$(cat <<'EOF'
app: rename searchPlayedMsg → playTrackResultMsg

Drops the modal-flow seq field; pure error transport now. Used by main
pane ⏎ in mainPaneSearchResults mode.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 32: Update README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Re-screenshot the layout**

The README's existing ASCII screenshot is the old single-pane card. Replace it with one that reflects the new layout.

In `README.md`, replace the existing code block at the top:

````markdown
```
┌─ goove ──────────────────────────────────────────────┐
│                                                      │
│   ▶  Stairway to Heaven                              │
│      Led Zeppelin                                    │
│      Led Zeppelin IV                                 │
│                                                      │
│      ▮▮▮▮▮▮▮▮▯▯▯▯▯▯▯▯▯▯▯▯▯   3:42 / 8:02            │
│                                                      │
│      volume  ▮▮▮▮▮▯▯▯▯▯   50%                        │
│                                                      │
└──────────────────────────────────────────────────────┘
 space: play/pause   n: next   p: prev   +/-: vol   /: search   o: output   l: browse   q: quit
```
````

with:

````markdown
```
┌─ goove ──────────────────────────────────────────────────┐
│ ┌─ Now Playing ──────────────────────────────────────────┐ │
│ │  ▓ART▓  ▶  Stairway to Heaven                          │ │
│ │         Led Zeppelin · Led Zeppelin IV                 │ │
│ │         ▮▮▮▮▮▮▮▮▯▯▯▯▯▯▯▯▯  3:42 / 8:02   vol 50%        │ │
│ └────────────────────────────────────────────────────────┘ │
│ ┌Playlists────┐ ┌─ Liked Songs (now playing) ────────────┐ │
│ │▶ Liked Songs│ │   1. Black Dog          Led Zeppelin   │ │
│ │  Recent     │ │   2. Rock and Roll      Led Zeppelin   │ │
│ │  Top 25     │ │ ▶ 3. Stairway to Heaven Led Zeppelin   │ │
│ └─────────────┘ │   4. Misty Mountain Hop Led Zeppelin   │ │
│ ┌Search───────┐ │                                        │ │
│ │ /led ze     │ │                                        │ │
│ │  3 results  │ │                                        │ │
│ └─────────────┘ │                                        │ │
│ ┌Output───────┐ │                                        │ │
│ │▶ MacBook    │ │                                        │ │
│ │  Sonos      │ │                                        │ │
│ └─────────────┘ └────────────────────────────────────────┘ │
│ space:play/pause  n:next  p:prev  +/-:vol  q:quit  · j/k:nav  ⏎:play │
└──────────────────────────────────────────────────────────┘
```
````

- [ ] **Step 2: Replace the Keys table**

Find the existing Keys table and replace with:

```markdown
## Keys

### Globals (work everywhere)

| key | action |
|---|---|
| `space` | play / pause (or launch Music if Disconnected) |
| `n` | next track |
| `p` | previous track |
| `+` / `=` | volume +5% |
| `-` | volume −5% |
| `q` | quit |
| `Tab` / `Shift-Tab` | cycle focus through Playlists → Search → Output → Main |
| `1` / `2` / `3` / `4` | jump focus to Playlists / Search / Output / Main |
| `/` | focus the Search panel and start typing |
| `o` | focus the Output panel |

### Panel-scoped

| panel | key | action |
|---|---|---|
| Playlists | `j` / `k` / `↑` / `↓` | move cursor (live-previews tracks in main pane) |
| Playlists | `⏎` | play the highlighted playlist |
| Search (idle) | any printable | enter input mode and start the query |
| Search (input) | `Backspace` | remove last rune |
| Search (input) | `⏎` | run the search; results show in main pane |
| Search (input) | `Esc` | clear and exit input mode |
| Output | `j` / `k` / `↑` / `↓` | move cursor |
| Output | `⏎` | switch audio to the highlighted device |
| Main | `j` / `k` / `↑` / `↓` | move cursor |
| Main | `⏎` | play the highlighted track |
| Main | `Esc` | (search-results mode only) return to selected playlist |
```

- [ ] **Step 3: Verify the rest of the README is still accurate**

CLI section — unchanged.
Logs / Development sections — unchanged.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
readme: sync with new multi-panel TUI

New screenshot showing the four-zone layout. Keys table split into
Globals and Panel-scoped sections.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 33: Final test sweep + tag

- [ ] **Step 1: Run the full suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run integration tests on macOS**

```bash
go test -tags=integration ./internal/music/applescript/
```

Expected: PASS.

- [ ] **Step 3: Vet and format**

```bash
go vet ./...
gofmt -l ./internal/app/
```

Expected: no output from either.

- [ ] **Step 4: Final smoke walk**

`go run ./cmd/goove`. Walk every keybind from the README's Keys table and verify each works.

- [ ] **Step 5: Tag**

```bash
git tag tui-overhaul-complete
```

- [ ] **Step 6: Open the PR**

```bash
git push -u origin feature/tui-overhaul
gh pr create --title "TUI overhaul — LazyGit-inspired multi-panel layout" --body "$(cat <<'EOF'
## Summary
- Replaces the single-screen now-playing view + three modal overlays with a persistent four-zone layout (now-playing on top, Playlists/Search/Output stacked left, main pane right).
- Migration phased into six sub-changes; each phase ships independently and leaves the app working.
- CLI / domain / music client / AppleScript layers untouched.

Spec: `docs/superpowers/specs/2026-05-04-tui-overhaul-design.md`
Plan: `docs/superpowers/plans/2026-05-04-tui-overhaul.md`

## Test plan
- [ ] All unit tests pass (`go test ./...`)
- [ ] AppleScript integration tests pass on macOS (`go test -tags=integration ./internal/music/applescript/`)
- [ ] Manual: walk every keybind from the new Keys table (README) on real Music.app
- [ ] Manual: verify graceful behaviour at narrow terminal widths

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## End of plan

Six phases, ~33 tasks, ~150 steps. Each task is reviewable in isolation. Each phase ends in a tagged shippable commit.
