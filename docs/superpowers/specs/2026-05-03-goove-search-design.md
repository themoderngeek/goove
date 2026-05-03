# goove — local-library search

**Date:** 2026-05-03
**Status:** Draft, awaiting review
**Predecessors:** `2026-04-30-goove-mvp-design.md`

## 1. Summary

Add a search feature to the goove TUI that lets the user find and play any
track in their local Apple Music library. Search is opened with `/` from the
now-playing view, displayed in a modal overlay (same shape as the existing
output picker), and runs OR-matched, case-insensitive, substring queries
against title / artist / album. Pressing enter on a result plays the track
and closes the modal. There is no CLI surface for this feature in v1.

## 2. Scope and non-goals

### In scope
- Local-library search for tracks only.
- Modal overlay UI, opened with `/`, esc cancels.
- Debounced live results (~250 ms idle, supersede in-flight queries).
- Substring, case-insensitive match across title, artist, album.
- Sort: title-matches first, then artist, then album; alphabetical within each group.
- 100-row cap with truncation hint when more matches exist.
- Plays a single track on enter; existing 1 Hz tick surfaces the new now-playing.

### Out of scope (deferred)
- Apple Music **catalog** search. AppleScript can't do this; it would need
  MusicKit (CGo / sidecar) or the public Apple Music API (developer key + JWT)
  and is meaningfully larger than v1.
- Album / artist / playlist as result types. Playlists already have `l`.
- Token-split / Google-style multi-word matching (`led stair` → Stairway by Led Zeppelin).
- Queueing the result list as the play context (so `n` walks the hits).
- Persistent search history.
- A CLI subcommand (`goove search …`).

## 3. UX

### Keybind
`/` from the now-playing view opens the modal.

### States
1. **Empty input** — modal open, cursor blinking, no query fired. Subtitle: `type to search your library`.
2. **Searching** — query in flight (after the 250 ms debounce). Previous results cleared. Subtitle: `searching…`.
3. **Results** — first row marked `▶`, rest indented to align. ↑/↓ moves the cursor. Footer summary: `N results`.
4. **No matches** — subtitle: `no matches in your library`.
5. **Truncation** — when `total > 100`, the last row is `…  100 of N — refine the query`.
6. **Error** — error footer line beneath the modal (red, same style as existing `errFooter`). Modal stays open so the user can retry. Keybind footer relabels `r refresh` → `r retry` while an error is showing; the underlying action is identical (re-run the current query).

(See `.superpowers/brainstorm/29974-1777829598/content/search-states.html` for
the wireframes signed off during brainstorming.)

### Suppression rules
- Disconnected (Music not running): `/` is a no-op. Library queries require Music.
- `m.picker != nil` (output picker open): `/` is a no-op.
- `m.mode == modeBrowser`: `/` is a no-op (browser owns its own keybinds).
- `m.permissionDenied`: `/` is a no-op.

### Inside the modal
- Printable characters append to the query, backspace removes the last rune.
- ↑/↓ or `k`/`j` moves the cursor.
- enter plays the highlighted track and closes the modal.
- `r` re-runs the current query immediately (skips debounce).
- esc closes the modal back to now-playing without playing anything.
- Editing the query at any state re-runs the search via debounce; cursor resets to row 0.

## 4. Architecture

The feature lives within the existing layering. No new top-level packages.

```
TUI (internal/app)         <- new search modal: search.go, plus messages
  └ domain (internal/domain)  <- new RankSearchResults pure helper, Track gains PersistentID
       └ music.Client interface  <- new SearchTracks, PlayTrack methods
            ├─ applescript impl   <- new scripts + parsers
            └─ fake impl          <- in-memory matcher
```

## 5. Domain

### `domain.Track` change
Add `PersistentID string` to `Track`. Apple Music tracks have a `persistent ID`
that is stable across syncs and unique within a library — the only safe handle
for "play this specific track" via AppleScript. Existing constructions
(`NowPlaying.Track`, playlist-track fetches) leave it as the zero value if not
populated; nothing else cares about it today.

### `domain.RankSearchResults`
Pure function:

```go
// RankSearchResults orders OR-matched tracks by where the query matched:
// title matches first, then artist, then album. Within each group, results
// are sorted alphabetically by title. Tracks that don't match anywhere
// (shouldn't happen given the upstream OR-match) sort last.
//
// Match is case-insensitive substring — same semantics as the AppleScript
// `whose name contains` clause.
func RankSearchResults(tracks []Track, query string) []Track
```

This is a pure transform — a unit test fixture is enough to lock the ordering.

## 6. Music client interface

Two new methods on `music.Client`:

```go
// SearchTracks returns library tracks whose title, artist, or album contains
// query (case-insensitive). Capped at the first 100 matches. The total field
// in the result lets callers decide whether to render a truncation hint.
SearchTracks(ctx context.Context, query string) (SearchResult, error)

// PlayTrack plays the track with the given persistent ID. Replaces the
// current play context; the existing 1 Hz status tick surfaces the new
// now-playing.
PlayTrack(ctx context.Context, persistentID string) error
```

Where `music.SearchResult` is a small wire type:

```go
type SearchResult struct {
    Tracks []domain.Track // at most 100 entries
    Total  int            // total underlying matches (>= len(Tracks))
}
```

Errors map to existing sentinels: `ErrNotRunning`, `ErrPermission`,
`ErrUnavailable`. A new sentinel `ErrTrackNotFound` is added for `PlayTrack`
when the persistent ID no longer resolves (track was deleted between search
and play — uncommon but possible).

## 7. AppleScript implementation

### Search script
One AppleScript that takes the query as a parameter and returns:
- a leading line with the total match count, and
- up to 100 record lines, one per track.

Each track line is tab-separated: `persistentID \t title \t artist \t album \t duration_seconds`.
Using tabs matches the existing parser style and avoids JSON-shelling. The
parser lives next to the existing ones in `internal/music/applescript/parse.go`.

Match clause:

```applescript
set hits to (every track of library playlist 1 whose ¬
    (name contains q) or (artist contains q) or (album contains q))
```

Total count is `count of hits`; the script then returns the first 100 items
if the count exceeds 100, or all of them otherwise. AppleScript has no built-in
`min`, so this is an explicit `if … then` in the script.

**Security:** the query is escaped before interpolation into the script
exactly as playlist names already are. Embedded `"` and `\` characters are
backslash-escaped; control characters (tab, newline) are stripped from the
input before query is built. This pattern follows the SECURITY-tagged code
already in `scripts.go`.

### PlayTrack script
```applescript
tell application "Music"
    play (some track of library playlist 1 whose persistent ID is "%s")
end tell
```
If no track matches, the AppleScript error maps to `ErrTrackNotFound`.

### Fake client
The `fake` client's `SearchTracks` filters its in-memory track list with a
plain `strings.Contains` over lowered title/artist/album. `PlayTrack` sets
`Now.Track` to the matching entry and `IsPlaying = true`. Used by app-layer
unit tests.

### Cap enforcement
Cap of 100 is enforced **inside the AppleScript** (avoid serialising thousands
of records back to Go). The `Total` field comes from the pre-cap `count of hits`.

## 8. App layer (Bubble Tea)

### State

A new file `internal/app/search.go` declaring:

```go
type searchState struct {
    query    string
    seq      uint64       // bumped on every keystroke; incoming msgs use it to discard stale results
    loading  bool
    results  []domain.Track
    total    int
    cursor   int
    err      error
}
```

`Model` gains a single new field:

```go
search *searchState // nil unless the search modal is open
```

### View
`View()` short-circuits on `m.search != nil` immediately after the existing
`m.picker != nil` check. Render lives in `search.go` next to the picker.

### Messages (added to `messages.go`)

```go
type searchDebounceMsg struct{ seq uint64 } // fired by tea.Tick after 250ms

type searchResultsMsg struct {
    seq    uint64        // matches the seq the query was issued under
    query  string        // the query the results are for (defensive)
    result music.SearchResult
    err    error
}

type searchPlayedMsg struct{ err error } // result of PlayTrack
```

### Update flow
1. `/` from now-playing → `m.search = &searchState{}`. No command — we don't fire a debounce until the user actually types something.
2. Printable key while `m.search != nil`:
   - append rune to `query`, increment `seq`, clear `results`/`total`/`err`, return `tea.Tick(250ms) → searchDebounceMsg{seq}`.
3. Backspace: same as above, with rune removed.
4. `searchDebounceMsg{seq}`:
   - If `seq != m.search.seq`, drop. (Stale.)
   - If `query == ""`, drop (don't fire empty queries).
   - Set `loading = true`, fire `client.SearchTracks(ctx, query)` via a `tea.Cmd` (same pattern as `fetchPlaylists`); result becomes a `searchResultsMsg{seq, query, ...}`.
5. `searchResultsMsg{seq, ...}`:
   - If `seq != m.search.seq`, drop.
   - Set `loading = false`. Populate `results` (via `domain.RankSearchResults`), `total`, `err`. Reset `cursor = 0`.
6. ↑/↓ in modal: move cursor, no command.
7. enter: fire `client.PlayTrack(persistentID)`; close the modal optimistically (`m.search = nil`). Track errors via `searchPlayedMsg`.
8. `r`: identical to keystroke flow but skip debounce — fire query directly.
9. esc: `m.search = nil`, no command.
10. Disconnected / picker-open / browser-open: `/` is a no-op.

### Debounce sequence numbers
The `seq` field is the standard "version-stamp every command" pattern (the
artwork fetch already uses this with `key`). Every state change that should
invalidate in-flight work bumps `seq`. Late-arriving messages with stale seqs
are dropped silently.

### Footer
Modal renders its own footer text — same pattern as the picker. Connected-state
footer (`connectedKeybindsText`) gains `/: search` between `+/-: vol` and
`o: output`. README and `goove help` are updated to match (similar polish to
the docs sync just landed).

## 9. Errors

Errors from `SearchTracks` and `PlayTrack` are surfaced through the modal's
own error footer (red, same style as `m.errFooter`) — they do not propagate
to `m.lastError` because the modal owns its own error display. When the modal
closes (esc / successful play), any error is discarded. The 1 Hz tick continues
in the background; transient errors don't stop polling.

## 10. Testing

### Domain
- `RankSearchResults` — table-driven tests covering: title-match priority,
  artist-match second, album-match third, alphabetical-by-title within group,
  case-insensitive matching, mixed groups.

### Music client
- `applescript/parse_test.go` — parse the tab-separated search-result format
  (zero results, one result, exactly 100 results, total > 100, empty fields).
- `applescript/client_test.go` — `SearchTracks` invokes the right script with
  the escaped query; error mapping (NotRunning, Permission, generic).
  `PlayTrack` invokes the right script with the persistent ID; mapping for
  TrackNotFound, NotRunning, Permission.
- `applescript/client_integration_test.go` (build-tagged) — runs `SearchTracks`
  against the real library, picks one, calls `PlayTrack`. Skipped by default.
- `fake/client_test.go` — `SearchTracks` returns matching tracks; `PlayTrack`
  flips state.

### App
- New `app/search_test.go` (or extend `update_test.go`) covering:
  - `/` opens the modal in Connected; no-op in Disconnected / picker-open /
    browser-open / permissionDenied.
  - Typing increments `seq`, schedules a debounce tick, clears prior results.
  - `searchDebounceMsg` with stale seq is dropped.
  - `searchResultsMsg` with stale seq is dropped; with current seq populates
    `results` and `total`.
  - Empty query never fires `SearchTracks`.
  - `r` skips the debounce.
  - enter calls `PlayTrack` and closes the modal.
  - esc closes without playing.
  - Error path (fake client returns an error) renders in the modal footer
    and keeps the modal open.

## 11. Risks and open questions

- **AppleScript performance on huge libraries.** `whose` clauses scale linearly
  in track count. A 50k-track library may take several seconds per query,
  which is bearable behind the debounce + "searching…" indicator but may
  warrant a future move to MusicKit. No action for v1 beyond the cap and
  debounce.
- **Persistent ID stability across iCloud Music Library re-syncs.** Apple's
  docs claim stability; in rare cases (library rebuild, machine swap) the IDs
  do change. We surface `ErrTrackNotFound` cleanly when this happens — the
  user sees an error in the modal footer, can retry the search, and goes
  again with the fresh IDs.
- **Empty libraries.** Search opens fine; first query returns zero matches and
  hits the no-matches state. No special-case handling needed.
- **Unicode case folding.** AppleScript's `contains` is case-insensitive for
  ASCII and reasonable for most Latin diacritics. Non-Latin scripts may match
  inconsistently; we accept this for v1 and don't pre-normalise.

## 12. Implementation outline (preview, not a plan)

The detailed plan is the next deliverable (writing-plans skill). Rough shape:

1. Domain — add `Track.PersistentID`, write `RankSearchResults` + tests.
2. Music client interface — add `SearchTracks`, `PlayTrack`, `SearchResult`, `ErrTrackNotFound`.
3. AppleScript — search and play-track scripts; parser; tests; integration test.
4. Fake client — implementations + tests.
5. App layer — `searchState`, messages, update flow, view (search.go); tests.
6. Footer + README + `goove help` updates.

Each step is independently testable and shippable behind the previous step.
