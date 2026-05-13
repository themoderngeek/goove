# goove — Queue management UI

**Date:** 2026-05-13
**Status:** Draft, awaiting review
**Predecessors:** `2026-05-04-queue-design.md` (read-only Up Next), `2026-05-04-tui-overhaul-design.md`, `2026-05-03-playlists-design.md`, `2026-05-03-goove-search-design.md`

## 1. Summary

Layer a goove-owned interactive queue on top of the existing read-only Up Next
view. Tracks enqueued with `a` from any Main panel track row play after the
currently-playing track ends; once the queue drains, playback resumes the
playlist that was interrupted (Spotify-style). Inspection and editing happen
in a full-screen overlay opened with `Q`; the existing Up Next stays as the
always-on teaser in the Now Playing panel, now showing `★` queued rows above
the playlist tail rows.

The queue is single-priority FIFO and in-memory only — it does not persist
across goove restarts. Playback handoff uses a "react on track-change tick"
strategy: when the status poller sees the current track change to something
that isn't the queue head, goove calls `PlayTrack(queueHead)` and pops. When
the queue is empty and a resume context is set, goove calls
`PlayPlaylist(name, --track index)` to hand control back. The trade-off is a
brief audio glitch (~0.5–1s of Music.app's "natural next" track before
override); accepted for V1.

## 2. Scope and non-goals

### In scope

- `QueueState` in `internal/app/` — a FIFO `[]domain.Track` with `Add`,
  `RemoveAt`, `MoveUp`, `MoveDown`, `PopHead`, `Clear`.
- `ResumeContext` (playlist name + 1-based next track index) captured on
  first handoff, cleared on resume dispatch.
- Status-tick handler dispatches `PlayTrack(queue head)` on track change and
  `PlayPlaylist(resume.PlaylistName, resume.NextIndex)` on queue drain.
- Global key `a` — add the focused Main panel track row (playlist track list
  or search results) to the queue tail. No-op on other focus targets.
- Global key `Q` — open the queue overlay from any focus.
- Global key `n` — when queue is non-empty, pop head and `PlayTrack(head)`
  instead of calling `Next()`. When empty, today's `Next()` behaviour.
- Queue overlay panel — modal, full-area, intercepts all keys, with cursor,
  remove (`x`), reorder (`K`/`J`), jump-to-track (`Enter` plays the selected
  item now; remaining items play next in their existing order — see §3.4),
  clear with confirmation (`c` then `y`), close (`Esc` or `Q`). Quit (`q`)
  is suppressed while open.
- Up Next teaser updated: queued rows first (`★` prefix), then existing
  playlist tail rows. Queue rows take priority when vertical room is tight.
- Hints bar adds `a:queue Q:queue-view`. Hidden while overlay is open
  (overlay has its own help row).
- Tests: queue unit tests, resume state-machine tests, overlay render and
  key handler tests, teaser merge-render tests, update wiring tests,
  fake-client call recording, one integration test for the live handoff.

### Out of scope (V1, follow-ups)

- Anticipated handoff (no audio glitch). Revisit if A's glitch annoys users.
- Persistence across goove restart (queue or resume context).
- Two-priority queue (Play Next vs Play Later — Apple Music model).
- Bulk-enqueue a whole playlist (`a` on the Playlists sidebar).
- Already-played history above the current track in the overlay.
- Time-until-track / total queue duration metadata.
- `goove queue` CLI verbs (`list`, `add`, `clear`). Deferred until queue
  has lifetime longer than a single TUI process (persistence or IPC). The
  CLI parser will accept `goove queue` and print a short help message
  pointing at the TUI keys, but no other behaviour.
- Cross-library search-and-enqueue from inside the overlay (i.e., no
  "search from within the queue view"). Adds are always from the Main
  panel's existing surfaces.

## 3. Behaviour

### 3.1 Data model

Three new pieces of state on the app `Model`:

```go
// internal/app/queue.go
type QueueState struct {
    Items []domain.Track // FIFO; head is index 0
}

// internal/app/resume.go
type ResumeContext struct {
    PlaylistName string // empty = no resume target (drain ends in silence)
    NextIndex    int    // 1-based; argument to PlayPlaylist's fromTrackIndex
}

// internal/app/model.go (additions)
type Model struct {
    // ...existing fields...
    queue         QueueState
    resume        ResumeContext
    lastTrackPID  string        // persistent ID seen on previous status tick
    lastPlaylist  string        // playlist name seen on previous status tick
    lastTrackIdx  int           // 1-based index of last-seen track within lastPlaylist; 0 if unknown
    overlay       overlayState  // open/closed + cursor index + pending-clear flag
    clearPrompt   bool          // true while waiting for y/n after `c` in overlay
}
```

The `lastPlaylist` / `lastTrackIdx` fields are derived once per tick from
`Connected.Now.CurrentPlaylistName` and a lookup of the current PID in the
cached playlist tracks (same lookup the existing Up Next teaser already does).
They're cached on `Model` because the handoff handler needs the *previous*
tick's values to capture the resume context — what would have been next had
goove not intervened.

### 3.2 Handoff state machine

The handoff handler runs on every status tick, immediately after `Model` is
updated with the new `Connected.Now` state and before `Model` is returned.
Pseudocode:

```
on statusTick(now):
    newPID = now.Track.PersistentID
    if newPID == lastTrackPID:
        return  // no track change
    prevPID = lastTrackPID
    lastTrackPID = newPID
    // (lastPlaylist / lastTrackIdx are updated by a separate helper using
    // the current tick's values, but the *previous* values are what we
    // need for resume capture — see §3.3.)

    if len(queue.Items) == 0:
        if resume.PlaylistName != "":
            dispatch PlayPlaylist(resume.PlaylistName, resume.NextIndex)
            resume = ResumeContext{}
        return

    if newPID == queue.Items[0].PersistentID:
        // We caused this transition on a previous tick; just pop.
        queue.PopHead()
        return

    // Track changed to something other than the queue head, and the queue
    // is non-empty. Intercept.
    if resume.PlaylistName == "" and prevPlaylist != "" and prevIdx > 0:
        resume = ResumeContext{
            PlaylistName: prevPlaylist,
            NextIndex:    prevIdx + 1,
        }
    dispatch PlayTrack(queue.Items[0].PersistentID)
    queue.PopHead()
```

Where `prevPlaylist` / `prevIdx` are the previous tick's cached values
(captured before the per-tick helper overwrites them).

Key invariants:

- The resume context is captured **only when empty** at intercept time. This
  prevents a multi-track queue run from overwriting itself between pops.
- Capture uses the **previous** tick's playlist/index, because the current
  tick already reflects whatever Music.app moved to (often the playlist's
  natural next track, which is what we're about to override).
- Pop happens whether we dispatched a `PlayTrack` (intercept) or simply
  recognised our own previous handoff has landed (pop-on-match).
- If queue drains and `resume` is empty, no dispatch — playback stays where
  Music.app put it (typically silence after the last queued track, since
  PlayTrack puts Music.app in a no-playlist context).

### 3.3 Per-tick `lastPlaylist` / `lastTrackIdx` capture

The handoff handler reads the **previous** tick's values for resume capture,
then a separate helper overwrites them with the **current** tick's values.
This is split into two operations to keep the handler's read-then-write
ordering explicit:

```
on statusTick(now):
    prevPlaylist = lastPlaylist
    prevIdx      = lastTrackIdx

    // ... handoff handler runs here, using prevPlaylist/prevIdx ...

    // After handoff, refresh the cache for next tick.
    lastPlaylist = now.CurrentPlaylistName
    lastTrackIdx = indexOfPID(now.Track.PersistentID, tracksByName[now.CurrentPlaylistName])
                   // 0 if not found or playlist not cached
```

`indexOfPID` is a linear scan over the cached playlist's tracks (same access
pattern as the existing Up Next teaser). When the playlist isn't cached or
the track isn't in it (e.g., during the brief window after handoff when
goove's queue track is playing and no playlist applies), `lastTrackIdx`
becomes 0 and `lastPlaylist` may be empty or "Library" — that's fine, the
handoff handler only captures resume when both are valid (`prevPlaylist != ""
&& prevIdx > 0`).

The very first status tick after launch has `lastTrackPID == ""`, so any PID
is a "change" — but with empty `prevPlaylist`/`prevIdx`, no resume context
gets captured. If the queue is empty (the normal launch state), no dispatch
either. First tick is a no-op as far as handoff is concerned.

### 3.4 Overlay interaction model

While `overlay.open == true`:

- The View renders the overlay over the four-panel layout (rendered at
  panel area, ignoring the normal panels' contents — they're not visible).
- The Update function checks `overlay.open` first and routes all key
  messages to the overlay's handler. Globals (space, n, p, +, -, q,
  Tab/Shift-Tab, digit jumps, `/`, `o`) are not consulted.
- The status tick continues to fire and the handoff handler runs as normal
  — the overlay is a view-layer concept, not a pause.

Overlay keys:

| key            | action                                                   |
|----------------|----------------------------------------------------------|
| `j` / `↓`      | cursor down (clamped to last item)                       |
| `k` / `↑`      | cursor up (clamped to 0)                                 |
| `Enter`        | dispatch `PlayTrack(items[cursor].PersistentID)`, **remove** the selected item from the queue (it's now playing — no longer "up next"), and set `pendingJumpPID` to the played item's PID. Items above the cursor remain at the head and will play next. Cursor stays on the same numeric index, clamped to `len(Items)-1` (so after removing the last item, cursor lands on the new last). See the worked example below for why `pendingJumpPID` is needed. |
| `x`            | `queue.RemoveAt(cursor)`. Cursor clamped to new length−1 if it overshoots. |
| `K`            | `queue.MoveUp(cursor)`; cursor follows the moved item.   |
| `J`            | `queue.MoveDown(cursor)`; cursor follows the moved item. |
| `c`            | set `clearPrompt = true`. Bottom help row swaps to the warning prompt. |
| `y` (when `clearPrompt`) | `queue.Clear()`. `clearPrompt = false`.        |
| any other key (when `clearPrompt`) | `clearPrompt = false`. No clear.     |
| `Esc` / `Q`    | close overlay (`overlay.open = false`, `clearPrompt = false`). |

The `Enter` semantic needs careful handling. Worked example: queue is `[A,
B, C, D]`; user puts the cursor on `C` and presses `Enter`. Desired
sequence of audio: `C` plays now, then when `C` ends `A` plays, then `B`,
then `D`, then resume the playlist.

Naive implementation (just dispatch `PlayTrack(C)`) fails because the
handoff handler on the next status tick would see "new PID is `C`, queue
head is `A`, `A` ≠ `C`, intercept" — and immediately dispatch
`PlayTrack(A)`, cutting off `C`.

The fix is a one-shot `pendingJumpPID` flag on `Model`. `Enter` sets it to
the PID of the played item; the handoff handler recognises that PID as
"goove caused this transition for a jump, not an intercept" and skips the
intercept branch:

```
on statusTick(now):
    newPID = now.Track.PersistentID
    if newPID == lastTrackPID: return
    prevPID = lastTrackPID
    lastTrackPID = newPID

    if newPID == pendingJumpPID:
        pendingJumpPID = ""
        // Capture resume so A, B, D will eventually fall back to playlist.
        if resume.PlaylistName == "" and prevPlaylist != "" and prevIdx > 0:
            resume = ResumeContext{PlaylistName: prevPlaylist, NextIndex: prevIdx + 1}
        return  // don't pop, don't intercept

    // ...remainder of §3.2...
```

The Enter handler:

```
on Enter in overlay:
    item = queue.Items[cursor]
    queue.RemoveAt(cursor)  // remove from queue; it's now playing
    pendingJumpPID = item.PersistentID
    dispatch PlayTrack(item.PersistentID)
    // cursor stays on the same index, clamped
```

When the user-jumped track (C) ends naturally, handoff sees queue head A ≠
new PID, intercepts → A plays. When A ends, B intercepts. When B ends, D
intercepts. When D ends, resume context (captured at the Enter dispatch
tick) fires, returning to playlist.

### 3.5 Up Next teaser changes

`renderUpNext` in `internal/app/panel_now_playing.go` gains a `queue
[]domain.Track` parameter (passed by the call site from `m.queue.Items`).
The render order, top-to-bottom in the right column, becomes:

1. `─ Up Next ─` header (existing).
2. Queue rows: `★ Title — Artist` per `queue.Items[i]`, truncated.
3. Playlist tail rows: `N. Title — Artist` for `tracks[i+1:]` where `i` is
   the index of the currently-playing track in the cached playlist
   (existing logic).

Total rows are capped at the available `queueRows` (today: `art_height -
text_height - 1`). Queue rows take priority — if the cap is 5 and the queue
has 7 items, only the first 5 queue items render and no playlist tail rows
are shown. If the queue has 2 and the cap is 5, both queue rows render
followed by 3 playlist tail rows.

Placeholders interact with queue rendering as follows:

| Condition (existing) | Behaviour with queue rows |
|---|---|
| `ShuffleEnabled == true` | Queue rows render first; the `shuffling — next track unpredictable` placeholder renders below them on the next available row (no playlist tail rows possible under shuffle). |
| `CurrentPlaylistName == ""` | Queue rows render; the `no queue` placeholder renders below them as today (interpretation: queue plays through, then stops). |
| Cache miss, fetch in flight | Queue rows render; `loading…` placeholder below. |
| Cache hit, current PID not in tracks | Queue rows render; `no queue` placeholder below. |
| Cache hit, current track is last entry | Queue rows render; `end of playlist` placeholder below. |
| All other Up Next suppression rules (§3.5 of the predecessor spec — width or height fallback) | Unchanged. When Up Next is suppressed entirely, queue rows are also not shown in the teaser. The overlay (`Q`) is the only way to see queue state in narrow mode. |

### 3.6 Hints bar

The bottom hints bar gains `a:queue Q:queue-view`. The full string becomes:

```
space:play/pause  n:next  p:prev  +/-:vol  a:queue  Q:queue-view  q:quit · j/k:nav  ⏎:play
```

While the overlay is open, the hints bar is suppressed (the overlay renders
its own help row at the bottom of its frame).

### 3.7 Edge cases

- **`PlayTrack(queue head)` returns an error.** Log at warn level. The
  queue head is popped regardless (we already removed it from the queue
  view's notional ordering when we dispatched). The handoff handler returns
  — next tick's behaviour proceeds from the new queue state. A faint
  notice `couldn't play queued track` displays in the hints bar for ~3
  seconds (re-use any existing transient-notice mechanism; if none exists,
  add a `transientNotice` field on `Model` with a tick-driven expiry).
- **`PlayPlaylist(resume)` returns an error.** Log at warn level. Clear
  `resume` so we don't retry on every tick. Transient notice
  `couldn't resume playlist`.
- **Music.app not running when `a` pressed.** Append to queue as normal.
  When Music.app is launched and a track starts playing, the next status
  tick is a track-change (`prevPID == ""`); with empty `prevPlaylist` no
  resume is captured but the queue head still intercepts. Acceptable —
  the user explicitly queued tracks; playing them when Music.app comes up
  is the right behaviour.
- **User adds the same track twice.** Allowed. Each `a` press appends an
  independent entry. Dedupe is the user's job via `x`.
- **Track row has no persistent ID** (parser edge case, e.g., a synthetic
  search result). `a` refuses to enqueue. Transient notice `track has no
  ID — can't queue`.
- **Queue grows large (e.g., 200 items).** Linear scans are fine — single
  user, terminal-bound. Overlay cursor scrolls within available vertical
  space; no pagination.
- **User skips backwards (`p`).** Unchanged. `p` calls `Previous()`.
  Music.app's natural prev may or may not respect our handoff history;
  that's Music.app's problem in V1.
- **User changes playlist context (`Enter` on Main track) while resume is
  already captured.** Resume is **not** overwritten (capture-only-if-empty).
  After queue drains, we resume the originally-captured context. This is
  a deliberate trade-off — see §6.

## 4. Implementation outline

### 4.1 New files

- **`internal/app/queue.go`** — `QueueState` struct with methods:
  - `(q *QueueState) Add(t domain.Track)` — append to `Items`.
  - `(q *QueueState) RemoveAt(i int)` — delete index `i` with bounds check.
  - `(q *QueueState) MoveUp(i int)` / `MoveDown(i int)` — swap with
    neighbour; no-op at edges.
  - `(q *QueueState) PopHead() (domain.Track, bool)` — remove and return
    `Items[0]`, or zero+false if empty.
  - `(q *QueueState) Clear()`.
  - `(q QueueState) Len() int`.

- **`internal/app/resume.go`** — `ResumeContext` struct and the
  status-tick handoff handler:
  - `func (m *Model) handleStatusTickForQueue(now domain.NowPlaying)
    tea.Cmd` — the §3.2 / §3.4 logic. Returns the `PlayTrack` /
    `PlayPlaylist` command (or `nil`).

- **`internal/app/panel_queue.go`** — overlay render and key handler:
  - `func renderOverlay(m Model, width, height int) string` — full-area
    render with header, body, resume footer, help row, optional clear
    prompt.
  - `func updateOverlay(m Model, msg tea.KeyMsg) (Model, tea.Cmd)` —
    routes overlay keys per §3.4.

### 4.2 Modified files

- **`internal/app/model.go`** — embed `queue QueueState`,
  `resume ResumeContext`, `lastTrackPID string`,
  `lastPlaylist string`, `lastTrackIdx int`,
  `overlay overlayState`, `clearPrompt bool`,
  `pendingJumpPID string`, optional `transientNotice` (text + expiry).

- **`internal/app/update.go`** —
  - In the status-tick branch: capture `prev*` values, call
    `m.handleStatusTickForQueue(now)`, then refresh `lastPlaylist` /
    `lastTrackIdx` / `lastTrackPID`. Combine the returned cmd with the
    existing tick cmd via `tea.Batch`.
  - In the global key branch: check `m.overlay.open` first; if open,
    route to `updateOverlay`. Otherwise handle `a`, `Q`, and the modified
    `n` behaviour. All existing global handling preserved.

- **`internal/app/view.go`** — when `m.overlay.open`, render
  `renderOverlay(m, w, h)` and return early. Otherwise existing layout.

- **`internal/app/panel_now_playing.go`** —
  - `renderUpNext` gains a `queue []domain.Track` parameter; render per
    §3.5.
  - Call site in `renderConnectedCardOnly` passes `m.queue.Items`.

- **`internal/app/hints.go`** — add `a:queue Q:queue-view` to the hints
  string. When overlay is open, return empty (or whatever the view layer
  expects to mean "no hints bar").

- **`internal/app/panel_main.go`** (or wherever Main-panel key handling
  lives) — `a` on a track row appends to `m.queue`. Wire via a new
  `enqueueCurrentRow` helper that looks at the focused row's track and,
  if non-empty persistent ID, calls `m.queue.Add(track)`. Refuses
  (transient notice) on empty PID.

- **`internal/cli/cli.go`** — register a stub `queue` subcommand that
  prints a short help message pointing at the TUI keys and exits 0.
  No other CLI work.

### 4.3 No changes required

- **AppleScript** (`internal/music/applescript/scripts.go`) — the queue
  feature uses already-exposed primitives (`PlayTrack`, `PlayPlaylist`,
  `Status`). All of the data needed (`PersistentID`, `CurrentPlaylistName`,
  `ShuffleEnabled`) is already provided by the existing Status script as
  of the read-only Up Next spec.
- **Domain** (`internal/domain/`) — no new fields. `Track.PersistentID`
  is already populated on Status, search results, and playlist tracks.
- **Music client interface** (`internal/music/client.go`) — `PlayTrack`
  and `PlayPlaylist` already match the signatures we need.

## 5. Testing

### 5.1 Unit tests

- **`internal/app/queue_test.go`** (new):
  - `Add` appends to tail.
  - `RemoveAt(0)`, `RemoveAt(last)`, `RemoveAt(out-of-range)` (no-op).
  - `MoveUp(0)`, `MoveDown(last)` (no-ops at edges).
  - `MoveUp(i)` swaps with `i-1`.
  - `PopHead` on empty returns `false`; on non-empty returns head and
    shrinks `Items`.
  - `Clear` empties.
  - `Len` reflects current length after each op.

- **`internal/app/resume_test.go`** (new) — table-driven on the handoff
  handler, using a fake `Client`:
  - No track change → no dispatch, no state change.
  - Track change, empty queue, no resume → no dispatch.
  - Track change, empty queue, resume set → dispatch
    `PlayPlaylist(resume)`, clear `resume`.
  - Track change, queue non-empty, no resume, valid prev playlist/idx →
    capture resume (prev, prev+1), dispatch `PlayTrack(head)`, pop.
  - Track change, queue non-empty, resume already set → do **not**
    overwrite resume, dispatch `PlayTrack(head)`, pop.
  - Track change, queue non-empty, prev playlist empty (no resume target
    valid) → dispatch `PlayTrack(head)`, pop, resume stays empty.
  - Track change, new PID matches queue head → pop only, no dispatch.
  - Track change, new PID matches `pendingJumpPID` → clear flag, capture
    resume if appropriate, no dispatch.

### 5.2 Render and key tests

- **`internal/app/panel_queue_test.go`** (new):
  - Empty state shows centered hint text.
  - Non-empty body: rows numbered `1.`..`N.`, cursor row prefixed `▶`.
  - Resume footer shows `then resumes` + name + `track N of M` when
    `resume.PlaylistName != ""`; shows `then stops` when empty.
  - Clear prompt swaps help row to warning text on `c`.
  - Key `j`/`k` clamps cursor; `x` removes; `K`/`J` reorder + cursor
    follows; `c` then `y` clears; `c` then `j` cancels (no clear,
    cursor moves); `Esc`/`Q` close.
  - `Enter` returns a `PlayTrack` cmd, removes the played item from
    queue, sets `pendingJumpPID`.

- **`internal/app/panel_now_playing_test.go`** (extended):
  - Teaser merge: 2 queued + cap 5 → 2 `★` rows then 3 playlist tail
    rows.
  - Teaser merge: 7 queued + cap 5 → 5 `★` rows, no tail rows.
  - Shuffle on + queue non-empty → `★` rows then `shuffling`
    placeholder.
  - All existing Up Next suppression rules still hold when queue is
    empty.

### 5.3 Wiring tests

- **`internal/app/update_test.go`** (extended):
  - `a` on a focused Main row with valid PID → `m.queue.Len()` grows by 1.
  - `a` on a focused Main row with empty PID → queue unchanged, transient
    notice set.
  - `a` on a non-Main focus → no-op.
  - `Q` → `m.overlay.open == true`.
  - `n` with non-empty queue → dispatches `PlayTrack(head)`, pops.
  - `n` with empty queue → dispatches `Next()` (unchanged from today).
  - `q` while overlay open → no-op (no quit cmd).
  - Status tick wiring: handler called with correct `prev*` values; cmd
    batched with existing tick cmd.

### 5.4 Fake client

- **`internal/music/fake/client.go`** (extended) — record `PlayTrack` and
  `PlayPlaylist` calls in an ordered slice so resume / intercept
  sequences can be asserted. The existing `PlayPlaylistCall` and
  `PlayTrackCall` exported record types (from `c809834`) are reused.

### 5.5 Integration test

- **`internal/music/applescript/client_integration_test.go`** (extended,
  build tag `integration`) — one end-to-end test:
  1. Start Music.app via existing helper.
  2. Call `PlayPlaylist("Liked Songs", 1)` and wait for Status to report
     a track playing.
  3. Pick a distinct second track from the same playlist (different PID)
     and enqueue it by emulating `a` directly on `m.queue` (the test is
     about the handoff, not the keypress wiring).
  4. Loop `Status()` with a 500ms tick and a 30s timeout, watching for
     the natural track change. When it fires, the test's own copy of
     the handoff handler dispatches `PlayTrack(queued.PID)`.
  5. Continue polling Status until the playing PID matches the enqueued
     track. Pass if reached within timeout.
  6. Cleanup: pause Music.app.

  This test is tolerant of timing jitter — Music.app's "natural next"
  may play for up to ~1s before our intercept lands. The assertion is
  eventual, not immediate.

## 6. Risks and trade-offs

- **Audio glitch on intercept (~0.5–1s).** Accepted for V1. If users
  notice and complain, the anticipated-handoff variant (predicting
  end-of-track via `duration - position`) is a follow-up spec.

- **Resume context staleness — capture-only-if-empty.** If the user
  changes playlist context (e.g., presses `Enter` on a track in
  playlist B) *after* a queue handoff has already captured playlist A,
  resume still points at A. When the queue drains, A resumes, not B.
  This is the deliberate cost of the simpler "capture once per run"
  rule. The alternative — overwriting resume on every status tick that
  observes a playlist change — would surprise users who deliberately
  queued tracks from B during an A-rooted session and expected the
  queue to keep playing the queued tracks after B finished. The chosen
  rule matches Spotify's behaviour.

- **Persistence-less queue.** Quitting goove or restarting Music.app
  loses the queue. Acceptable for V1 — adds zero infrastructure cost
  and matches the existing "TUI session" mental model. Persistence is
  a clean follow-up (state file under `~/Library/Application Support/goove/`).

- **CLI queue verbs deferred.** A stub `goove queue` subcommand exists
  to avoid "unknown command" surprise, but does nothing useful. Real
  CLI work waits on persistence or IPC. This is called out in `goove
  queue`'s help message so users aren't left guessing.

- **Overlay swallows quit (`q`).** Users in the overlay must close
  (`Esc`/`Q`) before quitting. Reasoned: `q` is a common typo when
  navigating with `j`/`k`, and an accidental quit mid-queue-curation
  is more annoying than the extra keypress to close. Trade-off
  accepted.

- **Adding the same track multiple times allowed.** No dedupe on
  `Add`. The user might accidentally enqueue the same track twice
  by repeatedly pressing `a`. `x` in the overlay removes duplicates,
  but the friction is real. Deferred — easy to add a dedupe option
  later (`a`-with-warning, or a setting).

- **Overlay is fully modal — globals don't fire.** Suppressing space /
  n / p / +/- inside the overlay is intentional (the overlay's job is
  curation, not transport), but a user who hits `space` to pause from
  inside the overlay will be surprised that nothing happens. The
  trade-off is symmetric with the `q` quit choice — keep the overlay
  focused on its one job. Revisit if the lack of transport keys in
  the overlay becomes a friction point.

- **Linear-scan `indexOfPID` per tick.** O(N) over the currently-cached
  playlist's tracks, every status tick. Bounded by the same playlist
  sizes the existing teaser already scans. If profiling later flags a
  hot spot, the cache can grow a `id → index` map. Not a V1 concern.

- **`pendingJumpPID` is a single-slot string.** If the user rapidly
  presses `Enter` on two different overlay rows before the first
  status tick observes the first jump, the second `Enter` overwrites
  the first jump's flag and dispatches its own `PlayTrack`. The first
  jump's intercept then fires as a normal intercept (no jump
  recognition), playing the queue head — which by then is the second
  jump's target's successor. Effectively the user "interrupted their
  own jump"; acceptable. The two-keypress race is rare enough not to
  warrant a queue-of-pending-jumps in V1.
