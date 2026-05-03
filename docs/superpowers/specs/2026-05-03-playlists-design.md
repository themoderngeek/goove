# Playlists — design spec

**Date:** 2026-05-03
**Status:** Approved, ready for implementation plan
**Predecessors:** `2026-04-30-goove-mvp-design.md`, `2026-05-02-cli-mode-design.md`, `2026-05-02-audio-targets-design.md`

## Goal

Let goove users browse and play their Apple Music playlists from both the CLI and the TUI. Stations are deliberately out of scope for this iteration (see [Out of scope](#out-of-scope)).

## Scope

In scope:
- List **user playlists** and **subscription playlists** (playlists added from Apple Music's catalogue). System playlists, smart playlists, and folder hierarchy are excluded.
- List the tracks of a chosen playlist.
- Start playback of a playlist, optionally from a chosen track.
- A new TUI **browser view** that hosts the above; reachable from now-playing via the `l` keybind.
- New CLI verbs: `goove playlists list | tracks | play` (plus a singular `playlist` alias).

Out of scope (see [Out of scope](#out-of-scope) for the full list and rationale): stations, search, playlist creation/editing, smart playlists, folders, system playlists, album art for playlists, persistent-ID addressing, shuffle/repeat, queue inspection.

## Architectural approach

Extend the existing `music.Client` interface with three methods (Approach 1 from brainstorming). Rejected alternatives: splitting out a separate `Library` interface (creates an awkward seam because `PlayPlaylist` is both a library and a transport concern), and a separate `internal/library` package (overkill for the surface area; would duplicate the AppleScript runner glue).

The layering matches the existing project:

```
TUI (Bubble Tea)  ─┐
                   ├──► music.Client ──► applescript impl ──► osascript
CLI               ─┘                  ╲─► fake impl (tests)
                          ▲
                          │
                  domain (pure types)
```

Frontends remain independent — the CLI does not depend on `internal/app`, and the TUI does not depend on `internal/cli`.

## Domain types

New file `internal/domain/playlist.go`:

```go
type Playlist struct {
    Name       string
    Kind       string // "user" | "subscription"
    TrackCount int
}
```

**Extend the existing `domain.Track`** (defined in `internal/domain/nowplaying.go`) with a `Duration` field:

```go
type Track struct {
    Title    string
    Artist   string
    Album    string
    Duration time.Duration // NEW — populated for playlist tracks; left zero
                           // for NowPlaying.Track (where NowPlaying.Duration
                           // remains the canonical per-playback duration source)
}
```

Reusing the existing type avoids a parallel `PlaylistTrack` shape. `NowPlaying.Duration` stays the canonical playback duration; `Track.Duration` is metadata that happens to be populated only in playlist contexts. `parseStatus` is unchanged (it leaves `Track.Duration` zero); `parsePlaylistTracks` populates it.

No persistent-ID field in v1. Every other piece of the codebase (notably `targets set`) addresses by name with exact-then-substring fallback; mirroring that pattern keeps the cognitive model consistent. If real ambiguity bites in practice, a `PersistentID` field can be added without changing the interface shape.

## Music client interface

Three additions to `music.Client`:

```go
Playlists(ctx context.Context) ([]domain.Playlist, error)
PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error)
PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error
```

`fromTrackIndex` is **0-based** (idiomatic Go). `0` means "play from the start". The CLI converts its 1-based `--track` flag at the boundary.

New sentinel error in `internal/music/client.go`:

```go
var ErrPlaylistNotFound = errors.New("playlist not found")
```

(Mirrors the existing `ErrDeviceNotFound`.)

## AppleScript implementation

Three new scripts in `internal/music/applescript/scripts.go`, following the existing tab-separated / linefeed-delimited convention with `NOT_RUNNING` / `NOT_FOUND` / `OK` sentinel returns.

```
scriptPlaylists       → name\tkind\ttrack_count, one playlist per line.
                        Iterates user playlists then subscription playlists in the
                        same script. Empty list ⇒ empty stdout.

scriptPlaylistTracks  → name\tartist\talbum\tduration_seconds, one track per line.
                        Returns "NOT_FOUND" if no playlist with that exact name exists.
                        %s placeholder for the playlist name.

scriptPlayPlaylist    → "OK" | "NOT_RUNNING" | "NOT_FOUND".
                        From start (Go fromIdx == 0):  play playlist "<name>"
                        Otherwise (Go fromIdx == k > 0):  play track (k+1) of playlist "<name>"
                        (AppleScript track addressing is 1-indexed, so the Go 0-based
                        fromIdx is converted at the script-formatting boundary.)
```

Parsing lives in `parse.go` next to the existing `parseAirPlayDevices`. The same caveats already documented for that parser carry over: tabs and linefeeds in names corrupt parsing, but Apple's UI does not permit either.

**Edge cases handled in the parser:**
- Skip rows with empty names. Music permits a literally-named-`""` playlist, and `play playlist ""` errors at the AppleScript level — pruning these in the parser is the cheapest fix.
- Skip rows with the wrong field count (defensive against unforeseen Music updates).

**Error mapping in the client:**
- stdout `NOT_RUNNING` → `music.ErrNotRunning`
- stdout `NOT_FOUND` → `music.ErrPlaylistNotFound`
- osascript stderr matching the existing permission detector → `music.ErrPermission`
- Otherwise wrap raw

## CLI surface

Two-level dispatcher under `playlists`, mirroring `targets`:

```
goove playlists list [--json]
    Lists user + subscription playlists.
    Plain: "<name>  (<kind>, <n> tracks)" per line.
    JSON:  array of {name, kind, track_count}.

goove playlists tracks <name> [--json]
    Lists tracks of the matched playlist.
    Plain: "<n>. <title> — <artist>  (<album>)  [<m:ss>]"  (1-indexed).
    JSON:  array of {index, title, artist, album, duration_sec}
           (matches the existing `goove status --json` track shape).

goove playlists play <name> [--track N]
    Starts playback of matched playlist.
    --track is 1-indexed; omitted = start from track 1.
    Silent on success.

goove playlists help, --help, -h
    Subcommand help.
```

A singular alias `goove playlist <subcommand>` dispatches to the same handlers — `goove playlist play "X"` reads more naturally than the plural form.

**Name resolution** reuses the `targets set` pattern: exact match wins; case-insensitive substring match; multiple matches → list candidates and exit 1; zero matches → "playlist not found" exit 1. If a third caller of this pattern appears later, factor it into a helper; for now duplicate it.

**Top-level `usageText` in `cli.go`** gains one line:

```
goove playlists list|tracks|play [args]   List/inspect/play playlists
```

**Exit codes** match the existing CLI convention: 0 success; 1 generic / not-found / ambiguous / out-of-range; 2 permission denied.

**Error messages:**
- `goove: playlist not found: <name>` (exit 1)
- `goove: %q matches multiple playlists:` followed by candidates (exit 1)
- `goove: playlist has no tracks: <name>` (exit 1, on `play` only)
- `goove: track index out of range: N (playlist has M tracks)` (exit 1)

## TUI browser view

### State machine

The browser **replaces** the now-playing card rather than overlaying it, so it warrants its own mode rather than reusing the picker's nil-pointer pattern.

```go
type viewMode int

const (
    modeNowPlaying viewMode = iota
    modeBrowser
)

type browserPane int

const (
    leftPane  browserPane = iota // playlists
    rightPane                    // tracks
)

type browserState struct {
    pane           browserPane
    playlists      []domain.Playlist
    playlistCursor int
    loadingLists   bool
    tracks         []domain.Track  // tracks of the playlist at playlistCursor
    trackCursor    int
    loadingTracks  bool
    err            error
}
```

`Model` gains `mode viewMode` and `browser *browserState`. The picker overlay remains orthogonal — it can appear over either mode.

### Layout

Two columns, fills the terminal:

```
┌─ goove · browser ────────────────────────────────────┐
│                                                      │
│ Playlists           │ Tracks — Liked Songs           │
│ ▸ Liked Songs       │   1. Stairway to Heaven        │
│   90s Mix           │   2. Black Dog                 │
│   Chill Evening     │ ▸ 3. Going to California       │
│   Workout (sub)     │   4. When the Levee Breaks     │
│                     │   …                            │
│                                                      │
└──────────────────────────────────────────────────────┘
 ↑↓: nav   tab: pane   ⏎: play   esc: back   space: ⏯
```

The focused pane is indicated by the `▸` cursor and a brighter accent on the column header. Long lists scroll via window-clamping around the cursor — no viewport library yet.

A new file `internal/app/browser.go` houses browser state, key handling, and rendering. `update.go` and `view.go` only gain mode-dispatch wrappers, keeping them from ballooning. `model.go` grows by a handful of lines.

This file boundary also makes future UI revamps cheaper: the browser is a self-contained unit that can be reshaped (or replaced) without disturbing the now-playing surface.

### Keybinds in browser mode

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move cursor in focused pane |
| `tab` / `→` `←` | Switch focused pane (also triggers track fetch on first focus of the right pane) |
| `enter` | Left pane: play whole playlist. Right pane: play playlist starting from highlighted track. |
| `r` | Refetch the focused pane's data (left → playlists; right → tracks) |
| `esc` | Return to now-playing |
| `l` | No-op (already in browser) |

**Transport keys remain live in browser mode**: `space` (play/pause), `n`, `p`, `+/-` work everywhere. `q` quits everywhere. Rationale: skipping a track or pausing while browsing is a natural action; forcing the user to leave the browser would be friction.

### Lazy fetching

Fetching is **on-demand**, not eager:

- Pressing `l` from now-playing → switch mode immediately, set `loadingLists: true`, dispatch `fetchPlaylists`.
- Moving the cursor in the left pane does **not** fetch tracks. Spamming `osascript` on every arrow press would be wasteful.
- Tracks fetch **only** when the user focuses the right pane (`tab` or `→`). This is the single trigger.
- Pressing `enter` on the left pane plays the playlist but does **not** fetch its tracks — playback and browsing are independent. If the user wants to see what's in the playlist they just started, they `tab` over.
- If the user has already focused the right pane for a playlist, then moves the left-pane cursor to a different playlist, the right pane shows the *previously fetched* tracks until the user re-focuses the right pane (which triggers a fresh fetch for the new selection).

This is a deliberate trade-off against eager-on-cursor-change. The chosen behaviour favours fewer shell-outs at the cost of one extra keystroke to peek at tracks.

### Messages and commands

New file `internal/app/browser.go` (or additions to `messages.go`):

```go
type playlistsMsg     struct { playlists []domain.Playlist; err error }
type playlistTracksMsg struct { name string; tracks []domain.Track; err error }
type playPlaylistMsg   struct { err error }

func fetchPlaylists(c music.Client) tea.Cmd
func fetchPlaylistTracks(c music.Client, name string) tea.Cmd
func playPlaylist(c music.Client, name string, fromIdx int) tea.Cmd
```

After a successful `playPlaylistMsg`, the existing 1Hz status tick will surface the new now-playing in its next poll — no special wiring needed.

### Now-playing keybind addition

In `modeNowPlaying`, pressing `l` opens the browser. The keybind footer in the now-playing view gains `l: browse`.

## Refresh model

- **Status polling unchanged** — the existing 1Hz status tick keeps now-playing live in both modes.
- **Playlist data is fetched on demand**, never polled. Playlists rarely change.
- **`r` in the browser** is the user's escape hatch when they've added a playlist in Music.app and want it to appear.

## Error handling

| Condition | CLI | TUI |
|---|---|---|
| Music not running | `goove: Apple Music isn't running (run 'goove launch' first)` exit 1 | Existing `Disconnected` screen via the next status tick |
| Permission denied | Existing message, exit 2 | Existing sticky `permissionDenied` screen |
| Playlist not found | `goove: playlist not found: <name>` exit 1 | `browserState.err` shown in the affected pane |
| Ambiguous match (CLI only) | List candidates, exit 1 | N/A — TUI selects by row |
| Empty playlist | `tracks` lists nothing; `play` errors `goove: playlist has no tracks: <name>` exit 1 | Right pane shows "(no tracks)"; `enter` is a no-op |
| `--track N` out of range | `goove: track index out of range: N (playlist has M tracks)` exit 1 | Cursor is clamped in TUI so this can't occur |

## Testing strategy

| Layer | What's tested | How |
|---|---|---|
| `internal/music/applescript/parse_test.go` | `parsePlaylists`, `parsePlaylistTracks` | Table tests with golden tab-separated input. Cases: empty input, single row, multi-row, empty-name skip, malformed-row skip |
| `internal/music/applescript/client_test.go` | New methods' error mapping | Stub the runner; assert `NOT_RUNNING` → `ErrNotRunning`, `NOT_FOUND` → `ErrPlaylistNotFound`, permission stderr → `ErrPermission` |
| `internal/music/applescript/client_integration_test.go` | Real osascript hit | `-tags=integration`. Read-only assertions only — list playlists, assert ≥0 returned without error. **Does not** trigger playback (too disruptive in an integration suite) |
| `internal/music/fake/client_test.go` | Fake playlist behaviour + counters | Pre-seed two playlists, three tracks each. Assert `PlayPlaylist` increments a counter and records `(name, fromIdx)` |
| `internal/cli/cli_test.go` | `playlists list/tracks/play`, name resolution, `--json`, exit codes, singular alias | Drive through `Run()` against the fake client; assert stdout, stderr, exit code |
| `internal/app/update_test.go` | Browser state transitions | `KeyMsg{Runes: 'l'}` from now-playing → mode = browser + fetch cmd. `playlistsMsg` → state populated, loading cleared. `tab` switches pane → triggers track fetch on first focus. `enter` on right pane → `playPlaylist` cmd issued with correct 0-based index. `esc` returns to now-playing. Transport keys (`space`, `n`, `p`, `+`, `-`) still dispatch in browser mode |

View rendering remains unit-test-free, consistent with the existing `view.go`. Browser layout is exercised by hand. Adding snapshot testing as a new pattern is out of scope for this spec.

## Out of scope

Deferred to a later spec, with reasoning:

- **Stations.** AppleScript's `radio tuner playlists` predates modern Apple Music personalised stations (Chill Mix, Get Up! Mix, Apple Music 1, …) and returns 0 items on a typical modern setup. The feature loses nothing meaningful by deferring; revisit when/if we move past AppleScript (CGo + MediaRemote, or MusicKit).
- **Search.** Natural next step. The browser view's two-column shape is intentionally compatible with a future search input.
- **Playlist creation, editing, deletion.** Read-only for v1.
- **Smart playlists, folders, system playlists ("Library", "Music", "Downloaded Music").** Excluded by the chosen scope (user + subscription only).
- **Album art for playlists.** AppleScript exposes it, but the existing art pipeline is per-track. Out of scope.
- **Persistent-ID addressing.** Name-based for v1; revisit if collisions bite.
- **Auto-refresh on external mutation.** No polling for playlist list changes; user presses `r` in the browser.
- **Shuffle / repeat toggles tied to playlist start.** `play` always plays in order.
- **Queue inspection / "play next".** Separate concern.

Deliberately *not* implemented even though they would be small additions, to keep this spec focused:

- **"Currently playing playlist" indicator** in the browser. AppleScript can tell us the current playlist; we don't surface it. Easy follow-up if it feels missing.
- **"Jump to current track's playlist" shortcut.** Same reasoning.

## Future-work breadcrumbs

The browser view is structured (`internal/app/browser.go` as a self-contained file, browser-only state under `browserState`) so that the natural next features can be added without restructuring:

- **Search** slots in as a third pane / overlay on the browser, hitting a future `music.Client.Search` method.
- **Stations**, when the underlying integration changes, become a third row in the left pane (Playlists / Stations / Search results) or a sibling mode.
- **Currently-playing-playlist indicator** is a small accent on the matching row in the left pane.
