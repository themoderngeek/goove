# goove — TUI overhaul (LazyGit-inspired multi-panel layout)

**Date:** 2026-05-04
**Status:** Draft, awaiting review
**Predecessors:** `2026-04-30-goove-mvp-design.md`, `2026-05-03-goove-search-design.md`, `2026-05-03-playlists-design.md`

## 1. Summary

Replace goove's current single-screen now-playing view + three modal overlays
(search, output picker, playlist browser) with a persistent four-zone layout
inspired by LazyGit: a now-playing panel on top, three stacked source panels
on the left (Playlists / Search / Output), and a main pane on the right that
defaults to the tracks of the currently-playing playlist. Focus moves between
panels with `Tab` or direct number jumps (`1`–`4`); navigation within a panel
uses `j`/`k`/arrows. Album art renders inline in the now-playing panel when
the terminal is wide enough.

The CLI (`internal/cli/*`), the AppleScript / Music client layer
(`internal/music/applescript`), and the domain types (`internal/domain/*`) are
**unchanged**. The work is contained inside `internal/app/`.

## 2. Scope and non-goals

### In scope
- Replace `internal/app/` TUI shell (`model.go`, `update.go`, `view.go`,
  `search.go`, `browser.go`, `picker.go`) with the new layout.
- Retire the three modal state types (`*searchState`, `*pickerState`,
  `*browserState`) and the `mode viewMode` / `browserPane` enums; their
  *functionality* lives on as panels.
- Inline album art in the now-playing panel, auto-hidden on narrow terminals.
- Keep all existing global keys working: `space`, `n`, `p`, `+`/`-`, `q`.
- Add new navigation keys: `Tab` / `Shift-Tab`, `1`/`2`/`3`/`4`, `j`/`k`,
  `↑`/`↓`, `/` (focuses Search and starts input mode), `o` (focuses Output),
  `Esc` (returns main pane to the selected playlist when it's showing search
  results).
- Behaviour parity for every current feature — playlist browse + play, library
  search + play, output device switch, play/pause/skip/volume, error footer.

### Out of scope (deferred)
- The CLI surface — every `goove …` subcommand keeps working identically.
- New features the new layout *enables* but doesn't include in this overhaul:
  queue / up-next panel, library tree by artist/album, lyrics, persisted
  selected-playlist across runs, themes, configurable keybinds.
- Restructuring `internal/music/*`, `internal/domain/*`, `internal/cli/*` — out
  of scope.
- Any change to AppleScript performance / caching behaviour beyond what
  already exists.

## 3. UX — layout

The layout is the same regardless of state; panels render placeholders rather
than disappearing. Brainstorming wireframes are at
`.superpowers/brainstorm/1247-1777848758/content/layout-hybrid.html`
(focus on Playlists pane, with art-enabled now-playing).

```
┌─ goove ──────────────────────────────────────────────────┐
│ ┌─ Now Playing ──────────────────────────────────────────┐ │
│ │  ▓ART▓  ▶  Track Title                                 │ │
│ │         Artist · Album                                 │ │
│ │         ▮▮▮▮▮▮▮▮▯▯▯▯▯▯▯▯▯  3:42 / 8:02   vol 50%        │ │
│ └────────────────────────────────────────────────────────┘ │
│ ┌Playlists────┐ ┌─ Liked Songs (now playing) ────────────┐ │
│ │▶ Liked Songs│ │   1. …                                 │ │
│ │  Recent     │ │   2. …                                 │ │
│ │  Top 25     │ │ ▶ 3. Stairway to Heaven  Led Zeppelin  │ │
│ └─────────────┘ │   4. …                                 │ │
│ ┌Search───────┐ │                                        │ │
│ │ /led ze     │ │                                        │ │
│ │  3 results  │ │                                        │ │
│ └─────────────┘ │                                        │ │
│ ┌Output───────┐ │                                        │ │
│ │▶ MacBook    │ │                                        │ │
│ │  Sonos      │ │                                        │ │
│ └─────────────┘ └────────────────────────────────────────┘ │
│ tab:focus  j/k:nav  ⏎:play  /:search  o:output  q:quit   │
└──────────────────────────────────────────────────────────┘
```

**Visual conventions.** A focused panel has a highlighted border (yellow);
the cursor row in a focused panel is yellow. The currently-playing source
(playlist row, output device) and currently-playing track use a green `▶`.
The bottom hint bar shows global keys plus the focused panel's panel-scoped
keys.

### Now Playing panel (top, full width)
- Renders the current `Connected.Now` exactly as today: title, artist, album,
  progress bar, time, volume bar, percent.
- When the terminal width is `>= artLayoutThreshold` (the existing constant)
  and a chafa-rendered art string is in the cache, the art renders left of
  the text (same `lipgloss.JoinHorizontal` composition used today).
- In `Idle` and `Disconnected` states, the panel renders a muted placeholder:
  `— nothing playing —` (Idle) or `— Music not running —` (Disconnected). The
  panel keeps its borders and chrome regardless of state.
- Not focusable. Tab order skips it.

### Playlists panel (left, top of stack)
- Lists the user's playlists, fetched on first focus (cached after).
- `j`/`k`/`↑`/`↓` moves the cursor.
- **Live preview (Q3-C):** every cursor move updates the main pane to show
  the tracks of the highlighted playlist. Tracks are cached per playlist
  inside the panel state — first preview triggers a fetch, subsequent
  previews hit cache.
- ⏎ plays the highlighted playlist (same code path used by the retired
  browser modal's "play playlist" action).
- Currently-playing playlist is marked with a green `▶`.

### Search panel (left, middle of stack)
- Renders one of three states:
  - **Idle:** `/  type to search` (single muted line; panel is shortest
    in this state).
  - **Input mode:** the query as the user types it, blinking caret, plus
    `searching…` while a query is in flight.
  - **Done:** `"led ze"` on top line, `23 results` muted on second line.
- Input mode is entered by focusing the panel (`2` or `Tab` to it) **and** the
  user starts typing. Typing while focused goes into the query (alphanumerics
  do NOT fire global skip-track shortcuts; see §5).
- ⏎ fires the search:
  - Main pane switches to `mainPaneSearchResults` and shows the results.
  - Focus moves to the main pane so `j`/`k` and ⏎ are immediately available.
- `Esc` while typing clears the query and exits input mode (panel returns to
  Idle state, focus stays on Search panel).
- Search uses the same `client.SearchTracks` + `domain.RankSearchResults`
  pipeline already in place. The `seq`-stamping for stale-result rejection is
  preserved.

### Output panel (left, bottom of stack)
- Lists AirPlay-capable devices, fetched on first focus (cached after).
- `j`/`k`/`↑`/`↓` moves the cursor.
- **Two-step (Q3-C):** cursor moves do NOT switch device. ⏎ on the
  highlighted device fires `client.SetDevice(...)` and updates the panel state
  on success.
- Currently-selected device marked with a green `▶`.

### Main pane (right of left column)
- Has its own internal mode (`mainPaneMode`):
  - `mainPaneTracks` (default) — shows the tracks of whichever playlist is
    currently selected in the Playlists panel.
  - `mainPaneSearchResults` — shows the rows of the most recent search.
- In `mainPaneTracks`: panel title is the playlist name, plus
  `(now playing)` when this playlist is the source of the current track. The
  currently-playing track row is marked with a green `▶`.
- In `mainPaneSearchResults`: panel title is `Search: "<query>" · N results`.
  Same row layout (title / artist).
- `j`/`k`/`↑`/`↓`/`g`/`G`/`Ctrl-d`/`Ctrl-u` navigate. (Strictly speaking the
  navigation key set is the Q4-B set — `g`/`G`/`Ctrl-d`/`Ctrl-u` are *not*
  required by the brainstormed scope. They're optional polish for the main
  pane only. See §11.)
- ⏎ plays the highlighted track:
  - In `mainPaneSearchResults`: `client.PlayTrack(persistentID)` (track-level
    play, same as the existing search modal's enter behaviour).
  - In `mainPaneTracks`: `client.PlayPlaylist(name, trackIndex)` — starts the
    selected playlist at the cursor's row (1-based index; same as the
    existing browser modal's enter behaviour).
- `Esc` in `mainPaneSearchResults` returns the pane to `mainPaneTracks` (i.e.
  back to the selected playlist). `Esc` in `mainPaneTracks` is a no-op.

### Hint bar (bottom)
- One line, faint style.
- Always shows the global keys (`space:play/pause  n:next  p:prev  +/-:vol  q:quit`).
- Adds the focused panel's own keys (e.g. on Playlists: `⏎:play  j/k:nav`;
  on Search input mode: `⏎:run  esc:clear`).
- Replaces the existing `connectedKeybindsText` constant.

## 4. UX — empty / disconnected / loading states

- **Music.app running, nothing playing (`Idle`).** Now-playing panel shows the
  muted `— nothing playing —` line and the volume bar. All other panels
  render normally — playlists, output, search work fine.
- **Music.app not running (`Disconnected`).** Full layout still renders. Every
  panel shows a muted `—` placeholder for content. Hint bar swaps to
  `space: launch Music   q: quit`. Pressing `space` launches Music; the
  status tick recovers the rest. **No full-screen takeover.**
- **Loading (panel data fetched on first focus).** Affected panel shows
  `loading…` muted; other panels are unaffected.
- **Permission denied** (`m.permissionDenied = true`). Keep the existing
  full-screen `renderPermissionDenied` takeover with the System Settings
  instructions — this is sticky, instructive, and not a per-panel concern.

## 5. UX — keyboard

### Global keys (work regardless of focus)
| Key | Action |
|---|---|
| `space` | play / pause (or launch Music if Disconnected) |
| `n` | next track |
| `p` | previous track |
| `+` / `=` | volume +5% |
| `-` | volume −5% |
| `q` | quit |
| `Tab` / `Shift-Tab` | cycle focus forward / backward through Playlists → Search → Output → Main → … |
| `1` | focus Playlists |
| `2` | focus Search |
| `3` | focus Output |
| `4` | focus Main |
| `/` | focus Search and enter input mode |
| `o` | focus Output |

`Esc` is panel-scoped (different behaviour per focused panel). See the
panel-scoped table below.

### Panel-scoped keys
| Panel | Key | Action |
|---|---|---|
| Playlists | `j`/`k`/`↑`/`↓` | move cursor (live-previews tracks in main pane) |
| Playlists | `⏎` | play the highlighted playlist |
| Search (idle) | any printable | enter input mode and start the query |
| Search (input) | printable | append to query |
| Search (input) | `Backspace` | remove last rune |
| Search (input) | `⏎` | fire search; main pane shows results; focus moves to main |
| Search (input) | `Esc` | clear query and exit input mode |
| Output | `j`/`k`/`↑`/`↓` | move cursor |
| Output | `⏎` | switch audio to the highlighted device |
| Main | `j`/`k`/`↑`/`↓` | move cursor |
| Main | `⏎` | play the highlighted track |
| Main | `Esc` | (search-results mode only) back to selected playlist |

### Search input vs globals — disambiguation
When focus is on the Search panel and the panel is in input mode:

- **Printable characters** (alphanumerics, punctuation, including `space`) go
  into the query. `n`, `p`, `q`, and `space` do NOT fire global skip / quit /
  play-pause shortcuts while typing.
- **Non-printable / control keys** still act as globals: `Tab`, `Shift-Tab`,
  `1`–`4`, `Ctrl-…`, plus `Esc` (panel-scoped) and `⏎` (panel-scoped).

To play/pause while a query is half-typed, the user `Tab`s or `Esc`s out of
input mode first. This matches the standard behaviour of every search field
in every TUI / terminal app.

### Retired keys
- `l` (currently opens the playlist browser modal) is freed up — the
  Playlists panel is always visible. Reserved for a future "library tree"
  panel; for v1, `l` is unbound.

## 6. Architecture

The TUI lives entirely in `internal/app/`. The package keeps a single flat
`Model` struct (matches the existing pattern) and a single top-level `Update`
/ `View` pair. Per-panel state, render, and key handling are factored into
sibling files; panels are NOT independent `tea.Model`s — they're plain
state structs operated on by the parent `Update`. This avoids the
message-forwarding boilerplate that "panel-as-Model" would force, at the cost
of weaker isolation (panels can read each other's state). For a package this
small, the trade is worth it.

### File layout

| File | Role |
|---|---|
| `model.go` | `Model` struct, `focus` enum, `mainPaneMode` enum, panel state types, `New` / `Init`. |
| `update.go` | Top-level `Update`. Handles Cmd-result messages, then `KeyMsg` → globals → focus-routed panel handler. |
| `view.go` | Top-level `View`. Composes the four panels + hint bar with lipgloss. Permission-denied takeover stays here. |
| `panel_now_playing.go` | Renders the now-playing panel (with optional art). Not focusable. |
| `panel_playlists.go` | Playlists panel: state struct, fetch wiring, key handler, render. |
| `panel_search.go` | Search panel: query/seq/results state, debounce wiring, key handler, render. |
| `panel_output.go` | Output panel: device-list state, fetch wiring, key handler, render. |
| `panel_main.go` | Main pane: `mainPaneMode` switch, key handler, render. |
| `hints.go` | Bottom hint bar — global keys + focused panel's keys. |
| `messages.go`, `tick.go` | Unchanged (Cmd → Msg types, ticker). |

### State model

```go
type focus int

const (
    focusPlaylists focus = iota
    focusSearch
    focusOutput
    focusMain
)

type mainPaneMode int

const (
    mainPaneTracks        mainPaneMode = iota // tracks of selected playlist
    mainPaneSearchResults                     // rows from last fired search
)

type playlistsPanel struct {
    items        []domain.Playlist
    cursor       int
    loading      bool
    err          error
    tracksByName map[string][]domain.Track // per-playlist track cache
    fetchingFor  map[string]bool           // suppresses duplicate fetches
}

type searchPanel struct {
    inputMode bool
    query     string
    seq       uint64
    loading   bool
    lastQuery string // the query the latest results are for
    total     int
    err       error
}

type outputPanel struct {
    devices []domain.AudioDevice
    cursor  int
    loading bool
    err     error
}

type mainPanel struct {
    mode             mainPaneMode
    cursor           int
    selectedPlaylist string         // name of the playlist whose tracks are showing
    searchResults    []domain.Track // populated when mode == mainPaneSearchResults
}

type Model struct {
    client music.Client

    state       AppState
    lastVolume  int
    lastError   error
    lastErrorAt time.Time

    permissionDenied bool

    width  int
    height int

    art      artState
    renderer art.Renderer

    focus focus

    playlists playlistsPanel
    search    searchPanel
    output    outputPanel
    main      mainPanel
}
```

### What goes away
- `searchState`, `pickerState`, `browserState` (all modal-shaped).
- `mode viewMode`, `browserPane`.
- `compactThreshold` / `renderCompact` — replaced by graceful narrow rendering
  per panel. (See §11 for the open question on minimum width handling.)
- `connectedKeybindsText` — replaced by the dynamic hint bar in `hints.go`.

### Cmds and messages

Panel key handlers may return `tea.Cmd`s. Result messages are handled at the
top level in `update.go`, which dispatches to the relevant panel state. The
existing message types (`statusMsg`, `searchResultsMsg`, `devicesMsg`,
`deviceSetMsg`, `playlistsMsg`, `tracksMsg`, `actionDoneMsg`, `tickMsg`,
`repaintMsg`, `clearErrorMsg`, `artworkMsg`) keep their wire shapes —
only their *handlers* move from "modal-aware" to "panel-aware."

### Live preview wiring

When the Playlists panel cursor moves:
1. Update `m.playlists.cursor`.
2. Read `name := m.playlists.items[cursor].Name`.
3. Set `m.main.selectedPlaylist = name` and `m.main.cursor = 0`.
4. If `m.playlists.tracksByName[name]` is missing and not already fetching:
   set `m.playlists.fetchingFor[name] = true` and return a `fetchTracks(name)`
   Cmd. The result `tracksMsg` populates the cache.
5. If cached, no Cmd.

`panel_main.go` reads `m.main.selectedPlaylist`, looks up
`m.playlists.tracksByName[name]`, and renders. While the cache is filling, it
shows `loading…`.

## 7. Phase plan (migration)

Each phase is a self-contained commit (or small PR) that leaves the app
working and tested. This mirrors the staging used for the search feature.

**Phase 1 — Layout shell.** Add the `focus` enum and the four-zone layout.
Now-playing panel renders today's `renderConnected` content in the new top
slot. Playlists / Search / Output panels show `—` placeholders. Main pane
shows a static hint. `Tab`/`Shift-Tab`/`1`/`2`/`3`/`4` cycle focus visibly
but don't dispatch yet. Existing global keys still work. **The old `/`, `o`,
`l` modals are still triggerable** and overlay on top, exactly as today —
this is the only phase where old and new visibly coexist. App ships and
tests pass.

**Phase 2 — Playlists panel + main pane (live preview).** Lift playlist
listing and play-playlist logic out of `browser.go` into the Playlists
panel and main pane. Wire live preview. ⏎ in Playlists plays the playlist;
⏎ in main pane plays from a track. **Retire the `l` browser modal**, the
`browserState`, the `mode viewMode` field, the `browserPane` enum.
Migrate browser-modal tests into `panel_playlists_test.go` and
`panel_main_test.go`.

**Phase 3 — Search panel.** Lift query/debounce/seq/result-fetch logic
out of `search.go` into the Search panel and main pane (search-results
mode). Type query in panel header → ⏎ fires search → main pane shows
results, focus jumps to main. `Esc` from main pane returns it to selected
playlist tracks. **Retire the `/` search modal** and `searchState`.
Migrate search-modal tests into `panel_search_test.go` and
`panel_main_test.go`.

**Phase 4 — Output panel.** Lift device-fetch and device-set logic out
of `picker.go` into the Output panel. Two-step semantics: cursor moves
don't switch device, ⏎ does. **Retire the `o` picker modal** and
`pickerState`. Migrate picker-modal tests into `panel_output_test.go`.

**Phase 5 — Album art in the now-playing panel.** Move chafa art
rendering into `panel_now_playing.go`. Auto-hide below
`artLayoutThreshold` width. Track-change cache invalidation
(`artState.key` matching) is unchanged.

**Phase 6 — Cleanup.** Remove `compactThreshold` / `renderCompact` (replaced
by per-panel graceful rendering — see §11), `connectedKeybindsText`,
`mode viewMode`. Final pass on `update_test.go` and `search_test.go` to
remove anything pointing at deleted state. README + `goove help` updated:
new keys, new screenshot.

## 8. Errors

The existing `lastError` / `errFooter` / `clearErrorAfter` pattern stays.
Errors render in the bottom strip above the hint bar exactly as today.

Per-panel fetch failures (e.g. `getPlaylists` returns an error): the panel
renders an inline muted-red `error: …` line in place of its content. Other
panels are unaffected. The panel re-fetches when refocused (or, for a future
enhancement, on a manual retry key).

`PlayTrack` / `SetDevice` / `PlayPlaylist` errors set `m.lastError` and
appear in the bottom error strip — same as today. The originating panel
stays as it was; the user's selection is not lost.

The `permissionDenied` full-screen takeover is preserved verbatim.

## 9. Testing

### Strategy
- **TDD per phase.** Each phase starts with tests for the new behaviour, then
  implementation.
- **Table-driven `KeyMsg` tests** are the spine, same as today.
- **`fake.Client`** is reused as-is — no new fixtures.
- **View tests stay light.** Don't snapshot full output. Assert specific
  fragments: focused panel highlighted, hint bar contents, main pane title.

### Test files (added in their respective phases)

| File | Coverage |
|---|---|
| `update_test.go` | Globals, focus transitions (`Tab`/`Shift-Tab`/`1`–`4`), permission-denied takeover. |
| `panel_playlists_test.go` | `j`/`k` cursor, ⏎ plays, live-preview wiring (selection moves → `m.main.selectedPlaylist` updated, fetch fired or cache hit). |
| `panel_search_test.go` | Idle → input mode transition, typing increments `seq`, debounce drop on stale seq, ⏎ fires search and moves focus, `Esc` clears. |
| `panel_output_test.go` | `j`/`k` cursor, ⏎ switches device, error-result handling. |
| `panel_main_test.go` | Mode flip on search results, `Esc` returns to tracks, ⏎ plays, cursor reset on selection change. |
| `panel_now_playing_test.go` | Render with/without art at width threshold, idle / disconnected placeholder rendering. |

### Tests that get migrated
- `search_test.go` debounce / seq / result tests → `panel_search_test.go`
  (same logic, panel shell instead of modal shell).
- `update_test.go` browser-modal blocks → `panel_playlists_test.go` /
  `panel_main_test.go`.
- `update_test.go` picker-modal blocks → `panel_output_test.go`.
- `update_test.go` search-modal blocks → `panel_search_test.go`.

### Tests that get deleted
- Anything asserting modal lifecycle (`m.search != nil`, `m.picker != nil`,
  modal-on-modal interactions, `mode == modeBrowser`). The state types don't
  exist in the new shape.

### Integration tests
- `client_integration_test.go` (real Music.app) is unchanged — it doesn't
  exercise the TUI layer.

### Manual smoke check at the end of each phase
- `go run ./cmd/goove` against real Music.app.
- Walk the keybinds: `Tab` cycle, number jumps, `j`/`k` in each panel, ⏎
  behaviours, search end-to-end, output switch, play/pause/skip, volume.

## 10. Risks and trade-offs

- **Phase 1's transient weirdness.** Phase 1 ships with the new layout *and*
  the old modals still triggerable on top — this is briefly visually
  inconsistent (`/` opens a modal over a layout that itself contains a
  search panel). This state lasts only until each modal is retired in its
  own phase. Acceptable for the safety of incremental migration.
- **Flat-Model isolation.** Panels can read each other's state. The team must
  remember not to. If this becomes a problem (it shouldn't at 3k LOC), the
  refactor to panel-as-`tea.Model` is mechanical.
- **Live-preview cache size.** `playlistsPanel.tracksByName` is unbounded —
  every previewed playlist is cached forever. Bounded user libraries make
  this fine in practice (tens of playlists, not thousands). If this is ever
  a concern, an LRU is a one-file change.
- **AppleScript performance for huge libraries.** Same concern as the search
  spec. No new exposure here — the new layout doesn't fire more queries.
- **Test rewrite cost.** `update_test.go` is 1247 lines and `search_test.go`
  is 424. A meaningful fraction has to be re-shaped. This is the largest
  single concrete cost of the migration. Phasing distributes it.

## 11. Open questions for the plan

These are noted-but-not-decided in this spec. The implementation plan should
resolve them or push them to a follow-up.

- **`g`/`G`/`Ctrl-d`/`Ctrl-u` in the main pane.** Out of the brainstormed
  scope (Q4 picked option B, not C). They're listed in §3 as optional polish;
  the plan should either include them as part of Phase 2 or drop them. My
  recommendation: drop for v1.
- **Minimum terminal width.** The current `renderCompact` path kicks in at
  width < 50. The new layout's minimum usable width is wider (probably ~70).
  The plan should decide the new threshold and the fallback behaviour
  (degrade to text-only single-column? show a "terminal too narrow" hint?).
  My recommendation: a single "terminal too narrow" centred hint below ~60
  cols; track the exact threshold during Phase 1 implementation.
- **Hint bar overflow at narrow widths.** At ~80 cols the hint bar can fit
  globals + a small panel-specific addendum. Below that, it has to truncate.
  The plan should define the truncation rule. My recommendation: globals
  always present, panel-specific keys dropped first.
- **Refresh keys.** The browser modal currently uses `r` to refresh tracks;
  the search modal uses `Ctrl-R`. The new layout has no obvious "refresh"
  affordance. The plan should decide whether to add a global / per-panel
  refresh key or omit it for v1. My recommendation: omit for v1; add later
  if users miss it.

## 12. Implementation outline (preview, not a plan)

The detailed plan is the next deliverable (writing-plans skill). Rough shape
maps directly to §7:

1. **Phase 1 — shell.** New layout + focus + Tab/number cycling, panels show
   placeholders, modals still work. Tests for focus transitions.
2. **Phase 2 — Playlists + main.** Lift from `browser.go`; live preview;
   retire `l` modal + `browserState`. Tests migrated.
3. **Phase 3 — Search.** Lift from `search.go`; main pane search-results
   mode; retire `/` modal + `searchState`. Tests migrated.
4. **Phase 4 — Output.** Lift from `picker.go`; retire `o` modal + `pickerState`.
   Tests migrated.
5. **Phase 5 — Album art in now-playing.** Wire chafa output into
   `panel_now_playing.go`.
6. **Phase 6 — Cleanup.** Remove `renderCompact`, `mode`, leftover constants;
   README + help update; final test sweep.

Each phase is independently shippable behind the previous phase.
