# goove Queue Management UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Layer a goove-owned interactive FIFO queue on top of the existing read-only Up Next. Tracks enqueued from Main panel rows with `a` play after the current track ends; `Q` opens a full-screen modal overlay with cursor, remove, reorder, jump-to-play, and clear-with-confirm. When the queue drains, playback resumes the interrupted playlist via `PlayPlaylist(name, --track index)`.

**Architecture:** Four layers, bottom-up.
1. **Queue state** — `QueueState` value type (FIFO `[]domain.Track`) with `Add`, `RemoveAt`, `MoveUp`, `MoveDown`, `PopHead`, `Clear`, `Len`. Lives on `Model.queue`.
2. **Handoff state machine** — `ResumeContext` (playlist name + 1-based next index) + `lastTrackPID` / `lastPlaylist` / `lastTrackIdx` / `pendingJumpPID` fields on `Model`. A new `handleQueueHandoff` method runs each status tick after the existing artwork / prefetch logic, comparing previous-tick state against the current tick's `Now` to decide whether to dispatch `PlayTrack(queueHead)` (intercept), `PlayPlaylist(resume)` (drain), or do nothing.
3. **Up Next teaser** — existing `renderUpNext` gains a `queue []domain.Track` parameter; renders `★ Title — Artist` queue rows above the existing playlist tail rows, prioritising queue rows when vertical room is tight. Placeholders interact per spec §3.5.
4. **Overlay** — new modal panel rendered over the four-panel layout when `m.overlay.open`. Intercepts all keys (globals suppressed). Keys: `j/k` cursor, `Enter` play-and-set-jump-flag, `x` remove, `K/J` reorder, `c`+`y` clear with confirm, `Esc/Q` close.

Plus: global `a` (enqueue focused Main row), global `Q` (open overlay), modified global `n` (play queue head when non-empty), hints bar update, CLI `queue` stub.

**Tech Stack:** Go 1.24, bubbletea + lipgloss. Spec: `docs/superpowers/specs/2026-05-13-queue-management-design.md`.

---

## File Structure

```
goove/
├── docs/superpowers/
│   ├── specs/2026-05-13-queue-management-design.md   # spec (existing)
│   └── plans/2026-05-13-queue-management.md          # this plan
└── internal/
    ├── app/
    │   ├── queue.go                  # T2 (new): QueueState type + methods
    │   ├── queue_test.go             # T2 (new): unit tests
    │   ├── resume.go                 # T5 (new): ResumeContext + handleQueueHandoff
    │   ├── resume_test.go            # T5 (new): table-driven handoff tests
    │   ├── panel_queue.go            # T8 + T9 (new): overlay render + key handler
    │   ├── panel_queue_test.go       # T8 + T9 (new): overlay render + key tests
    │   ├── model.go                  # T3 + T4 + T5 + T8: embed new fields
    │   ├── update.go                 # T4 + T6 + T10 + T11: route 'a'/'Q'/'n', wire handoff, route to overlay
    │   ├── update_test.go            # T4 + T6 + T10 + T11: wiring tests
    │   ├── view.go                   # T8: render overlay when open
    │   ├── panel_now_playing.go      # T7: renderUpNext takes queue param
    │   ├── panel_now_playing_test.go # T7: merge-render tests
    │   ├── panel_main.go             # T4: tested via update_test.go (no direct edit needed for 'a')
    │   └── hints.go                  # T12: add a/Q to globals; suppress when overlay open
    ├── cli/
    │   └── cli.go                    # T13: register "queue" subcommand stub
    └── music/
        └── applescript/
            └── client_integration_test.go  # T14: extend with handoff e2e test
```

No new dependencies. No changes to `domain`, the `music.Client` interface, or AppleScript scripts — all data and primitives already exist as of the read-only Up Next work.

## Naming and signature contract

| Symbol | Definition |
|---|---|
| `app.QueueState` | New struct in `internal/app/queue.go`. Single field `Items []domain.Track`. FIFO; `Items[0]` is the head. Mutated via methods only (no direct field access from outside the package would be safer, but methods are public so tests can compose easily). |
| `(q *QueueState) Add(t domain.Track)` | Append `t` to `Items`. Always succeeds; duplicates allowed. |
| `(q *QueueState) RemoveAt(i int)` | Delete `Items[i]`. No-op if `i < 0` or `i >= len(Items)`. |
| `(q *QueueState) MoveUp(i int) int` | Swap `Items[i]` with `Items[i-1]`. Returns the new index of the moved item (`i-1` on success, `i` if at edge or out of range). |
| `(q *QueueState) MoveDown(i int) int` | Swap `Items[i]` with `Items[i+1]`. Returns the new index of the moved item (`i+1` on success, `i` if at edge or out of range). |
| `(q *QueueState) PopHead() (domain.Track, bool)` | Remove and return `Items[0]`. Returns `(zero, false)` on empty. |
| `(q *QueueState) Clear()` | Replace `Items` with `nil`. |
| `(q QueueState) Len() int` | Return `len(q.Items)`. Value receiver — read-only. |
| `app.ResumeContext` | New struct in `internal/app/resume.go`. Fields `PlaylistName string` (empty = no resume target) and `NextIndex int` (1-based; argument to `client.PlayPlaylist`'s `fromTrackIndex`). |
| `(m Model) handleQueueHandoff(now domain.NowPlaying, prevPID, prevPlaylist string, prevIdx int) (Model, tea.Cmd)` | The handoff state machine from spec §3.2 / §3.4. Reads previous-tick cached values from arguments, mutates `m.queue`, `m.resume`, `m.pendingJumpPID`. Returns updated `Model` plus the `PlayTrack` or `PlayPlaylist` Cmd to dispatch (or `nil`). |
| `(m Model) indexOfPID(pid, playlistName string) int` | Helper on Model. Returns the 1-based index of `pid` inside the cached `tracksByName[playlistName]`, or `0` if not found / not cached / `pid` empty. |
| `app.overlayState` | New struct in `internal/app/panel_queue.go`. Fields `open bool` and `cursor int`. |
| `Model.queue QueueState` | New field on `Model`. Zero value is an empty queue — no initialisation needed in `New`. |
| `Model.resume ResumeContext` | New field. Zero value = no resume target. |
| `Model.lastTrackPID string` | Persistent ID seen on the previous status tick. Zero value (`""`) is the launch state. |
| `Model.lastPlaylist string` | `CurrentPlaylistName` seen on the previous status tick. |
| `Model.lastTrackIdx int` | 1-based index of last-seen track in `lastPlaylist`'s cached tracks. `0` if not found / not cached. |
| `Model.overlay overlayState` | New field. Zero value = closed. |
| `Model.clearPrompt bool` | True while waiting for `y`/anything-else after `c` in the overlay. Reset to false on overlay close or any non-`y` key. |
| `Model.pendingJumpPID string` | One-shot flag: PID the next status tick should treat as our own jump (skip both intercept and head-pop branches). Set by overlay `Enter`. Cleared by the handoff handler on the matching tick. |
| `renderOverlay(m Model, width, height int) string` | New in `panel_queue.go`. Full-area render of the queue overlay (header, body or empty-state, resume footer, help row or clear-prompt). |
| `updateOverlay(m Model, msg tea.KeyMsg) (Model, tea.Cmd)` | New in `panel_queue.go`. All overlay key routing. Returns model + cmd (no third "handled" bool — when overlay is open, all keys are absorbed). |
| `renderUpNext(now domain.NowPlaying, panel playlistsPanel, queue []domain.Track, rows, width int) string` | **Signature change.** Was `(now, panel, rows, width)`. Adds `queue` as the 3rd param. Render order: header, queue rows (`★` prefix), then playlist-tail rows. Queue rows take row-budget priority. |
| `enqueueFocusedMainRow(m Model) Model` | New helper in `panel_main.go`. Reads the focused Main panel row's track via `mainPaneRows(m)[m.main.cursor]`, validates PID, calls `m.queue.Add(track)`. On empty PID, sets `m.lastError = ErrNoPersistentID` (a new sentinel) and returns. Does **not** dispatch any Cmd. |
| `app.ErrNoPersistentID` | New sentinel error in `internal/app/queue.go`: `errors.New("track has no ID — can't queue")`. Surfaced via the existing `m.lastError` mechanism (3s auto-dissolve). The other transient notices in the spec (`couldn't play queued track`, `couldn't resume playlist`) reuse the same `m.lastError` path via the existing `playTrackResultMsg` / `playPlaylistMsg` error handlers. **No new transient-notice infrastructure is added.** |
| CLI `queue` subcommand | A new `case "queue"` branch in `cli.Run` that prints a fixed help block to `stdout` and returns `0`. No `music.Client` call. The help block points at the TUI keys (`a`, `Q`). |

No new message types are added. The handoff handler returns existing Cmds (`playTrack`, `playPlaylist`) whose result messages already exist and already funnel errors into `m.lastError`.

---

## Phase 1 — Bootstrap

### Task 1: Create feature branch and verify baseline

**No files modified.**

- [ ] **Step 1: Create the feature branch from main**

Run:
```bash
git checkout main
git checkout -b feature/queue-management
```

DO NOT run `git pull`. Local `main` carries the design spec commit (78388fd) which has not yet been pushed; pulling would either rebase the spec out (if upstream rewound) or no-op. Either way it's noise.

- [ ] **Step 2: Confirm spec and plan are present and tree is clean**

Run:
```bash
ls docs/superpowers/specs/2026-05-13-queue-management-design.md
ls docs/superpowers/plans/2026-05-13-queue-management.md
git status
git log -3 --format='%h %s'
```

Expected: both files present; tree clean (or only `.claude/`, `.superpowers/`, and the local `goove` binary untracked). Recent log shows the spec commit `78388fd docs: spec for queue management UI`.

- [ ] **Step 3: Confirm baseline tests pass**

Run:
```bash
make test
```

Expected: all packages PASS. If anything fails on a clean checkout, stop and surface to the user — the fix is not in this plan.

---

## Phase 2 — Queue state

### Task 2: QueueState type and methods (TDD)

**Files:**
- Create: `internal/app/queue.go`
- Create: `internal/app/queue_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/queue_test.go` with:

```go
package app

import (
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func tk(id, title string) domain.Track {
	return domain.Track{Title: title, PersistentID: id}
}

func TestQueueAddAppendsToTail(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	if q.Len() != 2 {
		t.Fatalf("Len = %d; want 2", q.Len())
	}
	if q.Items[0].PersistentID != "a" || q.Items[1].PersistentID != "b" {
		t.Errorf("order = %v; want [a b]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID})
	}
}

func TestQueueAddAllowsDuplicates(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("a", "A"))
	if q.Len() != 2 {
		t.Fatalf("duplicate not allowed: Len = %d; want 2", q.Len())
	}
}

func TestQueueRemoveAtMiddle(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	q.RemoveAt(1)
	if q.Len() != 2 {
		t.Fatalf("Len = %d; want 2", q.Len())
	}
	if q.Items[0].PersistentID != "a" || q.Items[1].PersistentID != "c" {
		t.Errorf("order = %v; want [a c]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID})
	}
}

func TestQueueRemoveAtOutOfRangeIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.RemoveAt(-1)
	q.RemoveAt(5)
	if q.Len() != 1 {
		t.Errorf("Len = %d; want 1", q.Len())
	}
}

func TestQueueMoveUpSwapsWithPrevious(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	got := q.MoveUp(2)
	if got != 1 {
		t.Errorf("MoveUp(2) returned %d; want 1", got)
	}
	if q.Items[1].PersistentID != "c" || q.Items[2].PersistentID != "b" {
		t.Errorf("after MoveUp(2): %v; want [a c b]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID, q.Items[2].PersistentID})
	}
}

func TestQueueMoveUpAtHeadIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got := q.MoveUp(0)
	if got != 0 {
		t.Errorf("MoveUp(0) returned %d; want 0", got)
	}
	if q.Items[0].PersistentID != "a" {
		t.Errorf("order changed: head = %s; want a", q.Items[0].PersistentID)
	}
}

func TestQueueMoveDownSwapsWithNext(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	got := q.MoveDown(0)
	if got != 1 {
		t.Errorf("MoveDown(0) returned %d; want 1", got)
	}
	if q.Items[0].PersistentID != "b" || q.Items[1].PersistentID != "a" {
		t.Errorf("after MoveDown(0): %v; want [b a c]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID, q.Items[2].PersistentID})
	}
}

func TestQueueMoveDownAtTailIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got := q.MoveDown(1)
	if got != 1 {
		t.Errorf("MoveDown(last) returned %d; want 1", got)
	}
	if q.Items[1].PersistentID != "b" {
		t.Errorf("order changed: tail = %s; want b", q.Items[1].PersistentID)
	}
}

func TestQueuePopHeadEmptyReturnsFalse(t *testing.T) {
	var q QueueState
	_, ok := q.PopHead()
	if ok {
		t.Errorf("PopHead on empty queue returned ok=true; want false")
	}
}

func TestQueuePopHeadReturnsAndShrinks(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got, ok := q.PopHead()
	if !ok {
		t.Fatal("PopHead returned ok=false; want true")
	}
	if got.PersistentID != "a" {
		t.Errorf("popped %s; want a", got.PersistentID)
	}
	if q.Len() != 1 || q.Items[0].PersistentID != "b" {
		t.Errorf("after pop: items = %v; want [b]", q.Items)
	}
}

func TestQueueClearEmpties(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Clear()
	if q.Len() != 0 {
		t.Errorf("after Clear: Len = %d; want 0", q.Len())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestQueue -v
```

Expected: build error — `undefined: QueueState`.

- [ ] **Step 3: Implement queue.go**

Create `internal/app/queue.go` with:

```go
package app

import (
	"errors"

	"github.com/themoderngeek/goove/internal/domain"
)

// ErrNoPersistentID is set on m.lastError when the user tries to enqueue
// a track whose PersistentID is empty (parser edge case — e.g. a synthetic
// search result without an ID). Surfaced in the bottom error strip.
var ErrNoPersistentID = errors.New("track has no ID — can't queue")

// QueueState is the goove-owned interactive queue. FIFO; head is Items[0].
// Mutated via methods so the model can rely on bounds checking. Direct
// access to Items is supported for reads (render path).
type QueueState struct {
	Items []domain.Track
}

// Add appends t to the queue tail. Duplicates allowed.
func (q *QueueState) Add(t domain.Track) {
	q.Items = append(q.Items, t)
}

// RemoveAt deletes the element at index i. No-op when i is out of range.
func (q *QueueState) RemoveAt(i int) {
	if i < 0 || i >= len(q.Items) {
		return
	}
	q.Items = append(q.Items[:i], q.Items[i+1:]...)
}

// MoveUp swaps Items[i] with Items[i-1]. Returns the new index of the
// moved item: i-1 on success, i if at head or out of range.
func (q *QueueState) MoveUp(i int) int {
	if i <= 0 || i >= len(q.Items) {
		return i
	}
	q.Items[i-1], q.Items[i] = q.Items[i], q.Items[i-1]
	return i - 1
}

// MoveDown swaps Items[i] with Items[i+1]. Returns the new index of the
// moved item: i+1 on success, i if at tail or out of range.
func (q *QueueState) MoveDown(i int) int {
	if i < 0 || i >= len(q.Items)-1 {
		return i
	}
	q.Items[i+1], q.Items[i] = q.Items[i], q.Items[i+1]
	return i + 1
}

// PopHead removes and returns Items[0]. (zero, false) on empty queue.
func (q *QueueState) PopHead() (domain.Track, bool) {
	if len(q.Items) == 0 {
		return domain.Track{}, false
	}
	head := q.Items[0]
	q.Items = q.Items[1:]
	return head, true
}

// Clear empties the queue.
func (q *QueueState) Clear() {
	q.Items = nil
}

// Len returns the number of queued items. Value receiver — read-only.
func (q QueueState) Len() int {
	return len(q.Items)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestQueue -v
```

Expected: all 11 `TestQueue*` tests PASS.

- [ ] **Step 5: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/queue.go internal/app/queue_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): add QueueState type with Add/Remove/Move/Pop/Clear/Len

In-memory FIFO queue for the upcoming queue management feature.
Exposes a method API for mutations (bounds-checked) and direct Items
access for the render path. Includes ErrNoPersistentID sentinel for
the 'a' key path's empty-PID case.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Expected: full test suite PASS; clean commit.

---

## Phase 3 — Model wiring and `a` enqueue

### Task 3: Embed queue state on Model

**Files:**
- Modify: `internal/app/model.go`

This task only adds the new fields; no behaviour changes yet. The TDD test for `a` lives in Task 4 — this task ensures the model compiles with the new fields.

- [ ] **Step 1: Add the new fields to the Model struct**

Open `internal/app/model.go`. Locate the `Model` struct (around line 97). Add new fields after the existing layout state. The full updated `Model` struct should read:

```go
// Model holds the entire goove TUI state.
type Model struct {
	client music.Client

	state       AppState
	lastVolume  int
	lastError   error
	lastErrorAt time.Time

	// Permission failure shows a blocking screen; the value is sticky.
	permissionDenied bool

	// Latest terminal size for layout decisions.
	width  int
	height int

	art      artState
	renderer art.Renderer // nil ⇒ chafa unavailable; track-change detection skips fetches

	// New layout state (Phase 1).
	focus     focusKind
	playlists playlistsPanel
	search    searchPanel
	output    outputPanel
	main      mainPanel

	// Queue management state (spec 2026-05-13-queue-management-design.md).
	queue          QueueState
	resume         ResumeContext
	lastTrackPID   string // PID seen on previous status tick; "" at launch
	lastPlaylist   string // CurrentPlaylistName on previous tick
	lastTrackIdx   int    // 1-based index of last-seen track in lastPlaylist; 0 if unknown
	overlay        overlayState
	clearPrompt    bool   // true while awaiting y/n after `c` in overlay
	pendingJumpPID string // one-shot: overlay Enter sets this; handoff handler clears on match
}
```

- [ ] **Step 2: Add placeholder type definitions so the model compiles**

The fields above reference `ResumeContext` and `overlayState`, which don't exist yet (created in Tasks 5 and 8 respectively). Add temporary placeholder declarations at the bottom of `model.go` so the package compiles:

```go
// Placeholders — replaced by full definitions in later tasks.
// ResumeContext is defined in resume.go (Task 5).
// overlayState is defined in panel_queue.go (Task 8).
type ResumeContext struct {
	PlaylistName string
	NextIndex    int
}

type overlayState struct {
	open   bool
	cursor int
}
```

These two types will be **moved** (not redefined) to their permanent homes in Tasks 5 and 8. Delete them from `model.go` at that point.

- [ ] **Step 3: Verify the package compiles and existing tests still pass**

Run:
```bash
go build ./...
go test ./internal/app/ -v
```

Expected: build PASS, all existing app tests PASS. No new tests introduced in this task.

- [ ] **Step 4: Commit**

Run:
```bash
git add internal/app/model.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): embed queue/resume/overlay state on Model

Placeholder ResumeContext and overlayState types are defined inline;
they move to resume.go and panel_queue.go in later tasks.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 4: Global `a` key — enqueue focused Main row

**Files:**
- Modify: `internal/app/panel_main.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/update_test.go`:

```go
func TestKeyAEnqueuesFocusedMainTrack(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	// Focus Main with one search-result row that has a PID.
	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "Hotel California", PersistentID: "HC1"}}
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 1 {
		t.Fatalf("queue.Len = %d; want 1", got.queue.Len())
	}
	if got.queue.Items[0].PersistentID != "HC1" {
		t.Errorf("enqueued PID = %q; want HC1", got.queue.Items[0].PersistentID)
	}
}

func TestKeyAOnEmptyPIDRefusesAndSetsError(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = []domain.Track{{Title: "NoPID"}} // empty PID
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (refused)", got.queue.Len())
	}
	if got.lastError == nil || !errors.Is(got.lastError, ErrNoPersistentID) {
		t.Errorf("lastError = %v; want ErrNoPersistentID", got.lastError)
	}
}

func TestKeyAOnNonMainFocusIsNoOp(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusPlaylists // not Main

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (no-op when focus != Main)", got.queue.Len())
	}
}

func TestKeyAOnMainWithEmptyRowsIsNoOp(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	m.focus = focusMain
	m.main.mode = mainPaneSearchResults
	m.main.searchResults = nil
	m.main.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0", got.queue.Len())
	}
	if got.lastError != nil {
		t.Errorf("lastError = %v; want nil (no-op silently)", got.lastError)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestKeyA -v
```

Expected: tests FAIL because `a` is not yet routed.

- [ ] **Step 3: Add the enqueue helper to panel_main.go**

Append to `internal/app/panel_main.go`:

```go
// enqueueFocusedMainRow appends the focused Main panel row's track to
// m.queue, or sets m.lastError if the track has no persistent ID. No-op
// when there are no rows or the cursor is out of range. Does not dispatch
// any Cmd.
func enqueueFocusedMainRow(m Model) Model {
	rows := mainPaneRows(m)
	if len(rows) == 0 || m.main.cursor < 0 || m.main.cursor >= len(rows) {
		return m
	}
	t := rows[m.main.cursor]
	if t.PersistentID == "" {
		m.lastError = ErrNoPersistentID
		m.lastErrorAt = time.Now()
		return m
	}
	m.queue.Add(t)
	return m
}
```

Add `"time"` to the imports if not already present.

- [ ] **Step 4: Route the `a` key in update.go**

Open `internal/app/update.go`. In `handleKey`, add a new case in the global key switch (after the existing `case "o":` and before `case "/":`), and arrange to also return a `clearErrorAfter` Cmd when an error was set:

```go
		case "a":
			if m.focus != focusMain {
				return m, nil
			}
			before := m.lastError
			m = enqueueFocusedMainRow(m)
			if m.lastError != nil && m.lastError != before {
				return m, clearErrorAfter()
			}
			return m, nil
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestKeyA -v
```

Expected: all four `TestKeyA*` tests PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/panel_main.go internal/app/update.go internal/app/update_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): wire global 'a' to enqueue focused Main row

'a' on a search-result or playlist-track row appends to m.queue.
On empty PersistentID, sets m.lastError to ErrNoPersistentID
(auto-dissolves via the existing clearErrorAfter mechanism).
No-op on non-Main focus or empty rows.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — Handoff state machine

### Task 5: ResumeContext + handleQueueHandoff handler (TDD)

**Files:**
- Create: `internal/app/resume.go`
- Create: `internal/app/resume_test.go`
- Modify: `internal/app/model.go` (remove the `ResumeContext` placeholder)

- [ ] **Step 1: Write the failing tests**

Create `internal/app/resume_test.go` with:

```go
package app

import (
	"context"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func handoffModel(t *testing.T) Model {
	t.Helper()
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
		{Title: "Misty", PersistentID: "MM"},
	})
	c.SetLibraryTracks([]domain.Track{
		{Title: "Hotel California", PersistentID: "HC"},
		{Title: "Wonderwall", PersistentID: "WW"},
	})
	m := New(c, nil)
	// Pre-cache the playlist so indexOfPID works in handler tests.
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
		{Title: "Misty", PersistentID: "MM"},
	}
	return m
}

func TestHandoffNoTrackChange(t *testing.T) {
	m := handoffModel(t)
	m.lastTrackPID = "ST"
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "ST"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil on no-change")
	}
	if got.queue.Len() != 0 || got.resume.PlaylistName != "" {
		t.Errorf("state mutated on no-change: %+v", got)
	}
}

func TestHandoffEmptyQueueNoResumeIsNoDispatch(t *testing.T) {
	m := handoffModel(t)
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	_, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil")
	}
}

func TestHandoffEmptyQueueWithResumeDispatchesPlayPlaylist(t *testing.T) {
	m := handoffModel(t)
	m.resume = ResumeContext{PlaylistName: "LZ", NextIndex: 3}
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "HC"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayPlaylist Cmd")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume not cleared: %+v", got.resume)
	}
	// Invoke the cmd to confirm it produces a playPlaylistMsg and that the
	// fake client recorded the call with the right args.
	out := cmd()
	if _, ok := out.(playPlaylistMsg); !ok {
		t.Errorf("cmd result = %T; want playPlaylistMsg", out)
	}
	rec := m.client.(*fake.Client).PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].Name != "LZ" || rec[0].FromIdx != 3 {
		t.Errorf("PlayPlaylist record = %v; want [{LZ 3}]", rec)
	}
}

func TestHandoffInterceptCapturesResumeAndPopsHead(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	// Previous tick: track ST (index 2) playing in LZ.
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3}", got.resume)
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (head popped)", got.queue.Len())
	}
	out := cmd()
	if _, ok := out.(playTrackResultMsg); !ok {
		t.Errorf("cmd result = %T; want playTrackResultMsg", out)
	}
	rec := m.client.(*fake.Client).PlayTrackRecord()
	if len(rec) != 1 || rec[0].PersistentID != "HC" {
		t.Errorf("PlayTrack record = %v; want [{HC}]", rec)
	}
}

func TestHandoffInterceptDoesNotOverwriteExistingResume(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	m.resume = ResumeContext{PlaylistName: "Other", NextIndex: 5}
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "Other" || got.resume.NextIndex != 5 {
		t.Errorf("resume overwritten: %+v; want {Other 5}", got.resume)
	}
}

func TestHandoffInterceptWithEmptyPrevPlaylistLeavesResumeEmpty(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "", 0)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured with no valid prev context: %+v", got.resume)
	}
}

func TestHandoffNewPIDMatchesQueueHeadPopsOnly(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "HC"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil (pop-on-match, no dispatch)")
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0", got.queue.Len())
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured on pop-on-match: %+v", got.resume)
	}
}

func TestHandoffNewPIDMatchesPendingJumpClearsFlag(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "A", PersistentID: "A1"})
	m.pendingJumpPID = "C1"
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "C1"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil (pending jump match)")
	}
	if got.pendingJumpPID != "" {
		t.Errorf("pendingJumpPID = %q; want cleared", got.pendingJumpPID)
	}
	if got.queue.Len() != 1 {
		t.Errorf("queue.Len = %d; want 1 (not popped on jump)", got.queue.Len())
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3} (captured on jump)", got.resume)
	}
}

func TestHandoffFirstTickIsNoOpWithEmptyPrev(t *testing.T) {
	m := handoffModel(t)
	// Empty queue, empty prev — typical launch state.
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "ST"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "", "", 0)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil on first tick with empty queue")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured on first tick: %+v", got.resume)
	}
}

func TestIndexOfPIDFindsTrack(t *testing.T) {
	m := handoffModel(t)
	if got := m.indexOfPID("ST", "LZ"); got != 2 {
		t.Errorf("indexOfPID(ST, LZ) = %d; want 2", got)
	}
	if got := m.indexOfPID("BD", "LZ"); got != 1 {
		t.Errorf("indexOfPID(BD, LZ) = %d; want 1", got)
	}
}

func TestIndexOfPIDReturnsZeroOnMiss(t *testing.T) {
	m := handoffModel(t)
	if got := m.indexOfPID("XX", "LZ"); got != 0 {
		t.Errorf("indexOfPID(miss) = %d; want 0", got)
	}
	if got := m.indexOfPID("ST", "Unknown"); got != 0 {
		t.Errorf("indexOfPID(unknown playlist) = %d; want 0", got)
	}
	if got := m.indexOfPID("", "LZ"); got != 0 {
		t.Errorf("indexOfPID(empty pid) = %d; want 0", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run "TestHandoff|TestIndexOfPID" -v
```

Expected: build error — `m.handleQueueHandoff` and `m.indexOfPID` undefined.

- [ ] **Step 3: Create resume.go**

Create `internal/app/resume.go` with:

```go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
)

// ResumeContext records the playlist and 1-based next-track index that
// handleQueueHandoff should hand control back to when the queue drains.
// Zero value = no resume target (drain ends in silence).
type ResumeContext struct {
	PlaylistName string
	NextIndex    int
}

// handleQueueHandoff runs once per status tick (after the existing
// artwork / playlist-prefetch logic in handleStatus). It compares the
// current tick's now-playing PID against lastTrackPID to detect track
// changes, then routes to one of four branches:
//
//   - No track change: return immediately, no mutation.
//   - newPID == pendingJumpPID: clear the flag, capture resume context
//     if valid and unset, return (don't pop the queue, don't dispatch).
//   - newPID == queue.Items[0].PersistentID: a previous tick's
//     intercept has landed; pop the head and return (no dispatch).
//   - Otherwise with non-empty queue: capture resume context if valid
//     and unset, dispatch PlayTrack(head) and pop. (Intercept.)
//   - Otherwise with empty queue and resume set: dispatch
//     PlayPlaylist(resume) and clear resume. (Drain.)
//
// prevPID / prevPlaylist / prevIdx are the *previous* tick's cached
// values, captured by the caller before refreshing m.lastTrackPID /
// m.lastPlaylist / m.lastTrackIdx for the next round.
func (m Model) handleQueueHandoff(now domain.NowPlaying, prevPID, prevPlaylist string, prevIdx int) (Model, tea.Cmd) {
	newPID := now.Track.PersistentID
	if newPID == prevPID {
		return m, nil
	}

	// Pending-jump match: overlay Enter dispatched a PlayTrack; recognise
	// our own transition without popping or re-intercepting.
	if newPID != "" && newPID == m.pendingJumpPID {
		m.pendingJumpPID = ""
		if m.resume.PlaylistName == "" && prevPlaylist != "" && prevIdx > 0 {
			m.resume = ResumeContext{PlaylistName: prevPlaylist, NextIndex: prevIdx + 1}
		}
		return m, nil
	}

	if m.queue.Len() == 0 {
		// Drain: hand back to interrupted playlist if we have one.
		if m.resume.PlaylistName != "" {
			cmd := playPlaylist(m.client, m.resume.PlaylistName, m.resume.NextIndex)
			m.resume = ResumeContext{}
			return m, cmd
		}
		return m, nil
	}

	head := m.queue.Items[0]
	if newPID != "" && newPID == head.PersistentID {
		// Our previous intercept has landed; pop and move on.
		m.queue.PopHead()
		return m, nil
	}

	// Intercept: capture resume (if empty and previous context valid)
	// and dispatch PlayTrack on the head.
	if m.resume.PlaylistName == "" && prevPlaylist != "" && prevIdx > 0 {
		m.resume = ResumeContext{PlaylistName: prevPlaylist, NextIndex: prevIdx + 1}
	}
	m.queue.PopHead()
	return m, playTrack(m.client, head.PersistentID)
}

// indexOfPID returns the 1-based index of pid inside the cached track
// list for playlistName, or 0 if pid is empty, the playlist isn't
// cached, or the PID isn't in the list. The 1-based convention matches
// PlayPlaylist's fromTrackIndex argument.
func (m Model) indexOfPID(pid, playlistName string) int {
	if pid == "" || playlistName == "" {
		return 0
	}
	tracks, ok := m.playlists.tracksByName[playlistName]
	if !ok {
		return 0
	}
	for i, t := range tracks {
		if t.PersistentID == pid {
			return i + 1
		}
	}
	return 0
}
```

- [ ] **Step 4: Remove the ResumeContext placeholder from model.go**

Open `internal/app/model.go` and delete the inline `ResumeContext` declaration added in Task 3 step 2 (leave the `overlayState` placeholder — it moves in Task 8). After the deletion, the bottom of `model.go` should have only:

```go
// Placeholder — replaced by full definition in panel_queue.go (Task 8).
type overlayState struct {
	open   bool
	cursor int
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run "TestHandoff|TestIndexOfPID" -v
```

Expected: all 10 tests PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/resume.go internal/app/resume_test.go internal/app/model.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): add handleQueueHandoff state machine + indexOfPID helper

The state machine routes each status tick by comparing the previous
tick's PID against the current Now.Track.PersistentID:
  - no change            → no-op
  - matches pending jump → clear flag, capture resume if appropriate
  - matches queue head   → pop (our previous intercept landed)
  - else queue non-empty → capture resume, PlayTrack(head), pop
  - else resume set      → PlayPlaylist(resume), clear

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 6: Wire handleQueueHandoff into handleStatus

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/update_test.go`:

```go
func TestHandleStatusInvokesHandoffAndRefreshesLastFields(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
	})
	c.SetLibraryTracks([]domain.Track{
		{Title: "Hotel California", PersistentID: "HC"},
	})
	m := New(c, nil)
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
	}
	// Prime previous-tick state to ST in LZ at index 2.
	m.lastTrackPID = "ST"
	m.lastPlaylist = "LZ"
	m.lastTrackIdx = 2
	// Queue Hotel California so the next tick triggers intercept.
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})

	// Status tick: Music.app moved to the next playlist track (BD again
	// won't do — pick a different PID so it's a real change).
	now := domain.NowPlaying{
		Track:               domain.Track{Title: "End", PersistentID: "END"},
		Volume:              50,
		IsPlaying:           true,
		CurrentPlaylistName: "LZ",
	}

	updated, cmd := m.Update(statusMsg{now: now})
	got := updated.(Model)

	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (intercept popped head)", got.queue.Len())
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3}", got.resume)
	}
	if got.lastTrackPID != "END" {
		t.Errorf("lastTrackPID = %q; want END", got.lastTrackPID)
	}
	if got.lastPlaylist != "LZ" {
		t.Errorf("lastPlaylist = %q; want LZ", got.lastPlaylist)
	}
	// END isn't in the cached LZ tracks, so lastTrackIdx is 0.
	if got.lastTrackIdx != 0 {
		t.Errorf("lastTrackIdx = %d; want 0", got.lastTrackIdx)
	}
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	// The intercept Cmd should produce a PlayTrack call when invoked.
	_ = cmd() // may be a tea.Batch wrapper — call it to flush
	rec := c.PlayTrackRecord()
	if len(rec) != 1 || rec[0].PersistentID != "HC" {
		t.Errorf("PlayTrack record = %v; want [{HC}]", rec)
	}
}

func TestHandleStatusRefreshesLastFieldsOnNormalTick(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{{Title: "Stairway", PersistentID: "ST"}})
	m := New(c, nil)
	m.playlists.tracksByName["LZ"] = []domain.Track{{Title: "Stairway", PersistentID: "ST"}}

	now := domain.NowPlaying{
		Track:               domain.Track{Title: "Stairway", PersistentID: "ST"},
		Volume:              50,
		IsPlaying:           true,
		CurrentPlaylistName: "LZ",
	}
	updated, _ := m.Update(statusMsg{now: now})
	got := updated.(Model)

	if got.lastTrackPID != "ST" {
		t.Errorf("lastTrackPID = %q; want ST", got.lastTrackPID)
	}
	if got.lastPlaylist != "LZ" {
		t.Errorf("lastPlaylist = %q; want LZ", got.lastPlaylist)
	}
	if got.lastTrackIdx != 1 {
		t.Errorf("lastTrackIdx = %d; want 1", got.lastTrackIdx)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestHandleStatus -v
```

Expected: tests FAIL — `handleQueueHandoff` is defined but not yet invoked from `handleStatus`; `last*` fields don't get refreshed.

- [ ] **Step 3: Add playPlaylistMsg error handler in Update**

The resume drain dispatches `playPlaylist(...)` which produces `playPlaylistMsg`. Today, no `case playPlaylistMsg:` exists in `Update`, so errors are silently dropped. Add a handler so spec §3.7's "couldn't resume playlist" notice surfaces in the bottom error strip.

Open `internal/app/update.go`. In the top-level `Update` method's switch statement, add a new case alongside `playTrackResultMsg` (just before it is fine):

```go
	case playPlaylistMsg:
		if msg.err != nil {
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		return m, nil
```

This fixes a pre-existing gap (the manual "play a playlist" path also benefited from silent error swallowing). The queue feature's drain dispatch makes the gap matter more.

- [ ] **Step 4: Wire the handler into handleStatus**

Open `internal/app/update.go`. Find the `handleStatus` method. At the end (after the existing artwork + queue-prefetch logic, just before the `switch len(cmds)`), insert:

```go
	// Queue handoff: compare current tick's PID against the cached
	// previous-tick state to decide intercept / drain / pop / no-op.
	// The handler returns a Cmd (or nil) which is batched with anything
	// the artwork / prefetch logic added.
	prevPID := m.lastTrackPID
	prevPlaylist := m.lastPlaylist
	prevIdx := m.lastTrackIdx
	var handoffCmd tea.Cmd
	m, handoffCmd = m.handleQueueHandoff(msg.now, prevPID, prevPlaylist, prevIdx)
	if handoffCmd != nil {
		cmds = append(cmds, handoffCmd)
	}
	// Refresh the cache for the next tick.
	m.lastTrackPID = msg.now.Track.PersistentID
	m.lastPlaylist = msg.now.CurrentPlaylistName
	m.lastTrackIdx = m.indexOfPID(msg.now.Track.PersistentID, msg.now.CurrentPlaylistName)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestHandleStatus -v
```

Expected: both `TestHandleStatus*` tests PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/update.go internal/app/update_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): invoke handleQueueHandoff each status tick + handle playPlaylistMsg errors

handleStatus captures previous-tick PID/playlist/index, runs the
handoff state machine, batches its Cmd with artwork+prefetch cmds,
then refreshes the cached last* fields for the next round.

Also adds a playPlaylistMsg case in Update so resume-drain errors
(and the existing manual play-playlist path) surface in the bottom
error strip via lastError, instead of being silently dropped.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Up Next teaser merge

### Task 7: renderUpNext renders queue rows above tail rows (TDD)

**Files:**
- Modify: `internal/app/panel_now_playing.go`
- Modify: `internal/app/panel_now_playing_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/panel_now_playing_test.go`:

```go
func TestRenderUpNextShowsQueueRowsAboveTail(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "T1"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["P"] = []domain.Track{
		{Title: "T1", PersistentID: "T1"},
		{Title: "T2", PersistentID: "T2"},
		{Title: "T3", PersistentID: "T3"},
		{Title: "T4", PersistentID: "T4"},
	}
	queue := []domain.Track{
		{Title: "HC", Artist: "Eagles", PersistentID: "HC"},
		{Title: "WW", Artist: "Oasis", PersistentID: "WW"},
	}
	got := renderUpNext(now, p, queue, 5, 40)
	if !strings.Contains(got, "★") {
		t.Errorf("expected ★ prefix for queue rows; got %q", got)
	}
	if !strings.Contains(got, "HC") || !strings.Contains(got, "WW") {
		t.Errorf("queue rows missing: %q", got)
	}
	if !strings.Contains(got, "T2") || !strings.Contains(got, "T3") || !strings.Contains(got, "T4") {
		t.Errorf("playlist tail missing (rows = 5, 2 used by queue, 3 left for tail): %q", got)
	}
}

func TestRenderUpNextQueueRowsTakeRowBudgetPriority(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "T1"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["P"] = []domain.Track{
		{Title: "T1", PersistentID: "T1"},
		{Title: "T2", PersistentID: "T2"},
		{Title: "T3", PersistentID: "T3"},
	}
	queue := []domain.Track{
		{Title: "Q1", Artist: "A", PersistentID: "Q1"},
		{Title: "Q2", Artist: "A", PersistentID: "Q2"},
		{Title: "Q3", Artist: "A", PersistentID: "Q3"},
		{Title: "Q4", Artist: "A", PersistentID: "Q4"},
		{Title: "Q5", Artist: "A", PersistentID: "Q5"},
		{Title: "Q6", Artist: "A", PersistentID: "Q6"},
		{Title: "Q7", Artist: "A", PersistentID: "Q7"},
	}
	got := renderUpNext(now, p, queue, 5, 40)
	// All 5 row budget consumed by queue → no tail rows.
	for _, q := range []string{"Q1", "Q2", "Q3", "Q4", "Q5"} {
		if !strings.Contains(got, q) {
			t.Errorf("missing queue row %s in %q", q, got)
		}
	}
	for _, x := range []string{"T2", "T3"} {
		if strings.Contains(got, x) {
			t.Errorf("unexpected tail row %s in %q (queue should consume all rows)", x, got)
		}
	}
}

func TestRenderUpNextShuffleWithQueueRendersStarRowsThenPlaceholder(t *testing.T) {
	now := domain.NowPlaying{
		ShuffleEnabled:      true,
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "T1"},
	}
	p := newPlaylistsPanel()
	queue := []domain.Track{
		{Title: "HC", Artist: "Eagles", PersistentID: "HC"},
	}
	got := renderUpNext(now, p, queue, 5, 40)
	if !strings.Contains(got, "★") || !strings.Contains(got, "HC") {
		t.Errorf("expected queue row visible under shuffle; got %q", got)
	}
	if !strings.Contains(got, "shuffling") {
		t.Errorf("expected shuffle placeholder below queue rows; got %q", got)
	}
}

func TestRenderUpNextEmptyQueueUnchanged(t *testing.T) {
	now := domain.NowPlaying{
		CurrentPlaylistName: "P",
		Track:               domain.Track{PersistentID: "T1"},
	}
	p := newPlaylistsPanel()
	p.tracksByName["P"] = []domain.Track{
		{Title: "T1", PersistentID: "T1"},
		{Title: "T2", PersistentID: "T2"},
	}
	got := renderUpNext(now, p, nil, 5, 40)
	if !strings.Contains(got, "T2") {
		t.Errorf("playlist tail missing when queue empty: %q", got)
	}
	if strings.Contains(got, "★") {
		t.Errorf("unexpected ★ prefix when queue is empty: %q", got)
	}
}
```

- [ ] **Step 2: Update the existing renderUpNext test signature**

The existing `TestRenderUpNext*` tests in `panel_now_playing_test.go` call `renderUpNext(now, p, rows, width)`. They will fail to compile after the signature change. Update each existing call to inject `nil` as the queue argument:

Run a search-and-replace across `internal/app/panel_now_playing_test.go`:
- `renderUpNext(now, p, 0, 30)` → `renderUpNext(now, p, nil, 0, 30)`
- `renderUpNext(now, p, 5, 0)` → `renderUpNext(now, p, nil, 5, 0)`
- `renderUpNext(now, p, 5, 30)` → `renderUpNext(now, p, nil, 5, 30)`

Inspect the file with `grep -n 'renderUpNext(' internal/app/panel_now_playing_test.go` and edit each call site to insert `nil` as the third argument.

- [ ] **Step 3: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run "TestRenderUpNext" -v
```

Expected: build error — `renderUpNext` defined with 4 params, called with 5.

- [ ] **Step 4: Update renderUpNext, upNextBody, and the call site**

Open `internal/app/panel_now_playing.go`. Replace the existing `renderUpNext` and `upNextBody` functions with:

```go
// renderUpNext renders the Up Next block for the now-playing panel:
// a "─ Up Next ─" header followed by queue rows (★ prefix) and/or
// upcoming playlist tail rows and/or a placeholder line. Returns ""
// when rows < 1 or width < 1 (caller falls back to centered layout).
//
// Row budget allocation (when total rows is positive):
//   - queue rows take priority and consume up to len(queue) rows
//   - any remaining rows go to placeholder/tail per upNextBody
//
// queue is the goove-owned queue (Model.queue.Items); pass nil/empty
// for the legacy read-only behaviour.
func renderUpNext(now domain.NowPlaying, panel playlistsPanel, queue []domain.Track, rows, width int) string {
	if rows < 1 || width < 1 {
		return ""
	}
	headerLabel := "─ Up Next "
	var header string
	if utf8.RuneCountInString(headerLabel) >= width {
		header = subtitleStyle.Render(truncate(headerLabel, width))
	} else {
		pad := strings.Repeat("─", width-utf8.RuneCountInString(headerLabel))
		header = subtitleStyle.Render(headerLabel + pad)
	}

	var sb strings.Builder
	queueRows := len(queue)
	if queueRows > rows {
		queueRows = rows
	}
	for i := 0; i < queueRows; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		row := fmt.Sprintf("★ %s — %s", queue[i].Title, queue[i].Artist)
		sb.WriteString(truncate(row, width))
	}

	remaining := rows - queueRows
	if remaining > 0 {
		body := upNextBody(now, panel, remaining, width)
		if body != "" {
			if queueRows > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(body)
		}
	}

	if sb.Len() == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, sb.String())
}
```

The `upNextBody` function is unchanged but its contract changes slightly: callers now pass the *remaining* row budget after queue rows are drawn. No edits needed inside `upNextBody`.

Update the call site in `renderConnectedCardOnly`. Locate:
```go
upNext := renderUpNext(s.Now, panel, queueRows, colWidth)
```

The current `renderConnectedCardOnly` signature is `(s Connected, art string, width int, panel playlistsPanel)`. It doesn't currently have access to `m.queue.Items`. Change the signature to accept the queue slice as a 5th parameter:

```go
func renderConnectedCardOnly(s Connected, art string, width int, panel playlistsPanel, queue []domain.Track) string {
```

Update the body's `renderUpNext` call to:
```go
upNext := renderUpNext(s.Now, panel, queue, queueRows, colWidth)
```

Then update the caller `renderNowPlayingPanel`. Locate:
```go
body = renderConnectedCardOnly(s, art, width, m.playlists)
```
Change to:
```go
body = renderConnectedCardOnly(s, art, width, m.playlists, m.queue.Items)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run "TestRenderUpNext|TestNowPlaying" -v
```

Expected: all `TestRenderUpNext*` and `TestNowPlaying*` tests PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/panel_now_playing.go internal/app/panel_now_playing_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): render queue rows above playlist tail in Up Next teaser

renderUpNext gains a queue param. Queue rows render with a ★ prefix
and take row-budget priority — any remaining rows fall through to
the existing upNextBody (placeholder or playlist tail). Empty queue
preserves today's render exactly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6 — Overlay rendering

### Task 8: Overlay panel render + view.go wiring (TDD)

**Files:**
- Create: `internal/app/panel_queue.go`
- Create: `internal/app/panel_queue_test.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/model.go` (remove the `overlayState` placeholder)

- [ ] **Step 1: Write the failing tests**

Create `internal/app/panel_queue_test.go` with:

```go
package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestRenderOverlayEmptyState(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Queue [0]") {
		t.Errorf("missing 'Queue [0]' header: %q", got)
	}
	if !strings.Contains(got, "queue is empty") {
		t.Errorf("missing empty-state hint: %q", got)
	}
}

func TestRenderOverlayWithItemsAndCursor(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.overlay.cursor = 1
	m.queue.Add(domain.Track{Title: "HC", Artist: "Eagles", PersistentID: "HC"})
	m.queue.Add(domain.Track{Title: "WW", Artist: "Oasis", PersistentID: "WW"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Queue [2]") {
		t.Errorf("missing 'Queue [2]' header: %q", got)
	}
	if !strings.Contains(got, "HC") || !strings.Contains(got, "WW") {
		t.Errorf("missing queue items: %q", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("missing cursor glyph: %q", got)
	}
	// Find which row carries the cursor — should be the 2nd (cursor=1).
	lines := strings.Split(got, "\n")
	cursorLine := ""
	for _, ln := range lines {
		if strings.Contains(ln, "▶") {
			cursorLine = ln
			break
		}
	}
	if !strings.Contains(cursorLine, "WW") {
		t.Errorf("cursor on wrong row; cursor line = %q", cursorLine)
	}
}

func TestRenderOverlayResumeFooterShowsPlaylistContext(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	m.resume = ResumeContext{PlaylistName: "LZ", NextIndex: 4}
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{}, {}, {}, {}, {}, {}, {}, {}, // 8 tracks
	}
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "then resumes") {
		t.Errorf("missing resume footer: %q", got)
	}
	if !strings.Contains(got, "LZ") {
		t.Errorf("missing resume playlist name: %q", got)
	}
	if !strings.Contains(got, "track 4 of 8") {
		t.Errorf("missing 'track 4 of 8' resume detail: %q", got)
	}
}

func TestRenderOverlayResumeFooterShowsStopWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "then stops") {
		t.Errorf("missing 'then stops' footer when resume empty: %q", got)
	}
}

func TestRenderOverlayClearPromptOverridesHelpRow(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.clearPrompt = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "Clear queue?") {
		t.Errorf("missing clear prompt: %q", got)
	}
	if strings.Contains(got, "j/k nav") {
		t.Errorf("regular help row still visible while clear prompt active: %q", got)
	}
}

func TestRenderOverlayHelpRowVisibleByDefault(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got := renderOverlay(m, 80, 24)
	if !strings.Contains(got, "j/k") {
		t.Errorf("missing help row: %q", got)
	}
}

func TestViewRendersOverlayWhenOpen(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T"}, Volume: 50}}
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})
	got := m.View()
	if !strings.Contains(got, "Queue [1]") {
		t.Errorf("View did not render overlay when open: %q", got)
	}
	// The Now Playing panel should NOT render when the overlay is open.
	if strings.Contains(got, "Now Playing") {
		t.Errorf("View should suppress normal panels when overlay open: %q", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run "TestRenderOverlay|TestViewRendersOverlay" -v
```

Expected: build error — `renderOverlay` undefined.

- [ ] **Step 3: Create panel_queue.go**

Create `internal/app/panel_queue.go` with:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// overlayState carries the queue overlay's view-layer state. Lives on
// Model. Zero value = closed; cursor 0 = head row.
type overlayState struct {
	open   bool
	cursor int
}

var (
	overlayCursorStyle  = lipgloss.NewStyle().Reverse(true)
	overlayWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF5F"))
)

// renderOverlay renders the full-area queue overlay. Layout (top to
// bottom):
//
//  1. Header row: "Queue [N]" left, close hint right.
//  2. Divider.
//  3. Body: queue rows (cursor row prefixed ▶ and reversed) or empty
//     state when len == 0.
//  4. Divider.
//  5. Resume footer: "─ then resumes ─\n<playlist> · track N of M"
//     or "─ then stops ─" when resume is empty.
//  6. Help row (or clear prompt when m.clearPrompt).
func renderOverlay(m Model, width, height int) string {
	_ = height // height drives no truncation in V1 — body grows; if it overflows the terminal scrolls
	if width < 20 {
		width = 20
	}

	count := m.queue.Len()
	header := fmt.Sprintf("Queue [%d]", count)
	closeHint := subtitleStyle.Render("Q/esc to close")
	headerRow := padBetween(header, closeHint, width)

	var bodyLines []string
	if count == 0 {
		bodyLines = append(bodyLines, subtitleStyle.Render("(queue is empty — press a on a track to add)"))
	} else {
		for i, t := range m.queue.Items {
			marker := "  "
			if i == m.overlay.cursor {
				marker = "▶ "
			}
			row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
			row = truncate(row, width)
			if i == m.overlay.cursor {
				row = overlayCursorStyle.Render(row)
			}
			bodyLines = append(bodyLines, row)
		}
	}
	body := strings.Join(bodyLines, "\n")

	var resumeFooter string
	if m.resume.PlaylistName != "" {
		total := len(m.playlists.tracksByName[m.resume.PlaylistName])
		resumeFooter = subtitleStyle.Render("─ then resumes ─") + "\n" +
			subtitleStyle.Render(fmt.Sprintf("%s · track %d of %d", m.resume.PlaylistName, m.resume.NextIndex, total))
	} else {
		resumeFooter = subtitleStyle.Render("─ then stops ─")
	}

	var helpRow string
	if m.clearPrompt {
		helpRow = overlayWarningStyle.Render("Clear queue? press y to confirm, any other key to cancel")
	} else {
		helpRow = subtitleStyle.Render("j/k nav · enter play now · x remove · K/J reorder · c clear")
	}

	out := lipgloss.JoinVertical(lipgloss.Left,
		headerRow,
		strings.Repeat("─", width),
		body,
		strings.Repeat("─", width),
		resumeFooter,
		"",
		helpRow,
	)
	return out
}

// padBetween joins left + right with a run of spaces so the combined
// string fills exactly width columns. If left+right already exceeds
// width, returns left+" "+right (no truncation in V1).
func padBetween(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
```

- [ ] **Step 4: Remove the overlayState placeholder from model.go**

Open `internal/app/model.go` and delete the `overlayState` placeholder block at the bottom. The file should no longer contain that type definition.

- [ ] **Step 5: Wire view.go to render the overlay when open**

Open `internal/app/view.go`. In `View()`, add a branch before `renderLayout(m)`:

```go
func (m Model) View() string {
	if m.permissionDenied {
		return renderPermissionDenied()
	}
	if m.width > 0 && m.width < compactThreshold {
		return renderTooNarrow()
	}
	if m.height > 0 && m.height < minLayoutHeight {
		return renderTooNarrow()
	}
	if m.overlay.open {
		width := m.width
		if width <= 0 {
			width = 100
		}
		height := m.height
		if height <= 0 {
			height = 30
		}
		return renderOverlay(m, width, height)
	}
	return renderLayout(m)
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run "TestRenderOverlay|TestViewRendersOverlay" -v
```

Expected: all 7 tests PASS.

- [ ] **Step 7: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/panel_queue.go internal/app/panel_queue_test.go internal/app/view.go internal/app/model.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): render queue overlay (read-only) when m.overlay.open

Adds renderOverlay with header (count + close hint), body (cursor
row reversed), resume footer (then-resumes / then-stops), and help
row (or clear-confirm prompt). View routes to it before the normal
four-panel layout when m.overlay.open. Keys come in the next task.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7 — Overlay keys and `Q` open

### Task 9: updateOverlay key handler (TDD)

**Files:**
- Modify: `internal/app/panel_queue.go`
- Modify: `internal/app/panel_queue_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/panel_queue_test.go`:

```go
import (
	tea "github.com/charmbracelet/bubbletea"
)
```
(Update the existing import block — `tea` is needed for KeyMsg.)

Then append the test functions:

```go
func openWithItems(items ...domain.Track) Model {
	m := newTestModel()
	m.overlay.open = true
	for _, t := range items {
		m.queue.Add(t)
	}
	return m
}

func TestOverlayJKMovesCursor(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.overlay.cursor != 1 {
		t.Errorf("after j: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.overlay.cursor != 2 {
		t.Errorf("clamped at last: cursor = %d; want 2", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.overlay.cursor != 1 {
		t.Errorf("after k: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.overlay.cursor != 0 {
		t.Errorf("clamped at head: cursor = %d; want 0", m.overlay.cursor)
	}
}

func TestOverlayXRemovesAtCursor(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m.overlay.cursor = 1
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.queue.Len() != 2 {
		t.Errorf("after x: Len = %d; want 2", m.queue.Len())
	}
	if m.queue.Items[1].PersistentID != "C" {
		t.Errorf("after x at 1: items[1] = %s; want C", m.queue.Items[1].PersistentID)
	}
}

func TestOverlayXClampsCursorAfterTailRemoval(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
	)
	m.overlay.cursor = 1
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.overlay.cursor != 0 {
		t.Errorf("cursor not clamped after tail removal: cursor = %d; want 0", m.overlay.cursor)
	}
}

func TestOverlayKJReordersAndCursorFollows(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
		domain.Track{Title: "C", PersistentID: "C"},
	)
	m.overlay.cursor = 2
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.queue.Items[1].PersistentID != "C" {
		t.Errorf("after K: items[1] = %s; want C", m.queue.Items[1].PersistentID)
	}
	if m.overlay.cursor != 1 {
		t.Errorf("cursor not following K: cursor = %d; want 1", m.overlay.cursor)
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.queue.Items[2].PersistentID != "C" {
		t.Errorf("after J: items[2] = %s; want C", m.queue.Items[2].PersistentID)
	}
	if m.overlay.cursor != 2 {
		t.Errorf("cursor not following J: cursor = %d; want 2", m.overlay.cursor)
	}
}

func TestOverlayCThenYClears(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
		domain.Track{Title: "B", PersistentID: "B"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !m.clearPrompt {
		t.Fatal("clearPrompt not set after c")
	}
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.queue.Len() != 0 {
		t.Errorf("queue not cleared after y: Len = %d", m.queue.Len())
	}
	if m.clearPrompt {
		t.Error("clearPrompt not reset after y")
	}
}

func TestOverlayCThenOtherKeyCancels(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A"},
	)
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.queue.Len() != 1 {
		t.Errorf("queue cleared on non-y after c: Len = %d; want 1", m.queue.Len())
	}
	if m.clearPrompt {
		t.Error("clearPrompt not reset after cancel key")
	}
}

func TestOverlayEscClosesAndResetsClearPrompt(t *testing.T) {
	m := openWithItems(domain.Track{Title: "A", PersistentID: "A"})
	m.clearPrompt = true
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.overlay.open {
		t.Error("overlay.open not cleared on Esc")
	}
	if m.clearPrompt {
		t.Error("clearPrompt not cleared on Esc")
	}
}

func TestOverlayQClosesAsWell(t *testing.T) {
	m := openWithItems(domain.Track{Title: "A", PersistentID: "A"})
	m, _ = updateOverlay(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	if m.overlay.open {
		t.Error("overlay.open not cleared on Q")
	}
}

func TestOverlayEnterDispatchesPlayTrackAndRemovesAndSetsPending(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A1"},
		domain.Track{Title: "B", PersistentID: "B1"},
		domain.Track{Title: "C", PersistentID: "C1"},
	)
	m.overlay.cursor = 2
	got, cmd := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a PlayTrack cmd")
	}
	if got.pendingJumpPID != "C1" {
		t.Errorf("pendingJumpPID = %q; want C1", got.pendingJumpPID)
	}
	if got.queue.Len() != 2 {
		t.Errorf("queue.Len = %d; want 2 (selected removed)", got.queue.Len())
	}
	if got.queue.Items[0].PersistentID != "A1" || got.queue.Items[1].PersistentID != "B1" {
		t.Errorf("remaining queue = %v; want [A1 B1]", got.queue.Items)
	}
	// Verify cmd routes to fake's PlayTrack — note the fake errs because
	// C1 isn't in its library; the cmd still completes.
	_ = cmd()
}

func TestOverlayEnterClampsCursorAfterRemoval(t *testing.T) {
	m := openWithItems(
		domain.Track{Title: "A", PersistentID: "A1"},
		domain.Track{Title: "B", PersistentID: "B1"},
	)
	m.overlay.cursor = 1
	got, _ := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got.overlay.cursor != 0 {
		t.Errorf("cursor not clamped: cursor = %d; want 0", got.overlay.cursor)
	}
}

func TestOverlayEnterOnEmptyQueueIsNoOp(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got, cmd := updateOverlay(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("cmd != nil on Enter with empty queue")
	}
	if got.pendingJumpPID != "" {
		t.Errorf("pendingJumpPID set on empty queue: %q", got.pendingJumpPID)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestOverlay -v
```

Expected: build error — `updateOverlay` undefined.

- [ ] **Step 3: Add updateOverlay to panel_queue.go**

Append to `internal/app/panel_queue.go`:

```go
import (
	tea "github.com/charmbracelet/bubbletea"
)
```
(Merge into the existing import block at the top.)

Then add at the bottom of the file:

```go
// updateOverlay handles keystrokes while the queue overlay is open. The
// overlay is fully modal — all keys land here, and globals (space, n,
// p, +/-, q-to-quit, Tab, digits, /, o) do not fire. When the user
// presses Esc or Q the overlay closes; thereafter the normal global
// key map applies.
func updateOverlay(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	// Clear-prompt mode absorbs the next keypress: y confirms, anything
	// else cancels. Esc/Q still close (handled below) — they implicitly
	// also cancel the prompt because we reset clearPrompt on close.
	if m.clearPrompt {
		s := msg.String()
		if s != "esc" && s != "Q" {
			if s == "y" {
				m.queue.Clear()
			}
			m.clearPrompt = false
			return m, nil
		}
	}

	switch msg.String() {
	case "esc", "Q":
		m.overlay.open = false
		m.clearPrompt = false
		return m, nil

	case "j", "down":
		if m.overlay.cursor < m.queue.Len()-1 {
			m.overlay.cursor++
		}
		return m, nil

	case "k", "up":
		if m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		return m, nil

	case "x":
		m.queue.RemoveAt(m.overlay.cursor)
		if m.overlay.cursor >= m.queue.Len() && m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		return m, nil

	case "K":
		m.overlay.cursor = m.queue.MoveUp(m.overlay.cursor)
		return m, nil

	case "J":
		m.overlay.cursor = m.queue.MoveDown(m.overlay.cursor)
		return m, nil

	case "c":
		m.clearPrompt = true
		return m, nil

	case "enter":
		if m.queue.Len() == 0 {
			return m, nil
		}
		if m.overlay.cursor < 0 || m.overlay.cursor >= m.queue.Len() {
			return m, nil
		}
		item := m.queue.Items[m.overlay.cursor]
		m.queue.RemoveAt(m.overlay.cursor)
		if m.overlay.cursor >= m.queue.Len() && m.overlay.cursor > 0 {
			m.overlay.cursor--
		}
		m.pendingJumpPID = item.PersistentID
		return m, playTrack(m.client, item.PersistentID)
	}
	return m, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestOverlay -v
```

Expected: all 11 `TestOverlay*` tests PASS.

- [ ] **Step 5: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/panel_queue.go internal/app/panel_queue_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): overlay key handler (j/k cursor, x/K/J edit, c+y clear, Enter jump)

updateOverlay handles all overlay keystrokes. Enter dispatches
PlayTrack on the cursor item, removes it from queue, and sets
pendingJumpPID so the handoff handler recognises the transition
on the next status tick without re-intercepting.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 10: Global `Q` opens overlay; overlay-open suppresses all other keys

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/update_test.go`:

```go
func TestKeyQOpensOverlay(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	got := updated.(Model)
	if !got.overlay.open {
		t.Fatal("overlay.open false after Q")
	}
	if got.overlay.cursor != 0 {
		t.Errorf("cursor = %d; want 0 on fresh open", got.overlay.cursor)
	}
}

func TestKeyQuitSuppressedWhileOverlayOpen(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Errorf("q while overlay open returned cmd %v; want nil (suppressed quit)", cmd)
	}
}

func TestKeysRouteToOverlayWhenOpen(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	m.queue.Add(domain.Track{Title: "A", PersistentID: "A1"})
	m.queue.Add(domain.Track{Title: "B", PersistentID: "B1"})

	// 'j' should move overlay cursor, not switch focus.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.overlay.cursor != 1 {
		t.Errorf("overlay cursor = %d; want 1", got.overlay.cursor)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run "TestKeyQ|TestKeysRouteToOverlay" -v
```

Expected: FAIL — `Q` not yet wired; overlay-open does not yet intercept keys.

- [ ] **Step 3: Wire overlay-open intercept and `Q` global**

Open `internal/app/update.go`. In `handleKey`, at the very top (after the `permissionDenied` guard but before the focus-routed switch), insert:

```go
	if m.overlay.open {
		mm, cmd := updateOverlay(m, msg)
		return mm, cmd
	}
```

Then in the global-key switch, add a `case "Q":` (next to the existing `case "/":` etc.):

```go
		case "Q":
			m.overlay.open = true
			m.overlay.cursor = 0
			return m, nil
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run "TestKeyQ|TestKeysRouteToOverlay" -v
```

Expected: all three tests PASS.

- [ ] **Step 5: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/update.go internal/app/update_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): global Q opens queue overlay; overlay-open suppresses all other keys

handleKey routes to updateOverlay before any focus / global handling
when m.overlay.open. q is implicitly suppressed (no quit dispatch).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 8 — Global `n` integration

### Task 11: Modify global `n` to play queue head when non-empty (TDD)

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/update_test.go`:

```go
func TestKeyNWithEmptyQueueCallsNext(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.NextCalls != 1 {
		t.Errorf("Next calls = %d; want 1 (empty queue path)", c.NextCalls)
	}
	if c.PlayTrackCalls != 0 {
		t.Errorf("PlayTrack calls = %d; want 0 (empty queue)", c.PlayTrackCalls)
	}
}

func TestKeyNWithQueueCallsPlayTrackAndPops(t *testing.T) {
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{
		{Title: "HC", PersistentID: "HC"},
	})
	c.SetTrack(domain.Track{Title: "T"}, 200, 10, true)
	m := New(c, nil)
	tmp, _ := m.Update(statusMsg{now: domain.NowPlaying{Track: domain.Track{Title: "T"}, IsPlaying: true}})
	m = tmp.(Model)
	m.queue.Add(domain.Track{Title: "HC", PersistentID: "HC"})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a Cmd")
	}
	cmd()
	if c.NextCalls != 0 {
		t.Errorf("Next calls = %d; want 0 (queue path should not call Next)", c.NextCalls)
	}
	if c.PlayTrackCalls != 1 {
		t.Errorf("PlayTrack calls = %d; want 1", c.PlayTrackCalls)
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (head popped)", got.queue.Len())
	}
	if got.pendingJumpPID != "HC" {
		t.Errorf("pendingJumpPID = %q; want HC (n is a user-driven jump)", got.pendingJumpPID)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestKeyN -v
```

Expected: `TestKeyNWithQueueCallsPlayTrackAndPops` FAILS — `n` still calls `Next` unconditionally.

- [ ] **Step 3: Modify the `n` case in handleKey**

Open `internal/app/update.go`. Find the existing `case "n":` in the global key switch and replace with:

```go
		case "n":
			if m.queue.Len() > 0 {
				head, _ := m.queue.PopHead()
				m.pendingJumpPID = head.PersistentID
				return m, playTrack(m.client, head.PersistentID)
			}
			return m, doAction(m.client.Next)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestKeyN -v
```

Expected: both `TestKeyN*` tests PASS. The existing `TestNKeyTriggersNext` test (from the original suite) also continues to pass because its model has an empty queue.

- [ ] **Step 5: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/update.go internal/app/update_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): global 'n' plays queue head when non-empty

Pops the queue head, sets pendingJumpPID so the handoff handler
recognises the transition on the next status tick without
re-intercepting, and dispatches PlayTrack. Empty-queue path
unchanged (still calls client.Next).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 9 — Hints bar

### Task 12: Update hints bar and suppress when overlay open (TDD)

**Files:**
- Modify: `internal/app/hints.go`
- Modify: `internal/app/hints_test.go`
- Modify: `internal/app/view.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/hints_test.go`:

```go
func TestHintBarIncludesAAndQ(t *testing.T) {
	m := newTestModel()
	got := renderHintBar(m)
	if !strings.Contains(got, "a:queue") {
		t.Errorf("hints missing 'a:queue': %q", got)
	}
	if !strings.Contains(got, "Q:queue-view") {
		t.Errorf("hints missing 'Q:queue-view': %q", got)
	}
}

func TestHintBarEmptyWhenOverlayOpen(t *testing.T) {
	m := newTestModel()
	m.overlay.open = true
	got := renderHintBar(m)
	if got != "" {
		t.Errorf("hints not empty while overlay open: %q", got)
	}
}
```

If `hints_test.go` doesn't already import `strings`, add it.

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run TestHintBar -v
```

Expected: tests FAIL — `globalKeysHint` doesn't yet contain `a:queue` / `Q:queue-view`; overlay-open suppression not yet wired.

- [ ] **Step 3: Update hints.go**

Replace `internal/app/hints.go` with:

```go
package app

import "strings"

// globalKeysHint is shown on every hint bar.
const globalKeysHint = "space:play/pause  n:next  p:prev  +/-:vol  a:queue  Q:queue-view  q:quit"

// renderHintBar returns the bottom-of-screen hint string. Always includes
// the global keys; appends panel-scoped hints based on m.focus. Style is
// applied by view.go (footerStyle.Render). Returns empty when the queue
// overlay is open — the overlay renders its own help row.
func renderHintBar(m Model) string {
	if m.overlay.open {
		return ""
	}
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
```

- [ ] **Step 4: Confirm renderLayout still renders the hint bar correctly when it's empty**

Open `internal/app/view.go`. The current `renderLayout` does:
```go
hint := footerStyle.Render(renderHintBar(m))
```
When `renderHintBar` returns `""`, this becomes an empty string. The subsequent `lipgloss.JoinVertical(lipgloss.Left, now, body, hint)` then renders an empty hint row — which is fine (a blank line). No code change needed in view.go for this task; the test verifies the hint string itself.

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run TestHintBar -v
```

Expected: both `TestHintBar*` tests PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/app/hints.go internal/app/hints_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(app): add a:queue and Q:queue-view to hint bar; suppress when overlay open

The overlay carries its own help row at the bottom of its frame.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 10 — CLI stub

### Task 13: Add `goove queue` subcommand stub (TDD)

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/cli_test.go`:

```go
func TestQueueSubcommandPrintsHelpAndExitsZero(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d; want 0", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "queue") {
		t.Errorf("stdout missing 'queue': %q", out)
	}
	if !strings.Contains(out, "TUI") {
		t.Errorf("stdout missing TUI reference: %q", out)
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "Q") {
		t.Errorf("stdout missing key references (a / Q): %q", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr non-empty: %q", stderr.String())
	}
}
```

Check the existing imports in `cli_test.go` — it likely already has `bytes`, `strings`, and `fake`. If not, add them.

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
go test ./internal/cli/ -run TestQueueSubcommand -v
```

Expected: FAIL — `queue` is currently "unknown command" → exit 1, prints usage to stderr.

- [ ] **Step 3: Add the queue subcommand to cli.go**

Open `internal/cli/cli.go`. Locate the switch in `Run`. Add a new case **before** the `default`:

```go
		case "queue":
			fmt.Fprint(stdout, queueHelpText)
			return 0
```

At the bottom of `cli.go`, add the help text constant:

```go
const queueHelpText = `goove queue — interactive queue management

Queue management lives inside the TUI; there are no CLI verbs in V1
because the queue is in-memory only and does not survive between
goove invocations.

To use the queue:
  1. Run 'goove' to launch the TUI.
  2. Focus a track row in the main panel (a playlist track list or
     search results).
  3. Press 'a' to add the focused track to the queue tail.
  4. Press 'Q' to open the queue overlay.
     Inside the overlay:
       j / k       move cursor
       enter       play this track now (others stay queued)
       x           remove track at cursor
       K / J       reorder up / down
       c then y    clear the queue (any other key cancels)
       esc / Q     close the overlay

Queued tracks play after the currently-playing track ends. Once the
queue drains, playback resumes the playlist that was interrupted.
`
```

- [ ] **Step 4: Update the usageText to mention `queue`**

In `internal/cli/cli.go`, locate the existing `usageText` constant. Add a line in the Usage section so users discover the subcommand:

```
  goove queue                 Print queue help (queue management is TUI-only)
```

Insert it between the existing `goove playlists ...` line and `goove help`. (The plan author confirmed via reading the file that those neighbours exist.)

- [ ] **Step 5: Run the test to verify it passes**

Run:
```bash
go test ./internal/cli/ -run TestQueueSubcommand -v
```

Expected: PASS.

- [ ] **Step 6: Run the full suite and commit**

Run:
```bash
make test
git add internal/cli/cli.go internal/cli/cli_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
feat(cli): add 'goove queue' stub subcommand

Prints a help block pointing at the TUI keys (a, Q) and explains
why there are no CLI verbs in V1 (in-memory only). Real CLI surface
waits on persistence or IPC, per the spec.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 11 — Integration test

### Task 14: Live handoff integration test

**Files:**
- Create: `internal/music/applescript/client_handoff_integration_test.go`

This test runs only with the `integration_handoff` build tag — **NOT** the existing `integration` tag. The existing `client_integration_test.go` file is read-only by policy (see its header comment: "The tests are read-only by design: they only call IsRunning and Status. They do not press play, change volume, or skip tracks."). The handoff test materially violates that policy (it calls `PlayPlaylist`, `PlayTrack`, and `Pause`), so it gets its own file and its own build tag to keep the read-only policy intact for the default integration run.

Requires Music.app on macOS, automation permission granted, and a "Liked Songs" playlist with at least 3 tracks. Not part of `make ci` or `make test-integration`. Run with: `go test -tags=integration_handoff ./internal/music/applescript/`.

- [ ] **Step 1: Create the new file with the correct build tag and header**

Create `internal/music/applescript/client_handoff_integration_test.go` with:

```go
//go:build darwin && integration_handoff

package applescript

import (
	"context"
	"testing"
	"time"
)

// These tests MUTATE Music.app state — they start playback, switch
// tracks, and pause. Kept in a separate file with a separate build tag
// (integration_handoff) so the default integration run stays read-only.
//
// Run with:
//   go test -tags=integration_handoff ./internal/music/applescript/
//
// Prerequisites:
//   - macOS with Music.app installed.
//   - Automation permission granted to the terminal binary you run
//     `go test` from (System Settings → Privacy & Security → Automation).
//   - A playlist named "Liked Songs" with at least 3 tracks.

func TestIntegrationQueueHandoffOverridesPlaylistNaturalNext(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	c := NewDefault()

	if err := c.Launch(ctx); err != nil {
		t.Skipf("can't launch Music.app: %v", err)
	}

	// Pick a known playlist. Liked Songs is the conventional default in
	// goove's docs. Bail out clean if absent.
	playlists, err := c.Playlists(ctx)
	if err != nil {
		t.Fatalf("Playlists: %v", err)
	}
	const playlistName = "Liked Songs"
	found := false
	for _, p := range playlists {
		if p.Name == playlistName {
			found = true
			break
		}
	}
	if !found {
		t.Skipf("playlist %q not present in this library", playlistName)
	}

	tracks, err := c.PlaylistTracks(ctx, playlistName)
	if err != nil {
		t.Fatalf("PlaylistTracks: %v", err)
	}
	if len(tracks) < 3 {
		t.Skipf("playlist %q needs >= 3 tracks; has %d", playlistName, len(tracks))
	}

	// Play from track 1. We'll queue tracks[2] so it doesn't accidentally
	// match the natural-next (tracks[1]).
	if err := c.PlayPlaylist(ctx, playlistName, 1); err != nil {
		t.Fatalf("PlayPlaylist: %v", err)
	}

	// Wait for Status to confirm playback has started.
	startPID := ""
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		np, err := c.Status(ctx)
		if err == nil && np.Track.PersistentID != "" {
			startPID = np.Track.PersistentID
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if startPID == "" {
		t.Fatal("Status never reported a non-empty PID after PlayPlaylist")
	}

	queuedPID := tracks[2].PersistentID
	if queuedPID == "" || queuedPID == startPID {
		t.Fatalf("can't pick a distinct queue target: queuedPID=%q startPID=%q", queuedPID, startPID)
	}

	// Simulate the handoff: when the current track changes, call
	// PlayTrack(queuedPID). The "natural next" (tracks[1]) may play
	// for up to ~1s before our override lands — that's the spec's
	// accepted glitch.
	lastPID := startPID
	overrideDeadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(overrideDeadline) {
		np, err := c.Status(ctx)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if np.Track.PersistentID != lastPID {
			// Track change observed — dispatch the queued PID.
			if err := c.PlayTrack(ctx, queuedPID); err != nil {
				t.Fatalf("PlayTrack(queued): %v", err)
			}
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Now wait (up to 10s) for Status to reflect the queued track.
	confirmDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(confirmDeadline) {
		np, err := c.Status(ctx)
		if err == nil && np.Track.PersistentID == queuedPID {
			// Success — queued track is now playing.
			if err := c.Pause(ctx); err != nil {
				t.Logf("Pause (cleanup) returned: %v", err)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("queued track never became the playing track within 10s of override")
}
```

The file is brand new (created in this Task's Step 1), so its imports start as `context`, `testing`, `time` — already present in the header from Step 1. No additional imports are needed.

- [ ] **Step 3: Run the handoff integration test**

Be prepared for the test to take over Music.app playback briefly.

Run:
```bash
go test -tags=integration_handoff ./internal/music/applescript/ -v -run TestIntegrationQueueHandoff
```

Expected: PASS within ~45s, or `SKIP` if Music.app is unavailable / "Liked Songs" doesn't exist / has fewer than 3 tracks. If it FAILS with a timeout, the test environment likely has a Music.app track that takes longer than the test allowance to finish naturally — increase the `overrideDeadline` budget or temporarily place a short test track at the front of "Liked Songs". Don't merge an intermittent failure into CI.

- [ ] **Step 4: Confirm the existing read-only integration suite is unaffected**

Run:
```bash
make test-integration
```

Expected: PASS — the existing `integration`-tagged tests still run read-only because the handoff test is gated by a different build tag (`integration_handoff`) and won't be compiled in.

- [ ] **Step 5: Commit**

Run:
```bash
git add internal/music/applescript/client_handoff_integration_test.go
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
test(integration): handoff e2e under separate integration_handoff tag

End-to-end test that plays Liked Songs from track 1, waits for the
natural transition to track 2, then calls PlayTrack on track 3 and
asserts Status eventually reflects the override. Tolerates Music.app's
~1s observable lag.

Lives in its own file with build tag integration_handoff (NOT
integration) because it mutates Music.app state — the default
integration suite stays read-only by policy.

Skips if Music.app is unavailable or the test playlist isn't present.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 12 — Manual smoke and CI

### Task 15: Manual smoke checklist + CI green

**No files modified.**

- [ ] **Step 1: Build the binary**

Run:
```bash
make build
ls -la goove
```

Expected: `goove` binary built without warnings.

- [ ] **Step 2: Run the smoke checklist manually**

Launch `./goove` (Music.app should be running). Walk through:

1. **Up Next teaser shows playlist tail** when no queue is set (sanity — existing behaviour preserved).
2. **Press `a`** on a search result row (focus Main, press `/`, type a query, Enter, then `a` on a row).
   - Confirm the Up Next teaser shows a `★ <title> — <artist>` row.
3. **Add 2–3 more tracks** with `a`. Teaser shows them in order, prioritising over playlist tail.
4. **Press `Q`** — the overlay should open, showing the queue with cursor on row 1.
5. **`j/k`** moves the cursor; **`x`** removes one; **`K/J`** reorders; **`c` then `y`** clears (then re-add a couple of tracks for the rest).
6. **`Enter`** on a queue row — that track plays now; remaining queue items still play next.
7. **`Esc` / `Q`** closes the overlay.
8. **Wait for the current track to end** — verify the queue head intercepts and plays. Note the ~0.5–1s of natural-next playing before our override (the accepted glitch).
9. **After the queue drains**, verify the playlist resumes from a sensible track (the one that would have come after the original intercept point).
10. **Press `q`** to quit (should NOT quit while overlay is open; close first, then quit).
11. **Test transient-error path**: `a` on a row with no PID is hard to trigger from the UI normally; this is best left to unit tests.

Write down anything that doesn't match expectation. If found, fix and re-test before continuing.

- [ ] **Step 3: Run the full CI pipeline locally**

Run:
```bash
make ci
```

Expected: all CI steps PASS (fmt-check, vet, vuln, lint, test-race, build). If `lint` complains about anything — likely about unused helpers if a step was over-eager — fix and re-run.

- [ ] **Step 4: Final commit if anything was tweaked during smoke testing**

If you tweaked any files during smoke testing, stage and commit them:

```bash
git status
git add <files>
git -c commit.gpgsign=false commit -m "$(cat <<'EOF'
fix(app): smoke-test follow-ups

<one-line summary per fix>

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If nothing changed, skip this step.

- [ ] **Step 5: Push and open a PR**

Per goove's convention, don't push without user approval. Surface the branch status to the user:

```bash
git log --oneline main..HEAD
git status
```

Report:
- Number of commits on the branch
- All CI passing locally
- Spec link: `docs/superpowers/specs/2026-05-13-queue-management-design.md`
- Plan link: `docs/superpowers/plans/2026-05-13-queue-management.md`

Ask the user whether to `git push -u origin feature/queue-management` and open a PR (and if so, what PR title / body summary they want).

---

## Done

All tasks complete. The queue management feature is implemented, tested (unit + integration), and CI-green. Remaining future work is documented in spec §2 "Out of scope" — pick those up as separate spec → plan cycles.
