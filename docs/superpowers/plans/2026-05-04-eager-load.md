# goove Eager-Load Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eager-load the Playlists panel, the Output (AirPlay devices) panel, and the first playlist's tracks on startup so the TUI is populated from frame zero instead of waiting for the user to tab to each panel.

**Architecture:** Two surgical edits inside `internal/app/`. (1) `New(...)` initialises `playlists.loading = true` and `output.loading = true` so frame zero shows `loading…` rather than empty placeholders. (2) `Init()` adds `fetchPlaylists` and `fetchDevices` to its existing `tea.Batch`. (3) The `playlistsMsg` handler in `update.go`, after populating items, additionally sets `main.selectedPlaylist` to the first item's name and fires `fetchPlaylistTracks` for it — but only when `selectedPlaylist` is empty (no-clobber guard). The lazy on-focus loaders stay untouched and become a natural retry path on failure.

**Tech Stack:** Go 1.24 (stdlib + bubbletea). Spec: `docs/superpowers/specs/2026-05-04-eager-load-design.md`.

---

## File Structure

```
goove/
└── internal/
    └── app/
        ├── model.go                  # T2 (loading flags) + T3 (Init batch)
        ├── update.go                 # T4 (playlistsMsg handler)
        ├── panel_playlists_test.go   # T2 (loading flag test) + T3 (Init test) + T4 (prefetch tests)
        └── panel_output_test.go      # T2 (loading flag test)
```

No new files. No changes outside `internal/app/`.

## Naming and signature contract

| Symbol | Definition |
|---|---|
| `playlistsPanel.loading` | Existing `bool` — set to `true` in `New(...)` so first frame shows `loading…`. |
| `outputPanel.loading` | Existing `bool` — set to `true` in `New(...)` for the same reason. |
| `Model.Init() tea.Cmd` | Existing — extends its `tea.Batch` with `fetchPlaylists(m.client)` and `fetchDevices(m.client)`. |
| `playlistsMsg` arm of `Model.Update` | Existing handler in `update.go:87-101` — gains a final guarded block that sets `m.main.selectedPlaylist` to `m.playlists.items[0].Name` and returns `fetchPlaylistTracks(m.client, name)` when `len(items) > 0 && m.main.selectedPlaylist == ""`. |

No new types, no new messages, no new exported symbols.

---

## Phase 1 — Bootstrap

### Task 1: Create feature branch and verify clean starting state

**No files modified.**

- [ ] **Step 1: Create the feature branch from main**

Run:
```bash
git checkout main
git checkout -b feature/eager-load
```

DO NOT run `git pull`. Local main may carry the eager-load spec commit which has not yet been pushed.

- [ ] **Step 2: Confirm spec/plan are present and tree is clean**

Run:
```bash
ls docs/superpowers/specs/2026-05-04-eager-load-design.md
ls docs/superpowers/plans/2026-05-04-eager-load.md
git status
git log -3 --format='%h %s'
```

Expected: both files present; tree clean (or only `.claude/` and `main` binary untracked).

- [ ] **Step 3: Confirm baseline tests pass**

Run:
```bash
go test ./...
```

Expected: all packages pass. If anything fails on a clean checkout, stop and surface to the user — the fix is not in this plan.

---

## Phase 2 — Loading flags in `New(...)`

This phase makes the panels render `loading…` from frame zero by initialising
the `loading` flags. Pure state-shape change; no Cmds involved.

### Task 2: Set `playlists.loading` and `output.loading` to true in `New(...)`

**Files:**
- Test: `internal/app/panel_playlists_test.go` — ADD one test
- Test: `internal/app/panel_output_test.go` — ADD one test
- Modify: `internal/app/model.go:126-138` (the `New` function body)

- [ ] **Step 1: Write the failing test for the playlists loading flag**

Append to `internal/app/panel_playlists_test.go`:

```go
func TestNewInitialisesPlaylistsPanelLoading(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	if !m.playlists.loading {
		t.Error("expected playlists.loading = true after New so first frame shows 'loading…' instead of an empty panel")
	}
}
```

- [ ] **Step 2: Write the failing test for the output loading flag**

Append to `internal/app/panel_output_test.go`:

```go
func TestNewInitialisesOutputPanelLoading(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	if !m.output.loading {
		t.Error("expected output.loading = true after New so first frame shows 'loading…' instead of an empty panel")
	}
}
```

- [ ] **Step 3: Run both tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run 'TestNewInitialisesPlaylistsPanelLoading|TestNewInitialisesOutputPanelLoading' -v
```

Expected: both FAIL with `expected … loading = true`. (`loading` defaults to false on the zero-value structs.)

- [ ] **Step 4: Edit `New` in `internal/app/model.go` to set both flags**

Open `internal/app/model.go` and change the `New` function (currently at
lines 126-138) so the `playlists` and `output` literals include `loading: true`:

```go
func New(client music.Client, renderer art.Renderer) Model {
	return Model{
		client:     client,
		renderer:   renderer,
		state:      Disconnected{},
		lastVolume: 50,
		playlists: playlistsPanel{
			tracksByName:   make(map[string][]domain.Track),
			fetchingFor:    make(map[string]bool),
			trackErrByName: make(map[string]error),
			loading:        true,
		},
		output: outputPanel{
			loading: true,
		},
	}
}
```

Note: only the `playlists` literal already existed — we add the `loading: true` field. The `output` field was previously defaulted (zero-value `outputPanel{}`); we now spell it out so we can set `loading`.

- [ ] **Step 5: Run the two new tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run 'TestNewInitialisesPlaylistsPanelLoading|TestNewInitialisesOutputPanelLoading' -v
```

Expected: both PASS.

- [ ] **Step 6: Run the full app package tests to confirm no regressions**

Run:
```bash
go test ./internal/app/
```

Expected: PASS. No existing tests should break — they all explicitly set the loading flag when they need a particular state.

- [ ] **Step 7: Commit**

```bash
git add internal/app/model.go internal/app/panel_playlists_test.go internal/app/panel_output_test.go
git commit -m "$(cat <<'EOF'
app: initialise playlists/output loading flags in New

Frame zero of the TUI now shows 'loading…' in the Playlists and Output
panels rather than the empty-state placeholders. Prep for the eager-load
fetches that follow — they need the loading state visible during the
short window before the first message arrives.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — Eager fetches in `Init()`

This phase wires the actual list fetches into startup so the panels populate
on their own without needing focus.

### Task 3: Extend `Init()`'s `tea.Batch` with `fetchPlaylists` and `fetchDevices`

**Files:**
- Test: `internal/app/panel_playlists_test.go` — ADD one test
- Modify: `internal/app/model.go:141-147` (the `Init` function body)

- [ ] **Step 1: Write the failing test that `Init` produces playlist + device fetches**

Append to `internal/app/panel_playlists_test.go`:

```go
func TestInitFetchesPlaylistsAndDevicesEagerly(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	c.SetDevices([]domain.AudioDevice{{Name: "MacBook"}})
	m := New(c, nil)

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init returned nil Cmd")
	}
	raw := initCmd()
	batch, ok := raw.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init Cmd produced %T; want tea.BatchMsg", raw)
	}

	// Run each child Cmd with a short per-Cmd deadline so the two scheduled
	// ticks (statusInterval = 1s, repaintInterval = 250ms) don't block the
	// test. The fake-client-backed fetches return synchronously well within
	// the deadline.
	var sawPlaylists, sawDevices bool
	for _, child := range batch {
		if child == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { ch <- c() }(child)
		select {
		case msg := <-ch:
			switch msg.(type) {
			case playlistsMsg:
				sawPlaylists = true
			case devicesMsg:
				sawDevices = true
			}
		case <-time.After(100 * time.Millisecond):
			// tick or otherwise slow Cmd — ignore
		}
	}
	if !sawPlaylists {
		t.Error("Init batch did not produce a playlistsMsg — fetchPlaylists missing")
	}
	if !sawDevices {
		t.Error("Init batch did not produce a devicesMsg — fetchDevices missing")
	}
}
```

If `time` is not yet imported in `panel_playlists_test.go`, add it to the existing import block. Existing imports there are `context`, `errors`, `testing`, `tea`, `domain`, `fake` — `time` is new.

- [ ] **Step 2: Run the new test to verify it fails**

Run:
```bash
go test ./internal/app/ -run TestInitFetchesPlaylistsAndDevicesEagerly -v
```

Expected: FAIL — both `sawPlaylists` and `sawDevices` are false because today's `Init` only batches status + ticks.

- [ ] **Step 3: Edit `Init` in `internal/app/model.go`**

Replace the body of `Model.Init` (currently at lines 141-147):

```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.client),
		scheduleStatusTick(),
		scheduleRepaintTick(),
		fetchPlaylists(m.client),
		fetchDevices(m.client),
	)
}
```

- [ ] **Step 4: Run the new test to verify it passes**

Run:
```bash
go test ./internal/app/ -run TestInitFetchesPlaylistsAndDevicesEagerly -v
```

Expected: PASS.

- [ ] **Step 5: Run the full app package tests to confirm no regressions**

Run:
```bash
go test ./internal/app/
```

Expected: PASS. The Init change shouldn't affect any other test (they run their own messages, not `Init`).

- [ ] **Step 6: Commit**

```bash
git add internal/app/model.go internal/app/panel_playlists_test.go
git commit -m "$(cat <<'EOF'
app: eager-fetch playlists and AirPlay devices in Init

Init's tea.Batch now also fires fetchPlaylists and fetchDevices alongside
the existing status probe + ticks, so the Playlists and Output panels
populate on launch instead of waiting for the user to tab to each.

The lazy onFocusPlaylists / onFocusOutput loaders are intentionally left
in place — they short-circuit on populated lists and naturally retry on
empty/failed startup fetches, which is the resilience path called for in
the spec.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — First-playlist track prefetch

This phase makes the main pane show the first playlist's tracks on launch
without requiring a cursor move.

### Task 4: Prefetch the first playlist's tracks in the `playlistsMsg` handler

**Files:**
- Test: `internal/app/panel_playlists_test.go` — ADD three tests
- Modify: `internal/app/update.go:87-101` (the `playlistsMsg` arm of `Update`)

- [ ] **Step 1: Write the three failing tests**

Append to `internal/app/panel_playlists_test.go`:

```go
func TestPlaylistsMsgPrefetchesFirstPlaylistTracksWhenSelectedEmpty(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}, {Name: "Recent"}})
	c.SetPlaylistTracks("Liked Songs", []domain.Track{{Title: "t1"}, {Title: "t2"}})
	m := New(c, nil)
	// main.selectedPlaylist defaults to "" — that's the trigger for prefetch.

	updated, cmd := m.Update(playlistsMsg{playlists: []domain.Playlist{
		{Name: "Liked Songs"}, {Name: "Recent"},
	}})
	got := updated.(Model)

	if got.main.selectedPlaylist != "Liked Songs" {
		t.Errorf("main.selectedPlaylist = %q; want %q (auto-selected first playlist)",
			got.main.selectedPlaylist, "Liked Songs")
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylistTracks Cmd for first playlist; got nil")
	}
	out := cmd()
	tracksMsg, ok := out.(playlistTracksMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want playlistTracksMsg", out)
	}
	if tracksMsg.name != "Liked Songs" {
		t.Errorf("playlistTracksMsg.name = %q; want %q", tracksMsg.name, "Liked Songs")
	}
}

func TestPlaylistsMsgDoesNotClobberExistingSelection(t *testing.T) {
	m := newTestModel()
	m.main.selectedPlaylist = "Recent" // user already moved cursor before list arrived

	updated, cmd := m.Update(playlistsMsg{playlists: []domain.Playlist{
		{Name: "Liked Songs"}, {Name: "Recent"},
	}})
	got := updated.(Model)

	if got.main.selectedPlaylist != "Recent" {
		t.Errorf("main.selectedPlaylist = %q; want %q (must not be clobbered)",
			got.main.selectedPlaylist, "Recent")
	}
	if cmd != nil {
		t.Errorf("expected no Cmd when selectedPlaylist is already set, got %T", cmd())
	}
}

func TestPlaylistsMsgWithEmptyResultDoesNotPrefetch(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(playlistsMsg{playlists: nil})
	got := updated.(Model)

	if got.main.selectedPlaylist != "" {
		t.Errorf("main.selectedPlaylist = %q; want empty (no items to select)",
			got.main.selectedPlaylist)
	}
	if cmd != nil {
		t.Errorf("expected no Cmd with empty playlists, got %T", cmd())
	}
}
```

- [ ] **Step 2: Run the three new tests to verify they fail**

Run:
```bash
go test ./internal/app/ -run 'TestPlaylistsMsgPrefetchesFirstPlaylistTracksWhenSelectedEmpty|TestPlaylistsMsgDoesNotClobberExistingSelection|TestPlaylistsMsgWithEmptyResultDoesNotPrefetch' -v
```

Expected:
- `TestPlaylistsMsgPrefetchesFirstPlaylistTracksWhenSelectedEmpty` FAILS — `selectedPlaylist` is still `""`, no Cmd.
- `TestPlaylistsMsgDoesNotClobberExistingSelection` PASSES already (today's handler doesn't touch `selectedPlaylist`).
- `TestPlaylistsMsgWithEmptyResultDoesNotPrefetch` PASSES already.

The first failing test is the one we're driving the implementation off. Keep the other two — they pin down invariants the implementation must preserve.

- [ ] **Step 3: Edit the `playlistsMsg` arm of `Update` in `internal/app/update.go`**

Locate the existing `case playlistsMsg:` block (currently lines 87-101). Replace its final `return m, nil` with the prefetch-or-return pattern:

```go
	case playlistsMsg:
		m.playlists.loading = false
		if msg.err != nil {
			// List-fetch failure — flash in the bottom strip (auto-dissolves)
			// rather than clobbering the Playlists panel. User retries by
			// re-focusing the panel.
			m.lastError = msg.err
			m.lastErrorAt = time.Now()
			return m, clearErrorAfter()
		}
		m.playlists.items = msg.playlists
		if m.playlists.cursor >= len(msg.playlists) {
			m.playlists.cursor = 0
		}
		if len(m.playlists.items) > 0 && m.main.selectedPlaylist == "" {
			name := m.playlists.items[0].Name
			m.main.selectedPlaylist = name
			return m, fetchPlaylistTracks(m.client, name)
		}
		return m, nil
```

The only change is the new `if len(m.playlists.items) > 0 && m.main.selectedPlaylist == ""` block above the final `return m, nil`. Everything above it is unchanged.

- [ ] **Step 4: Run the three tests to verify they pass**

Run:
```bash
go test ./internal/app/ -run 'TestPlaylistsMsgPrefetchesFirstPlaylistTracksWhenSelectedEmpty|TestPlaylistsMsgDoesNotClobberExistingSelection|TestPlaylistsMsgWithEmptyResultDoesNotPrefetch' -v
```

Expected: all three PASS.

- [ ] **Step 5: Run the full app package tests**

Run:
```bash
go test ./internal/app/
```

Expected: all PASS.

If `TestPlaylistsMsgPopulatesPanelStateOnSuccess` (update_test.go:589) starts producing unexpected behaviour, note that it discards the returned Cmd with `_` and so will continue to pass — the prefetch Cmd is silently dropped. No edit needed to that test.

- [ ] **Step 6: Run the entire test suite for the repository**

Run:
```bash
go test ./...
```

Expected: PASS across all packages. The change is local to `internal/app/`; nothing else should be affected.

- [ ] **Step 7: Commit**

```bash
git add internal/app/update.go internal/app/panel_playlists_test.go
git commit -m "$(cat <<'EOF'
app: prefetch first playlist's tracks on playlistsMsg

When the playlists list arrives and main.selectedPlaylist is empty
(launch path — no user cursor move yet), auto-select the first playlist
and fire fetchPlaylistTracks for it so the main pane shows tracks from
frame zero rather than an empty list waiting for j/k.

The selection is guarded so a fast user who moved the cursor before the
list arrived does not get their choice clobbered. Empty results are a
no-op.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Manual verification

### Task 5: Smoke-test the running TUI and confirm full test suite still passes

**No files modified.**

- [ ] **Step 1: Confirm the full test suite is green**

Run:
```bash
go test ./...
```

Expected: PASS across all packages. (Same as Task 4 Step 6 — re-running here as the final gate.)

- [ ] **Step 2: Build the binary**

Run:
```bash
go build -o goove ./cmd/goove
```

Expected: clean build, no warnings.

- [ ] **Step 3: Manual smoke test — Apple Music running**

Open Apple Music on the host so the eager fetches have something to return,
then run:

```bash
./goove
```

Expected on launch (within a couple of seconds):
- The Playlists panel populates with playlist names (no transient empty / "(no playlists)" frame).
- The Output panel populates with AirPlay devices (no transient empty / "(no devices)" frame).
- The main pane shows the tracks of the **first** playlist in the list, *without* having pressed `j`/`k` or having focused the Playlists panel.

Press `q` to quit.

- [ ] **Step 4: Manual smoke test — Apple Music NOT running (failure path)**

Quit Apple Music. Run `./goove` again.

Expected:
- The TUI launches and shows the Disconnected state in the now-playing area.
- Both Playlists and Output panels briefly show `loading…`, then clear to their empty-state placeholders (`(no playlists)` / `(no devices)`) once the failed fetches return.
- Pressing `Tab` to focus the Playlists panel re-fires `fetchPlaylists` (the lazy on-focus retry) — verify by pressing `space` (which calls `Launch`) to start Music.app, then `Tab` over Playlists; the list should populate.

Press `q` to quit.

- [ ] **Step 5: Confirm branch state**

Run:
```bash
git log --oneline main..HEAD
git status
```

Expected:
- Three commits on the branch (Phase 2, Phase 3, Phase 4).
- Working tree clean (or only unrelated `.claude/` and `main` binary untracked).

- [ ] **Step 6: Hand off**

The feature is now ready for review. Surface the branch name (`feature/eager-load`) and the three commit subjects so the user can decide whether to open a PR via the existing flow.

Do **not** open a PR or push the branch automatically — those are user-confirmation actions per the project's working norms.
