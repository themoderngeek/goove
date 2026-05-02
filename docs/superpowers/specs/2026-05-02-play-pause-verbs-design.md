# goove — Separate `play` and `pause` CLI Verbs

**Date:** 2026-05-02
**Status:** Approved
**Builds on:** [CLI Mode](2026-05-02-cli-mode-design.md), [Audio Targets](2026-05-02-audio-targets-design.md)
**Module path:** `github.com/themoderngeek/goove`

## 1. Summary

Add `goove play` and `goove pause` as separate CLI subcommands alongside the existing `goove toggle`. Lets a user bind unambiguous "definitely pause" / "definitely play" hotkeys without querying state first.

This was explicitly deferred from the CLI mode spec ("Separate play and pause commands would require widening music.Client; toggle is enough for v1"). Now that we're widening confidently in other features (album art, audio targets), the trade-off no longer holds.

## 2. Goals

**Must do:**

- Add `Play(ctx) error` and `Pause(ctx) error` to the `music.Client` interface
- Implement both methods on `applescript.Client` and `fake.Client`
- Add `goove play` and `goove pause` CLI subcommands (silent on success, errors via existing `errorExit` helper)
- Update `goove --help` usage text

**Deliberately does not do:**

- Pre-query Music's state before calling `play` / `pause` (an extra round-trip with no value — AppleScript handles the no-op cleanly)
- Remove or deprecate `goove toggle` (it stays — most useful for hotkey-bound "do something with playback")
- Change the TUI (space-bar toggle stays the natural in-app behaviour)
- Add a `--idempotent-check` flag (YAGNI)

## 3. Architecture

The four-layer architecture stays. Two new interface methods, two new AppleScript constants, two new CLI dispatch cases. No new files or packages.

| Concern | Choice | Why |
|---|---|---|
| Add `Play` / `Pause` to `music.Client` | Interface widening | The interface is the contract. Future implementations (MediaRemote backend) need to support these too. |
| Don't pre-query state | Skip the extra round-trip | AppleScript's `play` and `pause` verbs are idempotent at the Music.app level. Calling `play` while already playing is a silent no-op. Querying state first would add ~100ms latency for no behavioural difference. |
| Keep `goove toggle` | Don't deprecate | Most useful for hotkeys where the desired action is "flip whatever state Music is in." |
| `cmdPlay` and `cmdPause` mirror `cmdToggle`'s shape | One file (transport.go), one error helper, one pattern | Trivial to read and maintain. |

## 4. AppleScript scripts

```go
const scriptPlay  = `tell application "Music" to play`
const scriptPause = `tell application "Music" to pause`
```

Both are well-known Music.app AppleScript verbs. Idempotent at the Music.app level (no-op when state already matches). When Music isn't running, AppleScript may auto-launch it — same behaviour as the existing `playpause` verb.

## 5. CLI surface

```
goove play     Start playback (no-op if already playing)
goove pause    Pause playback (no-op if already paused)
goove toggle   Play/pause toggle (existing)
```

Silent on success (Unix convention). All error paths flow through the existing `errorExit` helper:

| State | Stderr | Exit |
|---|---|---|
| Success | (empty) | 0 |
| Music not running | `goove: Apple Music isn't running (run 'goove launch' first)` | 1 |
| Permission denied | `goove: not authorised to control Music — System Settings → Privacy & Security → Automation` | 2 |
| Other error | `goove: <wrapped error>` | 1 |

Identical contract to `goove toggle` / `goove next` / `goove prev`.

`goove --help` usage block gains two lines:

```
  goove play                  Start playback (no-op if already playing)
  goove pause                 Pause playback (no-op if already paused)
```

## 6. Module structure

```
goove/
└── internal/
    ├── music/
    │   ├── client.go                                  # MODIFY — add Play, Pause to Client interface
    │   ├── applescript/
    │   │   ├── scripts.go                             # ADD — scriptPlay, scriptPause constants
    │   │   ├── client.go                              # ADD — Play, Pause methods on Client
    │   │   └── client_test.go                         # ADD — TestPlayRunsScript, TestPauseRunsScript
    │   └── fake/
    │       ├── client.go                              # ADD — PlayCalls, PauseCalls counters; Play, Pause methods (with running guards)
    │       └── client_test.go                         # ADD — fake-side tests for Play and Pause
    └── cli/
        ├── transport.go                               # ADD — cmdPlay, cmdPause handlers
        ├── cli.go                                     # MODIFY — add "play" and "pause" cases; update usageText
        └── cli_test.go                                # ADD — TestPlay*, TestPause* tests (success + not-running)
```

`cmd/goove/main.go` is untouched — the existing dispatcher already routes any subcommand to `cli.Run`.

## 7. Key types and signatures

```go
// internal/music/client.go — interface gains:
Play(ctx context.Context) error
Pause(ctx context.Context) error
```

```go
// internal/music/applescript/scripts.go — constants:
const scriptPlay  = `tell application "Music" to play`
const scriptPause = `tell application "Music" to pause`
```

```go
// internal/music/applescript/client.go — methods:
func (c *Client) Play(ctx context.Context) error {
    _, err := c.run(ctx, scriptPlay)
    return err
}
func (c *Client) Pause(ctx context.Context) error {
    _, err := c.run(ctx, scriptPause)
    return err
}
```

```go
// internal/music/fake/client.go — fields and methods:
PlayCalls  int
PauseCalls int

func (c *Client) Play(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.forcedErr != nil { return c.forcedErr }
    if !c.running        { return music.ErrNotRunning }
    c.PlayCalls++
    return nil
}

func (c *Client) Pause(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.forcedErr != nil { return c.forcedErr }
    if !c.running        { return music.ErrNotRunning }
    c.PauseCalls++
    return nil
}
```

```go
// internal/cli/transport.go — handlers:
func cmdPlay(client music.Client, stderr io.Writer) int {
    if err := client.Play(context.Background()); err != nil {
        return errorExit(err, stderr, true)
    }
    return 0
}
func cmdPause(client music.Client, stderr io.Writer) int {
    if err := client.Pause(context.Background()); err != nil {
        return errorExit(err, stderr, true)
    }
    return 0
}
```

```go
// internal/cli/cli.go — dispatcher gains two cases (placed adjacent to "toggle"):
case "play":   return cmdPlay(client, stderr)
case "pause":  return cmdPause(client, stderr)
case "toggle": return cmdToggle(client, stderr)
```

## 8. Testing strategy

Same pattern as `toggle` / `next` / `prev`.

**`internal/music/applescript/client_test.go`** — two new tests:

- `TestPlayRunsPlayScript` — mock runner asserts `r.script == scriptPlay`
- `TestPauseRunsPauseScript` — same for scriptPause

**`internal/music/fake/client_test.go`** — extend:

- `TestPlayIncrementsCounter` — `c.PlayCalls == 1` after one call
- `TestPauseIncrementsCounter` — same for `PauseCalls`
- `TestPlayNotRunningReturnsErrNotRunning` — fake guards on `c.running`
- `TestPauseNotRunningReturnsErrNotRunning` — same

**`internal/cli/cli_test.go`** — extend:

- `TestPlaySuccessSilentExit0` — fake counter increments, no stdout, no stderr, exit 0
- `TestPlayNotRunningExit1WithHint` — stderr contains "isn't running" + "goove launch"
- `TestPauseSuccessSilentExit0`
- `TestPauseNotRunningExit1WithHint`
- `TestPlayPermissionDeniedExit2`
- `TestPausePermissionDeniedExit2`

No integration tests — the AppleScript verbs are well-trodden, and the existing `TestIntegrationStatus` smoke-tests the AppleScript pipeline overall.

## 9. Architectural decisions log

| Decision | Why |
|---|---|
| Widen `music.Client` (don't add the verbs only at applescript impl) | Interface is the contract for any future backend. |
| AppleScript `play` and `pause` are idempotent — don't pre-query | Avoids a round-trip with no behavioural value. |
| Keep `goove toggle` | Most useful for hotkey-bound "do something with playback" actions. |
| Methods named `Play` / `Pause` (not `Resume` / `Suspend`) | Symmetric with the existing `PlayPause`. Mirrors AppleScript verb names exactly. |
| No TUI changes | Space-bar toggle is the natural in-app gesture. CLI is the right place for unambiguous verbs. |
| Fake `Play` / `Pause` follow the same `forcedErr → running → counter` pattern as `PlayPause` | Consistency. The fake's behaviour matrix already exists; new methods plug into it identically. |

## 10. Scope notes

This spec stands alone. It widens `music.Client` by 2 methods and adds 6 small functions across 4 files. No new packages, no new dependencies, no new patterns. Smaller than every prior feature in this codebase.
