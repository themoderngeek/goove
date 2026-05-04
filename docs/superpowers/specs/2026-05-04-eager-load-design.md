# goove — eager-load Playlists, Output devices, and first playlist's tracks on startup

**Date:** 2026-05-04
**Status:** Draft, awaiting review
**Predecessors:** `2026-05-04-tui-overhaul-design.md`

## 1. Summary

Today the Playlists panel and Output panel fetch their data lazily, on the
user's first focus of each panel (`onFocusPlaylists`, `onFocusOutput`). On
launch all three left-column panels appear empty until the user tabs to them,
and the main pane stays empty until the user moves the Playlists cursor for
the first time.

This change makes the TUI eager-load on startup:

- The list of playlists is fetched in `Init`.
- The list of AirPlay output devices is fetched in `Init`.
- When the playlist list arrives, the first playlist's tracks are fetched
  immediately so the main pane shows content from frame zero.

The lazy on-focus loaders stay in place untouched. They become a natural
retry path: if a startup fetch fails (Apple Music not running yet, transient
AppleScript error), focusing the panel later re-triggers the fetch.

The change is contained to `internal/app/model.go` and
`internal/app/update.go`. No new state, no new message types, no UI changes.

## 2. Scope and non-goals

### In scope

- Add `fetchPlaylists` and `fetchDevices` to the `tea.Batch` returned by
  `Model.Init`.
- Initialise `playlists.loading = true` and `output.loading = true` in `New`
  so the panels render `loading…` from frame zero.
- In the `playlistsMsg` handler, when items arrive and
  `main.selectedPlaylist == ""`, set `main.selectedPlaylist` to the first
  item's name and fire `fetchPlaylistTracks` for it.
- Tests for the new `Init` Cmd shape, the `New` loading flags, and the
  `playlistsMsg` first-item prefetch (including the no-clobber guard).

### Out of scope

- A refresh keybinding. Lazy on-focus retry is the only refresh mechanism;
  this is unchanged.
- Status-gated prefetching. Eager fetches fire unconditionally; on
  `Disconnected` they will fail and the lazy on-focus path will retry.
- Prefetching anything else (search history, recently-played, etc.).
- UI / layout changes. The `loading…` placeholder is already wired up.

## 3. Behaviour

### 3.1 Startup sequence

`Init` returns a `tea.Batch` containing:

- `fetchStatus(client)` — existing.
- `scheduleStatusTick()` — existing.
- `scheduleRepaintTick()` — existing.
- `fetchPlaylists(client)` — **new**.
- `fetchDevices(client)` — **new**.

The three fetches run concurrently. Order of arrival is not significant: each
message updates its own panel's state independently.

`New` additionally sets:

```go
playlists: playlistsPanel{
    ...,
    loading: true,
},
output: outputPanel{
    loading: true,
},
```

so the first frame rendered shows `loading…` in both panels rather than
`(no playlists)` / `(no devices)`.

### 3.2 First-playlist track prefetch

When `playlistsMsg` arrives at the update loop, after the existing
`m.playlists.items = msg.playlists` and `m.playlists.loading = false`
assignments (and the existing cursor-clamp), the handler additionally:

1. Returns immediately on empty results (no items → nothing to prefetch).
2. If `m.main.selectedPlaylist == ""`, sets it to `m.playlists.items[0].Name`
   and returns `fetchPlaylistTracks(m.client, items[0].Name)` as the result
   Cmd.
3. If `m.main.selectedPlaylist != ""` (the user has already moved the
   Playlists cursor before the list arrived — a fast-user race), the handler
   leaves `selectedPlaylist` alone and does **not** fire a track fetch. The
   user's choice wins.

This is essentially the body of `onPlaylistsCursorChanged` minus the
debounce: we want the prefetch to fire immediately, not via a tick.

### 3.3 Failure paths

If `fetchPlaylists` returns an error, the existing handler sets
`loading = false` and `items` stays empty. `onFocusPlaylists` then sees
`len(items) == 0 && !loading` on the next focus and re-fires the fetch.
The same is true for `fetchDevices` / `onFocusOutput`.

If `fetchPlaylistTracks` returns an error for the auto-prefetched first
playlist, the existing per-playlist error map (`trackErrByName`) records it,
the main pane shows the error in its current style, and the user can move
the Playlists cursor to re-attempt or pick another playlist.

### 3.4 Race: user tabs to Playlists before list arrives

`loading == true` is set in `New`. When `onFocusPlaylists` runs its
short-circuit check (`len(items) > 0 || loading`), `loading` is true, so it
returns no Cmd. No duplicate fetch fires.

## 4. Implementation outline

### 4.1 `internal/app/model.go`

```go
func New(client music.Client, renderer art.Renderer) Model {
    return Model{
        ...
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

### 4.2 `internal/app/update.go`

In the `playlistsMsg` arm, after the existing
`m.playlists.items = msg.playlists` / cursor-clamp / error-handling
assignments (which already exist and stay as-is), add:

```go
if len(m.playlists.items) > 0 && m.main.selectedPlaylist == "" {
    name := m.playlists.items[0].Name
    m.main.selectedPlaylist = name
    return m, fetchPlaylistTracks(m.client, name)
}
```

The exact placement and any merge with an existing return statement is an
implementation detail for the plan; the invariant is: a single Cmd is
returned, and the user's `selectedPlaylist` choice is never overwritten.

## 5. Testing

New / changed tests in `internal/app/`:

- **`panel_playlists_test.go`** / **`panel_output_test.go`**: assert that
  `New(...)` produces a model with `playlists.loading == true` and
  `output.loading == true` respectively. (There's no `model_test.go` today;
  the panel-scoped test files are the natural home.)
- **Init test**: invoke `Model.Init()`, run each returned Cmd, and assert
  the produced messages include a `playlistsMsg` and a `devicesMsg`
  (in addition to the existing status / tick messages). Use a stub
  `music.Client` that returns deterministic playlists and devices.
- **`update_test.go`** — `playlistsMsg` first-item prefetch:
  - Given a model with `main.selectedPlaylist == ""`, dispatch a
    `playlistsMsg` with two items. Assert
    `m.main.selectedPlaylist == items[0].Name` and the returned Cmd, when
    invoked, produces a `playlistTracksMsg` for `items[0].Name`.
  - Given a model with `main.selectedPlaylist == "Other"`, dispatch the
    same message. Assert `selectedPlaylist` is still `"Other"` and the
    returned Cmd does **not** produce a `playlistTracksMsg`.
  - Given an empty `playlistsMsg`, assert no track-prefetch Cmd is
    returned.

The existing `onFocusPlaylists` / `onFocusOutput` tests already cover the
retry-on-empty path; no new tests are needed there.

## 6. Risks and trade-offs

- **Wasted AppleScript calls when Music isn't running.** On a cold launch
  with Music quit, the eager fetches will fire and fail. The cost is two
  failed AppleScript invocations per launch — small, and equivalent to the
  cost a user would pay anyway the first time they tab to those panels. We
  considered status-gating the prefetches (Approach 3 in brainstorming) and
  rejected it as over-engineered for the win.
- **Auto-selection of the first playlist may surprise users who expected
  the main pane to start blank.** This is consistent with the README's
  startup illustration (Liked Songs already showing tracks) and with the
  general LazyGit-style pattern of "panels are populated, not empty".
