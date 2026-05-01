# goove — MVP Design

**Date:** 2026-04-30
**Status:** Approved
**Module path:** `github.com/themoderngeek/goove`
**Platforms:** macOS only (build fails on other OSes)

## 1. Summary

goove is a TUI for controlling the Apple Music app on macOS, written in Go. The MVP is a small, focused player remote: play/pause, next, previous, now-playing display, volume. The architecture is layered so that future expansions (search, playlists, audio-target selection, Apple Books, Apple Podcasts, a CLI mode) can be added without rewriting the core.

A secondary purpose of the project is the developer's learning of Go and AI-assisted development, but the design itself does not pay rent for that goal — the code stays idiomatic and the spec stays a spec.

## 2. Goals

**v1 must do:**

- Play / pause toggle
- Skip to next track
- Skip to previous track
- Show "now playing": title, artist, album, progress (elapsed/total), playing/paused state, volume
- Adjust volume up/down

**v1 deliberately does not do:**

- Search
- Playlist browsing or building
- Audio-target switching (HomePod, AirPlay, headphones)
- Album art rendering
- Apple Books or Apple Podcasts integration
- A CLI subcommand mode (e.g. `goove play`)
- Custom keybinds, persistent settings, auto-update
- Linux / Windows support
- Distribution packaging (Homebrew, notarization, signed binaries)

These are non-goals for v1, not forbidden forever. The architecture leaves room for each.

## 3. Architecture

Four layers, each with a single responsibility, each replaceable in isolation.

```
┌──────────────────────────────────────────────────────────────┐
│  TUI layer  (Bubble Tea)                                     │
│    Init → Update(msg) → View()                               │
│    Owns: keybinds, rendering, the 1Hz status tick,           │
│          the 4Hz repaint tick                                │
└────────────────────────┬─────────────────────────────────────┘
                         │  reads/writes
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  Domain model  (pure Go, no I/O)                             │
│    NowPlaying { Track, Position, Duration, IsPlaying,        │
│                 Volume, LastSyncedAt }                       │
│    AppState   { Disconnected | Idle | Connected }            │
│    DisplayedPosition() interpolates from LastSyncedAt        │
└────────────────────────┬─────────────────────────────────────┘
                         │  calls
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  Music client  (Go interface)                                │
│    IsRunning, Launch, Status, PlayPause, Next, Prev,         │
│    SetVolume                                                 │
└────────────────────────┬─────────────────────────────────────┘
                         │  implementations
                         ▼
┌─────────────────────────────┐  ┌─────────────────────────────┐
│ applescript.Client (real)   │  │ fake.Client (tests)         │
│   shells out to osascript   │  │   in-memory state machine   │
└─────────────────────────────┘  └─────────────────────────────┘
```

What this layering buys:

- The TUI never knows how Apple Music is reached. Replacing AppleScript with MediaRemote later is a new `Client` implementation, not a rewrite.
- The domain model has zero I/O, which keeps position-interpolation logic unit-testable and decoupled from rendering.
- The `fake.Client` is the entire below-TUI testing strategy — no real Music.app needed for `Update` tests.
- A future CLI frontend (`goove play`, `goove status`) could sit alongside the TUI as a second consumer of `domain` + `music.Client`, with no changes to the existing layers.

### Key technology choices

| Concern | Choice | Alternatives considered |
|---|---|---|
| Music control mechanism | **AppleScript via `osascript` shell-out** | MediaRemote private framework (CGo, fragile, unblocks audio targets later); ScriptingBridge (CGo, faster than `osascript`, same API surface). AppleScript is sufficient for every v1 feature and avoids CGo on day one. |
| TUI library | **Bubble Tea** (Charm, Elm Architecture) | tcell (lower-level, manual draw loop). Bubble Tea is the de-facto Go TUI library and matches our state-machine model. |
| AppleScript invocation | **Per-call `os/exec` shell-out** | Persistent `osascript -i` REPL. ~100ms per call is fine at our 1Hz polling rate; persistent-session machinery would be unjustified complexity. |

## 4. Module structure

```
goove/
├── go.mod                                     # module github.com/themoderngeek/goove
├── README.md
├── docs/
│   └── superpowers/specs/                     # this file lives here
├── cmd/
│   └── goove/
│       └── main.go                            # wires Client + initial Model, runs Bubble Tea
└── internal/
    ├── app/
    │   ├── model.go                           # tea.Model: AppState, KeyMap, viewport size
    │   ├── update.go                          # Update(msg) — event handling
    │   ├── view.go                            # View() — rendering
    │   ├── tick.go                            # 1Hz status tick + 4Hz repaint tick
    │   └── *_test.go                          # tests using fake.Client
    ├── domain/
    │   ├── nowplaying.go                      # NowPlaying value type + interpolation
    │   └── nowplaying_test.go
    └── music/
        ├── client.go                          # Client interface + sentinel errors
        ├── applescript/
        │   ├── client.go                      # real implementation: osascript shell-out
        │   ├── scripts.go                     # AppleScript snippets as Go consts
        │   └── client_integration_test.go     # build-tagged: needs Music.app
        └── fake/
            └── client.go                      # in-memory state machine
```

`internal/` is enforced by the Go toolchain: any package under `internal/` can only be imported by code within the same module. This prevents domain types and client internals from leaking into a public API by accident.

## 5. Key types

### Domain layer

```go
// internal/domain/nowplaying.go
package domain

import "time"

type Track struct {
    Title  string
    Artist string
    Album  string
}

// NowPlaying is a snapshot from the Music app at LastSyncedAt.
// DisplayedPosition() lets the View interpolate without re-querying.
type NowPlaying struct {
    Track        Track
    Position     time.Duration  // as last reported by Music
    Duration     time.Duration
    IsPlaying    bool
    Volume       int            // 0..100
    LastSyncedAt time.Time      // wall-clock when Position was sampled
}

// DisplayedPosition advances Position by elapsed wall-clock if playing,
// clamped to Duration. Pure function.
func (n NowPlaying) DisplayedPosition(now time.Time) time.Duration { ... }
```

### App-state sum type

Go has no sealed classes, so a closed sum type is modelled as an interface with an unexported method:

```go
// internal/app/model.go
package app

type AppState interface{ isAppState() }

type Disconnected struct{}                      // Music.app not running
type Idle         struct{ Volume int }          // Music running, nothing loaded
type Connected    struct{ Now domain.NowPlaying }

func (Disconnected) isAppState() {}
func (Idle)         isAppState() {}
func (Connected)    isAppState() {}
```

The unexported `isAppState()` method makes the interface unsatisfiable from outside the package, closing the union over `Disconnected | Idle | Connected`.

### Music client interface

```go
// internal/music/client.go
package music

type Client interface {
    IsRunning(ctx context.Context) (bool, error)
    Launch(ctx context.Context) error
    Status(ctx context.Context) (domain.NowPlaying, error)
    PlayPause(ctx context.Context) error
    Next(ctx context.Context) error
    Prev(ctx context.Context) error
    SetVolume(ctx context.Context, percent int) error
}

var (
    ErrNotRunning  = errors.New("music: app not running")
    ErrNoTrack     = errors.New("music: no track loaded")
    ErrUnavailable = errors.New("music: applescript call failed")
    ErrPermission  = errors.New("music: automation permission denied")
)
```

Every client method takes a `context.Context`. The applescript implementation wraps each `osascript` invocation in `context.WithTimeout(ctx, 2*time.Second)` so a hung AppleScript call cannot block the UI.

## 6. Runtime behaviour

### Bubble Tea event loop in one paragraph

`Update(msg, model) -> (model, cmd)` runs for every message; `View(model)` renders after every message. A `Cmd` is "an I/O action that produces a `Msg` when it completes." Nothing in `Update` blocks — all I/O happens inside `Cmd`s and returns as `Msg`s on a later turn of the loop.

### Messages

```go
type tickMsg     struct { now time.Time }   // 1Hz: triggers a Status() fetch
type repaintMsg  struct{}                   // 4Hz: triggers a re-render only
type statusMsg   struct {
    now domain.NowPlaying
    err error
}
type actionDoneMsg struct{ err error }      // result of PlayPause/Next/Prev/SetVolume
```

Plus Bubble Tea built-ins: `tea.KeyMsg`, `tea.WindowSizeMsg`, `tea.QuitMsg`.

### The two-tick model

- **1Hz status tick (`tickMsg`)** — schedules the next tick and fires a `Status()` lookup. The result lands as a `statusMsg`.
- **4Hz repaint tick (`repaintMsg`)** — pure no-op message. Causes Bubble Tea to re-render, so the progress bar advances visibly between status syncs (the bar's value is computed by `NowPlaying.DisplayedPosition(time.Now())`).

The progress bar feels smooth at 4Hz while AppleScript is only called at 1Hz + on user actions.

### Update flow (pseudocode)

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tickMsg:
        return m, tea.Batch(scheduleStatusTick(), fetchStatus(m.client))

    case repaintMsg:
        return m, scheduleRepaintTick()

    case statusMsg:
        switch {
        case errors.Is(msg.err, music.ErrNotRunning):
            m.state = Disconnected{}
        case errors.Is(msg.err, music.ErrNoTrack):
            // Volume is preserved across state transitions on Model.lastVolume
            // (set on every successful Status sync). Default is 50 on first run.
            m.state = Idle{Volume: m.lastVolume}
        case msg.err != nil:
            m.lastError = msg.err
            m.lastErrorAt = time.Now()
        default:
            m.state = Connected{Now: msg.now}
        }
        return m, nil

    case tea.KeyMsg:
        switch msg.String() {
        case " ":      return m, doAction(m.client.PlayPause)
        case "n":      return m, doAction(m.client.Next)
        case "p":      return m, doAction(m.client.Prev)
        case "+", "=": return optimisticVolume(m, +5), doAction(volumeDelta(m.client, +5))
        case "-":      return optimisticVolume(m, -5), doAction(volumeDelta(m.client, -5))
        case "q":      return m, tea.Quit
        }

    case actionDoneMsg:
        if msg.err != nil {
            m.lastError = msg.err
            m.lastErrorAt = time.Now()
        }
        return m, fetchStatus(m.client)  // immediate refresh after every action
    }
    return m, nil
}
```

`optimisticVolume` updates `m.state.(Connected).Now.Volume` immediately so the volume bar moves on keypress, before the AppleScript call returns. The next `statusMsg` reconciles to the real value.

### View

```go
func (m Model) View() string {
    switch s := m.state.(type) {
    case Connected:
        pos := s.Now.DisplayedPosition(time.Now())  // local interpolation
        return renderCard(s.Now, pos, m.errFooter())
    case Idle:
        return renderIdle(s.Volume, m.errFooter())
    case Disconnected:
        return renderDisconnected(m.errFooter())
    }
    return ""
}
```

### Startup sequence

1. `cmd/goove/main.go` constructs `applescript.Client`, builds initial `Model{state: Disconnected{}, lastVolume: 50}`, starts Bubble Tea.
2. The initial `Cmd` returned from `Init()` is an `IsRunning` check.
3. If `IsRunning` returns false → stay in `Disconnected`, render the "Music isn't running — press space to launch, q to quit" screen. Pressing space calls `Launch` and waits for a follow-up `IsRunning` to return true (poll every 250ms for up to 5 seconds).
4. If `IsRunning` returns true → fire a `Status()` call. The resulting `statusMsg` is handled by the same code path as any later tick:
   - `NowPlaying` returned → `Connected`, render the card.
   - `ErrNoTrack` → `Idle`, render the "nothing playing" screen.
   - `ErrNotRunning` (race: Music quit between checks) → back to `Disconnected`.

Note on the `Model.lastVolume` field used above: it's a plain `int` on the model, set on every successful `Status` sync (`m.lastVolume = msg.now.Volume`). It survives state transitions so volume keeps a sensible value when transitioning Connected → Idle → Connected.

### Layout

The "Standard" layout — a now-playing card showing track / artist / album, progress bar with elapsed/total, volume bar, status icon, and a keybind footer.

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
 space: play/pause   n: next   p: prev   +/-: vol   q: quit
```

When the terminal is narrower than **50 columns**, the View switches to a single-line compact variant. The switch happens on `tea.WindowSizeMsg`. The 50-column figure is the working threshold for v1; bump it during implementation if the card visibly clips at 50.

### Keybindings (v1, hardcoded)

| Key | Action |
|---|---|
| `space` | Play / pause toggle |
| `n` | Next track |
| `p` | Previous track |
| `+` / `=` | Volume +5% |
| `-` | Volume −5% |
| `q` | Quit |

## 7. Error handling and edge cases

### Error reactions

| Error | Reaction |
|---|---|
| `ErrNotRunning` | Transition to `Disconnected{}`, redraw |
| `ErrNoTrack` | Transition to `Idle{}`, redraw |
| `ErrPermission` | Show a blocking screen with instructions, `q` to quit |
| Anything else | Show a one-line red error footer for ~3s; keep last known good state |

The transient error footer is held in `Model.lastError` + `Model.lastErrorAt` and cleared by a delayed `clearErrorMsg` fired via `tea.Tick`.

The principle: a TUI user expects signal. Errors are visible. Silence is a worse failure mode than a brief red line.

### Edge cases

| Situation | Behaviour |
|---|---|
| Music starts up while goove is open | Next `tickMsg` finds it running → fetch status → render. No user action needed. |
| Music quits while goove is open | Next status call returns `ErrNotRunning` → fall back to `Disconnected`. Last-known card is *not* preserved. |
| Music running, nothing loaded | `Status()` returns `ErrNoTrack` → `Idle` state, "Music is open, nothing playing — press space or n to start". |
| User holds `+` to ramp volume | Each keypress fires an independent `SetVolume`. Optimistic local update keeps the bar moving. OS keyboard repeat (~30/s) is fine because calls are async. |
| Terminal resized below ~50 cols | `WindowSizeMsg` switches to compact single-line layout; back to the card above the threshold. |
| AppleScript call hangs | 2-second `context.WithTimeout` per call → `ErrUnavailable`, error footer, no UI block. |
| Run on Linux/Windows | `cmd/goove/main.go` has `//go:build darwin`; the build fails fast on other OSes. |

## 8. macOS automation permissions

The first time goove issues an AppleScript command, macOS prompts: *"`goove` (or your terminal app) wants control of `Music.app`."* The user must click Allow once.

If they click Don't Allow, every subsequent `osascript` call returns AppleScript error code **−1743** ("not allowed to send Apple events"). The applescript client maps this to `ErrPermission`.

When `ErrPermission` is the response, goove shows a blocking screen:

```
 Apple Music has blocked goove from controlling it.

 Open  System Settings → Privacy & Security → Automation
 Find  goove (or your terminal app)
 Toggle on  Music

 Then quit and re-run goove.

 q to quit
```

Subtlety: when goove is launched from Terminal.app, iTerm2, Ghostty, etc., macOS attributes the automation request to the **terminal**, not to the goove binary. The user grants the terminal-app permission to control Music. This is a macOS quirk, not something goove can fix from inside. The screen text acknowledges it ("goove (or your terminal app)").

## 9. Testing strategy

Three test layers, mirroring the architecture.

### Domain — pure unit tests

Table-driven tests for `DisplayedPosition`, volume clamping, any other pure logic. Covers behaviour under playing/paused, near-zero positions, positions past `Duration`.

### App layer — `Update`-driven tests with `fake.Client`

`Update` is a pure function: feed it a `Msg`, inspect the returned `Model` and `Cmd`. The `fake.Client` is an in-memory state machine that lets tests script Music.app responses (running / not running / play state / volume / error injection).

Representative tests:

- `TestSpaceTriggersPlayPauseThenRefresh` — feed `tea.KeyMsg{" "}`, assert `Cmd` calls `client.PlayPause`; feed the resulting `actionDoneMsg`, assert next `Cmd` is `fetchStatus`.
- `TestStatusErrTransitionsToDisconnected` — feed `statusMsg{err: ErrNotRunning}`, assert `state == Disconnected{}`.
- `TestStatusErrNoTrackTransitionsToIdle` — `statusMsg{err: ErrNoTrack}` → `Idle`.
- `TestVolumeIsOptimisticallyUpdated` — `KeyMsg{"+"}` → `Connected.Now.Volume` immediately +5, before any action completes.
- `TestErrorFooterIsClearedAfterTimeout` — set `lastError`, feed `clearErrorMsg`, assert footer empty.

Golden-file `View()` tests are possible but not required for v1.

### AppleScript client — opt-in integration tests

Build-tagged with `//go:build integration`. Run with `go test -tags=integration ./internal/music/applescript`. Requires a Mac with Music.app running and automation permission granted. Verifies that the AppleScript snippets actually return what we expect from a real Music.app. Not run in CI.

## 10. Build, run, and observability

- **Develop:** `go run ./cmd/goove`
- **Build:** `go build -o goove ./cmd/goove`
- **Install:** `go install github.com/themoderngeek/goove/cmd/goove@latest`
- **CI:** GitHub Actions on push — `go vet ./...`, `go test ./...`, `go build ./...`. Integration tests are not run in CI. A macOS runner is not strictly required for unit tests because the AppleScript code sits behind the `Client` interface.
- **Logging:** structured logs via `log/slog` (Go 1.21+ stdlib) to `~/Library/Logs/goove/goove.log`. Default level INFO; `GOOVE_LOG=debug` env var bumps to DEBUG.

Distribution packaging (Homebrew tap, signed binary, notarization) is a post-MVP concern.

## 11. Architectural decisions log

Each row captures *why* a decision was made, so future maintainers (or a future you) can judge whether the rationale still holds.

| Decision | Why |
|---|---|
| AppleScript over MediaRemote / ScriptingBridge | Sufficient for every v1 feature; avoids CGo on day one; well-documented and stable across macOS versions. |
| Bubble Tea over tcell | Elm Architecture matches the natural state-machine model of a media player; vast ecosystem of widgets if we need them; no compelling reason to hand-roll a draw loop. |
| Per-call `os/exec` over persistent `osascript -i` REPL | At 1Hz polling, ~100ms per call is fine. Persistent-session machinery would be unjustified complexity. Revisit if/when polling needs to go higher-frequency. |
| Two ticks (1Hz status + 4Hz repaint) | Keeps AppleScript load light while making the progress bar feel alive. |
| 5% volume increment | Coarse enough that a few keypresses span the range, fine enough not to feel chunky. |
| Show "Music isn't running" instead of auto-launching | Avoids a surprise GUI window on `goove` startup; user is in control. |
| Visible error footer over silent + log-only | TUI users expect signal. A red line is a better failure mode than silence. |
| Sum type via interface + unexported method | Closest Go gets to a sealed class. Lets the compiler enforce a closed set of `AppState` variants within the `app` package. |
| `internal/` package layout | Toolchain-enforced privacy. Domain types and client internals can't leak to consumers. |
| `cmd/goove/` binary entrypoint | Conventional Go layout. Leaves room for additional binaries (e.g. future `cmd/goovectl/` CLI) without restructuring. |
| Integration tests behind a build tag | Real Music.app needed, not CI-friendly. Default `go test ./...` stays fast and hermetic. |

## 12. Scope notes

This spec is intentionally a single deliverable. Each non-goal in §2 deserves its own spec when its turn comes — search alone is a non-trivial design with its own AppleScript subset, its own UI state, its own keybinds.

The point of v1 is to ship the smallest version of goove that the developer would actually use, and to do so on architecture that won't have to be torn up to add the next feature.
