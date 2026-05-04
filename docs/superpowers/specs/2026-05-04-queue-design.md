# goove — Up Next queue inside the Now Playing panel

**Date:** 2026-05-04
**Status:** Draft, awaiting review
**Predecessors:** `2026-05-04-tui-overhaul-design.md`, `2026-05-04-eager-load-design.md`, `2026-05-03-playlists-design.md`

## 1. Summary

Add a read-only **Up Next** view to the Now Playing panel, showing the
upcoming tracks of the currently-playing playlist after the current track.
The list renders in the wasted vertical space alongside the lower portion
of the album art — the panel's right column becomes a vertical stack of
`title/artist/album → progress → volume → ─ Up Next ─ → upcoming track
rows`. The current horizontal join switches from `lipgloss.Center` to
`lipgloss.Top` so the right column anchors at the top and the queue grows
downward against the bottom of the art.

The queue is decoupled from the Main panel's "browse" cursor: regardless
of which playlist the user is browsing in Main, Up Next always reflects
the playing context. This solves a real workflow gap (browsing without
losing track of what's playing) and reclaims the empty space beside the
art.

The view is read-only: no focus target, no cursor, no key handlers. The
existing four-panel focus model (`Playlists → Search → Output → Main`) is
unchanged. Interactive jump-to-queued-track is explicitly deferred.

## 2. Scope and non-goals

### In scope

- Extend `scriptStatus` to return three additional fields: persistent ID
  of the current track, shuffle-enabled flag, and current playlist name.
- Extend `scriptPlaylistTracks` to return persistent ID per track.
- Extend `domain.NowPlaying` with `CurrentPlaylistName` and
  `ShuffleEnabled`. Populate `PersistentID` on `NowPlaying.Track`
  (currently always empty) — used to locate the playing track inside
  the playlist's track list.
- Populate `Track.PersistentID` on every track returned by
  `PlaylistTracks` (currently always empty for those tracks).
- Render Up Next inside the Now Playing panel with placeholder states for
  shuffle / no-playlist-context / loading / not-found / end-of-playlist.
- Suppress Up Next in narrow terminal mode (`width < artLayoutThreshold`).
- Suppress Up Next when art height ≤ text height (no room).
- When a status tick reports a new `CurrentPlaylistName` not in the
  `tracksByName` cache, dispatch the existing `fetchPlaylistTracks` cmd
  to populate it.
- Tests: parser, render, update, fake-client, integration.

### Out of scope

- Interactive Queue (focus target, cursor, Enter to jump).
- Apple Music's user-curated **Up Next** queue (right-click → Play Next /
  Play Later). AppleScript does not expose this; it likely needs MusicKit
  Swift, tracked in `2026-05-04-musickit-feasibility-design.md`.
- Already-played history above the current track.
- `goove queue` CLI command (deferred to a follow-up spec).
- Time-until-track and total queue duration metadata.
- Cross-playlist queueing.
- Refactoring the existing center-aligned art/text layout for any other
  reason (e.g. compressing artist/album to one line). The alignment
  switch is the only visual change.

## 3. Behaviour

### 3.1 Layout

The Now Playing panel keeps its existing full-top-row geometry. Inside
the panel, the body is rebuilt as:

- Left column: rendered album art (unchanged), present only when
  `width ≥ artLayoutThreshold` and art bytes are loaded for the playing
  track key.
- Right column: vertical stack of
  - title row (state glyph + track title)
  - artist row
  - album row
  - blank line
  - progress bar + position / duration
  - blank line
  - volume bar + percentage
  - **Up Next block** (new) — see 3.2

The two columns are joined horizontally with `lipgloss.Top` (was
`lipgloss.Center`). The panel height is then `lipgloss.Height(body) + 2`
as today; the body height is `max(art_height, text_height + 1 +
queue_rows)`, where `text_height` is the rendered height of the
title/artist/album/progress/volume block, the `+1` is the `─ Up Next ─`
header, and `queue_rows` is the number of body rows actually rendered
in Up Next (track rows or 1 for a placeholder). When Up Next is
suppressed (see 3.5), the body height is the existing
`max(art_height, text_height)`.

### 3.2 Up Next block

The Up Next block is appended to the right column. It is composed of:

- A faint header line: `─ Up Next ─` repeated to the right column width.
- One of the following bodies:
  - Track rows: `N. Title — Artist`, one per upcoming track, truncated
    with ellipsis to the right column width.
  - A single faint placeholder line (for non-renderable states — see
    3.4).

Up Next is rendered only when **all** of these hold:

1. `m.state` is `Connected`.
2. The Now Playing panel is in art-side-by-side mode
   (`width ≥ artLayoutThreshold` and art is non-empty).
3. `art_height − text_height − 1 ≥ 1`, where `text_height` is the
   rendered height of the title/artist/album/progress/volume block and
   the `−1` accounts for the header row. The remaining count is the
   number of body rows available for either the track list or a single
   placeholder line.

If conditions 1–3 don't hold, the right column is unchanged from today
and the existing vertical centering against the art is preserved.
Center-vs-top alignment thus differs by mode — that's fine; alignment
switches on whether Up Next is being rendered.

### 3.3 Refresh and cache

Up Next is **derived state** on each render — there is no new persistent
field on `Model`. The render path computes:

1. If `m.state` is not `Connected`, no Up Next.
2. If `Connected.Now.ShuffleEnabled` is true → "shuffling" placeholder.
3. If `Connected.Now.CurrentPlaylistName == ""` → "no queue" placeholder.
4. Look up `m.playlists.tracksByName[CurrentPlaylistName]`.
   - If absent and `m.playlists.fetchingFor[CurrentPlaylistName]` is
     false: not the render's job to dispatch the fetch — see below for
     the dispatch site. Render "loading…" placeholder.
   - If absent and currently fetching: render "loading…" placeholder.
   - If present but `trackErrByName[CurrentPlaylistName] != nil`: render
     "no queue" placeholder. (We don't surface the error inside Now
     Playing — it's already shown when the user focuses Main on that
     playlist.)
5. With cached tracks, find the index `i` such that
   `tracks[i].PersistentID == Connected.Now.Track.PersistentID`.
   - If not found: "no queue" placeholder. (Possible when the playing
     track was added to the playlist after the cache was populated, or
     when the playing track is from outside the named playlist.)
   - If `i == len(tracks) − 1`: "end of playlist" placeholder.
   - Otherwise: render rows from `tracks[i+1:]`, capped at the available
     row count.

The fetch dispatch lives in the `update` handler for `statusMsg` (or
whatever message type carries Status results today). After the existing
Now-Playing assignment, when `Connected.Now.CurrentPlaylistName != ""`
and that playlist is neither cached nor currently being fetched, the
handler returns `fetchPlaylistTracks(m.client, name)` as part of its
result Cmd. This reuses the same machinery as the eager-load path. A
playlist-name change between two consecutive `statusMsg` ticks is the
trigger to re-evaluate; an unchanged name avoids a refetch.

The existing eager-load behaviour (prefetching the *first* playlist's
tracks) is unchanged. In the common case the playing playlist is the
first playlist (Liked Songs by default), so the cache is already warm
on the first tick; in the uncommon case the playing playlist is
different, the dispatch above kicks in within one status tick (~1s).

### 3.4 Placeholder copy

Each placeholder is rendered as a single faint line where the track
rows would otherwise go. Header is still drawn.

| Condition                                  | Placeholder text                       |
|--------------------------------------------|----------------------------------------|
| `ShuffleEnabled == true`                   | `shuffling — next track unpredictable` |
| `CurrentPlaylistName == ""`                | `no queue`                             |
| Cache miss, fetch in flight                | `loading…`                             |
| Cache hit but `trackErrByName != nil`      | `no queue`                             |
| Cache hit, current track ID not in tracks  | `no queue`                             |
| Cache hit, current track is the last entry | `end of playlist`                      |

Two distinct conditions both render `no queue` (no-context vs
not-found). This is intentional — both mean "we cannot derive a queue
for this state" from the user's perspective.

### 3.5 Width and height fallbacks

- `width < artLayoutThreshold` (currently 70): Up Next is skipped
  entirely. The Now Playing panel renders today's text-only body. This
  is the "narrow mode" path called out in §2.
- Art-side-by-side mode but `art_height − text_height − 1 < 1`: Up
  Next is skipped (no row available even for the header + a single
  placeholder line). The right column doesn't grow beyond what the
  art can visually anchor against, and we don't push the panel
  taller for the queue alone.

In both cases the alignment stays as-is (centered against the art).

## 4. Implementation outline

### 4.1 AppleScript — `internal/music/applescript/scripts.go`

Extend `scriptStatus` to return 10 newline-separated fields (was 7):

```applescript
tell application "Music"
    if not running then return "NOT_RUNNING"
    try
        set t to current track
    on error
        return "NO_TRACK"
    end try
    set ttl to (name of t) as text
    set art to (artist of t) as text
    set alb to (album of t) as text
    set pos to (player position as text)
    set dur to (duration of t as text)
    set xstate to (player state as text)
    set vol to (sound volume as text)
    set pid to (persistent ID of t) as text
    set shuf to (shuffle enabled as text)
    set plName to ""
    try
        set plName to (name of current playlist) as text
    on error
        set plName to ""
    end try
    return ttl & linefeed & art & linefeed & alb & linefeed & pos ¬
        & linefeed & dur & linefeed & xstate & linefeed & vol ¬
        & linefeed & pid & linefeed & shuf & linefeed & plName
end tell
```

Notes on the AppleScript itself:

- `persistent ID of t` is documented to return a 16-character hex string;
  the Track type's existing `PersistentID` field uses the same.
- `shuffle enabled` is read off the application; if Music.app
  exposes it only on `current playlist` in this macOS version, the
  implementation will pivot at plan-time. The parsed value is the
  literal string `true` or `false`.
- `name of current playlist` may error when nothing is playing in a
  playlist context (e.g. a track played via `PlayTrack` from search).
  The inner `try` traps that to `""`. The downstream Go code treats
  empty-string as "no queue".
- Library-as-current-playlist: when Music.app reports the master
  Library as the current playlist, `plName` will be the localised
  string `"Library"` (or the user's locale equivalent). The render
  layer treats "Library" identically to non-empty playlist names; if
  the user actually has a custom playlist named Library, that's
  indistinguishable, but it will simply attempt a tracks fetch and
  resolve correctly. We do not special-case the string.

Extend `scriptPlaylistTracks` to append persistent ID as a 5th
tab-separated field per track row:

```applescript
set ln to (name of t) & tab & (artist of t) & tab & (album of t) ¬
    & tab & ((duration of t) as text) & tab & (persistent ID of t)
```

### 4.2 Parser — `internal/music/applescript/parse.go`

- The Status parser learns three new lines (positions 7, 8, 9 zero-indexed).
  Order is fixed: persistent ID, shuffle, playlist name. Validate length
  ≥ 10; older 7-line outputs are rejected (no backward compatibility
  required — this is an internal protocol).
- The PlaylistTracks parser learns a 5th tab-separated field per track
  row. Validate column count ≥ 5; treat missing persistent ID as a
  parse error (Music.app always returns one for library tracks).

### 4.3 Domain — `internal/domain/nowplaying.go`

```go
type NowPlaying struct {
    Track        Track
    Position     time.Duration
    Duration     time.Duration
    IsPlaying    bool
    Volume       int
    LastSyncedAt time.Time

    // Queue context — populated by Status.
    CurrentPlaylistName string
    ShuffleEnabled      bool
}
```

The persistent ID parsed from line 7 of the Status output goes into
`Track.PersistentID`; no second field on `NowPlaying`. Update the
`Track.PersistentID` doc comment: was "populated by search results;
left empty elsewhere", becomes "populated by search results, playlist
tracks, and the now-playing track. Apple Music's stable per-library
track handle, used by PlayTrack and to locate the playing track inside
its playlist for the Up Next view."

### 4.4 Model and update — `internal/app/model.go`, `internal/app/update.go`

No new state struct fields. The dispatch site is the existing
`statusMsg` (or equivalent) handler in `update.go`:

```go
// After the existing assignment of the new Connected.Now state:
if connected, ok := m.state.(Connected); ok {
    name := connected.Now.CurrentPlaylistName
    if name != "" {
        _, cached := m.playlists.tracksByName[name]
        if !cached && !m.playlists.fetchingFor[name] {
            m.playlists.fetchingFor[name] = true
            return m, fetchPlaylistTracks(m.client, name)
        }
    }
}
```

Exact integration with any existing return/cmd at the end of the handler
is an implementation detail for the plan. Invariant: a single Cmd is
returned per message; the queue-prefetch fires at most once per
playlist-name change; the existing eager-load and Main-cursor prefetches
are unchanged.

### 4.5 Rendering — `internal/app/panel_now_playing.go`

`renderConnectedCardOnly` is restructured:

```go
func renderConnectedCardOnly(s Connected, art string, width int, m Model) string {
    text := buildNowPlayingText(s) // title/artist/album/progress/volume — same content as today
    if width < artLayoutThreshold || art == "" {
        return text
    }
    artHeight := lipgloss.Height(art)
    textHeight := lipgloss.Height(text)
    queueRows := artHeight - textHeight - 1 // -1 for the "─ Up Next ─" header
    upNext := renderUpNext(s.Now, m, queueRows, rightColumnWidth(width, art))
    if upNext == "" {
        // No room or not applicable — fall back to today's centered layout.
        return lipgloss.JoinHorizontal(lipgloss.Center, art, "  ", text)
    }
    rightCol := lipgloss.JoinVertical(lipgloss.Left, text, upNext)
    return lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", rightCol)
}
```

The `renderUpNext(now domain.NowPlaying, m Model, rows int, width int) string`
helper encapsulates the dispatch from §3.3 and returns `""` to signal
"don't render". `m` is needed to access `m.playlists.tracksByName`,
`fetchingFor`, and `trackErrByName`.

`rightColumnWidth(width int, art string) int` returns the panel content
width minus the art width minus the gap (`"  "`), used to truncate
track rows.

The signature change to `renderConnectedCardOnly` (taking `Model`) means
the call site in `renderNowPlayingPanel` passes `m` through. Existing
callers in tests update accordingly.

## 5. Testing

New / changed tests:

- **`internal/music/applescript/parse_test.go`** — add fixtures for the
  10-line `Status` output. One happy path with a playlist + persistent
  ID + shuffle off; one with shuffle on; one with empty playlist name
  (track played from search). Extend the `PlaylistTracks` fixtures with
  the 5th tab-separated persistent-ID column.

- **`internal/app/panel_now_playing_test.go`** — new render cases:
  - **Happy path:** `Connected` with `CurrentPlaylistName == "Liked
    Songs"`, persistent ID matching `tracks[2]`, art height 12, text
    height 7 → assert rendered output contains `─ Up Next ─` and the
    titles of `tracks[3]` and `tracks[4]`.
  - **Shuffle:** `ShuffleEnabled == true` → assert output contains
    `shuffling`.
  - **No playlist context:** `CurrentPlaylistName == ""` → assert
    output contains `no queue`.
  - **Loading:** `CurrentPlaylistName == "Recents"` not in cache and
    `fetchingFor["Recents"] == true` → assert output contains
    `loading…`.
  - **End of playlist:** persistent ID matches the last track → assert
    output contains `end of playlist`.
  - **Track not in playlist:** persistent ID doesn't match any track in
    the cached list → assert output contains `no queue`.
  - **Narrow mode:** `width < artLayoutThreshold` → assert output is
    identical to today's text-only render (no Up Next, no header).
  - **Art shorter than text:** art height ≤ text height → assert no Up
    Next is rendered and the existing centered layout is preserved.

- **`internal/app/update_test.go`** — `statusMsg` handler:
  - Given a `statusMsg` with `CurrentPlaylistName == "Recents"` and
    `tracksByName` empty and `fetchingFor` empty: assert
    `fetchingFor["Recents"] == true` after handling and the returned
    Cmd, when invoked, produces a `playlistTracksMsg` for `"Recents"`.
  - Same setup but `fetchingFor["Recents"] == true`: assert no fetch
    Cmd is returned.
  - Same setup but `tracksByName["Recents"]` already populated: assert
    no fetch Cmd is returned.
  - `CurrentPlaylistName == ""` (or empty): assert no fetch Cmd is
    returned.

- **`internal/music/fake/client.go`** — set the new fields on
  `NowPlaying` returned from the fake's `Status`. Update its tracks to
  carry persistent IDs so existing tests don't have to assemble them
  manually for queue scenarios.

- **`internal/music/applescript/client_integration_test.go`**
  (`//go:build integration`) — extend the existing Status integration
  test (or add one) that, after starting Apple Music and playing a
  known playlist via existing helpers, asserts `Status()` returns a
  non-empty `CurrentPlaylistName` and `Track.PersistentID`. This is the
  validation point for the AppleScript surface assumptions in §4.1.

## 6. Risks and trade-offs

- **AppleScript shuffle surface uncertainty.** `shuffle enabled` may be
  on `application "Music"` (older iTunes carry-over) or on `current
  playlist` (modern Music.app). The implementation plan resolves this
  empirically when the integration test is wired up; the spec is
  written assuming the application-level form. If only the
  per-playlist form works, the AppleScript snippet pivots but no Go
  signature changes are needed.

- **`current playlist` semantics outside playlist contexts.** When
  Music.app falls back to "Library" as the current playlist (e.g. after
  a search-driven `PlayTrack`), we'll attempt to fetch a playlist
  literally named Library via `PlaylistTracks`. The existing
  `PlaylistTracks` script enumerates user and subscription playlists
  and will return an error / not-found, which the render path maps to
  `no queue`. Acceptable. If users complain, we can special-case the
  string at the parse boundary.

- **Per-tick AppleScript cost.** Three additional property reads per
  status tick. Per the MusicKit feasibility doc, AppleScript invocation
  overhead is dominated by the `osascript` fork (~50–100ms cold), not
  per-property cost. Negligible regression.

- **Cache divergence.** The Up Next view reads from
  `tracksByName`, the same cache used by Main panel browsing. If a user
  modifies a playlist in Music.app while goove is running, both views
  go stale together. There is no cache invalidation today and we don't
  add one. Acceptable for v1.

- **Top-vs-center alignment switch.** In art-side-by-side mode, the
  right column was vertically centered against the art and is now
  top-aligned with Up Next anchoring against the bottom of the art.
  Users who expected the title/progress to sit visually centered on
  the art may notice. We considered keeping center alignment and
  inserting Up Next as a separate centered block but that wastes space
  asymmetrically; top alignment is the simpler model and matches the
  filling-the-bottom-blank-space rationale that motivated the feature.

- **Persistent-ID matching cost.** Linear scan over the cached track
  list per render. Playlists are bounded by AppleScript's existing
  enumeration limits; 1k-track playlists are common but not pathological.
  If profiling later shows a hot spot, an `id → index` map can be added
  to the cache.

- **Semantic overlap with Apple Music's own "Up Next"** (the
  user-curated queue from Play Next / Play Later). Calling this view
  "Up Next" risks confusion when our list and Music.app's sidebar
  disagree. Trade-off accepted: it is the natural label and the
  per-playlist meaning is the obvious user expectation in a TUI. A
  follow-up spec for the user-curated queue would unify under a
  "Queue" panel name if pursued.
