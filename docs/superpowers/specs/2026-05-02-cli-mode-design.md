# goove — CLI Mode Design

**Date:** 2026-05-02
**Status:** Approved
**Builds on:** [goove MVP design](2026-04-30-goove-mvp-design.md), [Album Art design](2026-05-01-album-art-design.md)
**Module path:** `github.com/themoderngeek/goove`

## 1. Summary

Add a CLI subcommand mode to the existing `goove` binary. Bare `goove` (no args) launches the TUI exactly as today. With a recognised first argument, `goove` runs as a one-shot CLI command consuming the existing `music.Client` interface directly — no Bubble Tea, no alt-screen, no log file write, exit when done.

This validates the four-layer architecture's "swap pieces in" promise: a second frontend slots in alongside the TUI without touching the music package, the domain package, or the art package. The CLI is meant for shell scripting (status-bar integrations, hotkey-driven transport control via Karabiner/Hammerspoon, simple `goove pause` from a meeting).

## 2. Goals

**v1 must do:**

- Six subcommands wired to `music.Client` methods: `status`, `toggle`, `next`, `prev`, `volume <n>`, `launch`
- `status` supports plain (default) and JSON (`--json` / `-j`) output formats
- Help via `goove --help`, `goove -h`, or `goove help` (all equivalent)
- Standard exit codes: `0` success, `1` user/runtime error (Music not running, bad args, unknown command), `2` permission denied
- Stdout silent on success for transport commands (Unix convention); errors to stderr
- No log file activity in CLI mode — `~/Library/Logs/goove/goove.log` only updated by the TUI
- Bare `goove` (no args) keeps launching the TUI exactly as today

**v1 deliberately does not do:**

- Separate `play` and `pause` commands (would require widening `music.Client` with `Play()` and `Pause()` methods — `toggle` is enough for v1)
- `goove status --watch` or `goove tail` (use a shell loop)
- `goove search`, `goove queue`, etc. (separate features)
- Tab completion files (Bash / Zsh / Fish)
- `goove version` / `goove about`
- A `--config` flag or persistent settings
- Cross-platform CLI (still darwin-only, same as TUI)

## 3. Architecture

The MVP's four-layer architecture stays. CLI mode adds **one new sibling package** to `app`:

```
internal/
├── app/      ← TUI (Bubble Tea) — UNCHANGED
├── cli/      ← NEW: CLI dispatcher and subcommand handlers
├── domain/   ← UNCHANGED
├── art/      ← UNCHANGED
└── music/    ← UNCHANGED (interface and implementations untouched)
```

The `cli` package exports a single function:

```go
package cli

func Run(args []string, client music.Client, stdout, stderr io.Writer) int
```

`Run` returns an exit code; `main` calls `os.Exit` with it. Tests inject `bytes.Buffer` for stdout/stderr and `fake.Client` for the music backend — every command is unit-testable without shelling out to `osascript` or touching `Music.app`.

The dispatcher in `cmd/goove/main.go` routes based on `os.Args`:

```go
func main() {
    if len(os.Args) > 1 && isCLIMode(os.Args[1]) {
        client := applescript.NewDefault()
        os.Exit(cli.Run(os.Args[1:], client, os.Stdout, os.Stderr))
    }
    // TUI mode (existing path — setupLogging + app.New + tea.NewProgram)
    ...
}

func isCLIMode(firstArg string) bool {
    // Help flags trigger CLI mode (so `goove --help` doesn't launch the TUI).
    // Otherwise: any first arg that doesn't start with a dash is treated as a subcommand.
    return firstArg == "-h" || firstArg == "--help" || !strings.HasPrefix(firstArg, "-")
}
```

**Why a new package and not just functions in `cmd/goove/`:**

- Keeps `main.go` thin (it stays a wiring/dispatch file)
- Makes the CLI a pure unit testable with `bytes.Buffer` + `fake.Client`
- Mirrors the existing `internal/app/` ↔ `internal/cli/` symmetry — both are "frontends" consuming the same `music.Client` interface

**Why CLI mode skips `setupLogging`:**

The existing `setupLogging` opens `~/Library/Logs/goove/goove.log` for append and writes `slog.Info("goove starting")` immediately. For CLI mode this would spam the log file on every `goove status` call (potentially many times per minute if used in a status-bar polling loop). CLI errors print to stderr only.

If a user wants debug-level CLI logging in the future, `GOOVE_LOG=debug` could opt into a stderr-mirror — out of scope for v1.

## 4. Subcommand contract

### `goove status` — print current track

**Plain output (default):**

```
▶ Hippie Sunshine — Kasabian (1:01 / 3:06) vol 21%
```

The leading symbol is `▶` if `IsPlaying`, `⏸` otherwise. If `Artist` is empty the ` — Artist` segment is dropped. `Album` is not shown in plain mode (use `--json` for full data). Position and duration are formatted via the existing `app.formatDuration` (or an equivalent local helper to avoid a cross-package dependency on `internal/app`).

**JSON output (`--json` or `-j`):**

```json
{"is_playing":true,"track":{"title":"Hippie Sunshine","artist":"Kasabian","album":"ACT III"},"position_sec":61,"duration_sec":186,"volume":21}
```

Single-line, no pretty-printing. Field names are explicit (`position_sec`, not `pos`). The `track` field is `null` when `IsPlaying` is false and no track is loaded.

**Edge cases:**

| State | Plain stdout | JSON stdout | Stderr | Exit |
|---|---|---|---|---|
| Connected (track playing/paused) | formatted line | full struct | (empty) | 0 |
| Idle (Music running, no track) | `(no track loaded)` | `{"is_playing":false,"track":null}` — volume is omitted (current `music.Client` interface returns `ErrNoTrack` from `Status()` without volume info) | (empty) | 0 |
| Music not running (`ErrNotRunning`) | (empty) | (empty) | `goove: Apple Music isn't running` | 1 |
| Permission denied (`ErrPermission`) | (empty) | (empty) | `goove: not authorised to control Music — System Settings → Privacy & Security → Automation` | 2 |
| Other transient error (`ErrUnavailable` etc.) | (empty) | (empty) | `goove: <wrapped error>` | 1 |

Errors print to stderr regardless of `--json`. Tools that pipe `goove status --json | jq` either get valid JSON on stdout or nothing.

### `goove toggle` — play/pause toggle

Calls `music.Client.PlayPause`. No stdout output on success.

| State | Stderr | Exit |
|---|---|---|
| Success | (empty) | 0 |
| Music not running | `goove: Apple Music isn't running (run 'goove launch' first)` | 1 |
| Permission denied | (same as `status` permission message) | 2 |
| Other error | `goove: <wrapped error>` | 1 |

### `goove next` — skip to next track

Identical contract to `goove toggle` but calls `music.Client.Next`.

### `goove prev` — skip to previous track

Identical contract to `goove toggle` but calls `music.Client.Prev`.

### `goove volume <n>` — set volume

`<n>` is a required integer argument. Out-of-range values are silently clamped to `[0, 100]` (matches existing `applescript.Client.SetVolume` clamping).

| State | Stderr | Exit |
|---|---|---|
| Success | (empty) | 0 |
| Missing argument | `goove: volume requires a value (0-100)` | 1 |
| Non-numeric argument | `goove: invalid volume: <arg>` | 1 |
| Music not running | `goove: Apple Music isn't running (run 'goove launch' first)` | 1 |
| Permission denied | (same as `status` permission message) | 2 |

### `goove launch` — launch Music.app

Calls `music.Client.Launch`. Idempotent — succeeds whether Music was running or not.

| State | Stderr | Exit |
|---|---|---|
| Success | (empty) | 0 |
| Permission denied | (same as `status` permission message) | 2 |
| Other error | `goove: <wrapped error>` | 1 |

Notably, `launch` does NOT fail if Music is already running — that's the whole point of an idempotent launch.

### `goove --help`, `goove -h`, `goove help` — usage

Prints to **stdout** (not stderr — `--help` is a successful invocation). Exit 0. Format:

```
goove — Apple Music TUI controller

Usage:
  goove                       Launch the TUI
  goove status [--json]       Print the current track (one line)
  goove toggle                Play/pause toggle
  goove next                  Skip to the next track
  goove prev                  Skip to the previous track
  goove volume <0..100>       Set the volume (silently clamps out-of-range)
  goove launch                Launch Apple Music if not running
  goove help, --help, -h      Show this message

Logs: ~/Library/Logs/goove/goove.log (TUI mode only)
Project: github.com/themoderngeek/goove
```

### Unknown command

`goove <unknown>` prints `goove: unknown command: <name>\n\n` to stderr followed by the usage block (also to stderr), exit 1.

## 5. Module structure

```
goove/
├── internal/
│   └── cli/                                          # NEW
│       ├── cli.go                                    # Run() + dispatch + usage + transport handlers (toggle/next/prev/launch) + volume handler
│       ├── status.go                                 # cmdStatus + plain/JSON formatters + StatusJSON struct
│       └── cli_test.go                               # full suite using fake.Client + bytes.Buffer
└── cmd/goove/main.go                                 # MODIFY: add isCLIMode + dispatch shim
```

The `status` command lives in its own file because it's the only one with non-trivial output formatting (two formats, several edge-case branches, the `StatusJSON` struct). Pulling it out keeps `cli.go` focused on dispatch + the simple "call and exit" handlers.

## 6. Key types and signatures

```go
package cli

// Run is the CLI entry point. Returns the exit code.
func Run(args []string, client music.Client, stdout, stderr io.Writer) int

// Internal handlers — one per subcommand.
// Each returns the exit code; writes to stdout/stderr as appropriate.
func cmdStatus(args []string, client music.Client, stdout, stderr io.Writer) int
func cmdToggle(client music.Client, stderr io.Writer) int
func cmdNext(client music.Client, stderr io.Writer) int
func cmdPrev(client music.Client, stderr io.Writer) int
func cmdVolume(args []string, client music.Client, stderr io.Writer) int
func cmdLaunch(client music.Client, stderr io.Writer) int

// printUsage writes the help text to the given writer. Returns 0.
func printUsage(w io.Writer) int

// statusJSON is the wire format for `status --json`. JSON tags keep field names
// explicit (position_sec, not pos) for downstream tools.
type statusJSON struct {
    IsPlaying   bool      `json:"is_playing"`
    Track       *trackRef `json:"track"`                  // null when no track is loaded
    PositionSec *int      `json:"position_sec,omitempty"` // omitted in Idle
    DurationSec *int      `json:"duration_sec,omitempty"` // omitted in Idle
    Volume      *int      `json:"volume,omitempty"`       // omitted in Idle (see §4)
}

type trackRef struct {
    Title  string `json:"title"`
    Artist string `json:"artist"`
    Album  string `json:"album"`
}
```

The `Run` function pattern:

```go
func Run(args []string, client music.Client, stdout, stderr io.Writer) int {
    if len(args) == 0 {
        return printUsage(stderr) // shouldn't happen — main only calls Run with args present
    }
    switch args[0] {
    case "status":
        return cmdStatus(args[1:], client, stdout, stderr)
    case "toggle":
        return cmdToggle(client, stderr)
    case "next":
        return cmdNext(client, stderr)
    case "prev":
        return cmdPrev(client, stderr)
    case "volume":
        return cmdVolume(args[1:], client, stderr)
    case "launch":
        return cmdLaunch(client, stderr)
    case "help", "--help", "-h":
        return printUsage(stdout)
    default:
        fmt.Fprintf(stderr, "goove: unknown command: %s\n\n", args[0])
        printUsage(stderr)
        return 1
    }
}
```

The error-mapping helper used by every command (extracted to keep handlers DRY):

```go
// errorExit prints an error to stderr based on the music.Client error type
// and returns the appropriate exit code.
func errorExit(err error, stderr io.Writer) int {
    switch {
    case errors.Is(err, music.ErrPermission):
        fmt.Fprintln(stderr, "goove: not authorised to control Music — System Settings → Privacy & Security → Automation")
        return 2
    case errors.Is(err, music.ErrNotRunning):
        fmt.Fprintln(stderr, "goove: Apple Music isn't running (run 'goove launch' first)")
        return 1
    default:
        fmt.Fprintf(stderr, "goove: %v\n", err)
        return 1
    }
}
```

`cmdStatus` overrides the `ErrNotRunning` message to omit the `(run 'goove launch' first)` hint (status's job is reporting state; the hint applies to commands that mutate state).

## 7. Runtime behaviour

CLI mode is strictly synchronous:

1. `main` calls `applescript.NewDefault()` to construct the client (same as TUI mode).
2. `main` calls `cli.Run(os.Args[1:], client, os.Stdout, os.Stderr)`.
3. `Run` dispatches to the handler.
4. The handler calls one or more `music.Client` methods, each of which goes through the existing `osascript` path with its 2-second per-call timeout.
5. The handler writes output and returns an exit code.
6. `main` calls `os.Exit(code)`.

No goroutines, no Bubble Tea program, no signal handling beyond Go's default (Ctrl-C terminates the process).

Total wall-clock for a typical `goove status` invocation: ~100ms (osascript spawn + AppleScript run + parse + format).

## 8. Error handling

All errors from `music.Client` flow through `errorExit`, which:

- Maps `ErrPermission` → exit 2 + the macOS Automation hint
- Maps `ErrNotRunning` → exit 1 + the "run goove launch first" hint
- Everything else → exit 1 + `goove: <err.Error()>` to stderr

Argument parsing errors (missing volume value, non-numeric volume, unknown command) print directly to stderr and return exit 1 without going through `errorExit`.

## 9. Testing strategy

All tests live in `internal/cli/cli_test.go`. Standard shape:

```go
func TestStatusPlainConnected(t *testing.T) {
    c := fake.New()
    c.Launch(context.Background())
    c.SetTrack(domain.Track{Title: "T", Artist: "A"}, 186, 61, true)
    var stdout, stderr bytes.Buffer

    code := Run([]string{"status"}, c, &stdout, &stderr)

    if code != 0 {
        t.Errorf("exit = %d; want 0", code)
    }
    if got := stdout.String(); !strings.Contains(got, "T — A") {
        t.Errorf("stdout = %q", got)
    }
    if stderr.Len() != 0 {
        t.Errorf("unexpected stderr: %q", stderr.String())
    }
}
```

The full test list (~18 tests):

- **status**: `TestStatusPlainConnected`, `TestStatusJSONConnected`, `TestStatusPlainIdle`, `TestStatusJSONIdle`, `TestStatusNotRunning`, `TestStatusPermission`, `TestStatusUnavailable`
- **toggle**: `TestToggleSuccess` (asserts `c.PlayPauseCalls == 1`), `TestToggleNotRunning`, `TestTogglePermission`
- **next**: `TestNextSuccess`, `TestNextNotRunning`
- **prev**: `TestPrevSuccess`, `TestPrevNotRunning`
- **volume**: `TestVolumeSuccess` (asserts the value passed), `TestVolumeMissingArg`, `TestVolumeInvalidArg`, `TestVolumeClampHigh`, `TestVolumeClampLow`
- **launch**: `TestLaunchSuccess`, `TestLaunchAlreadyRunning` (idempotent — succeeds when fake's `running` is already true), `TestLaunchPermission`
- **dispatch**: `TestUnknownCommand`, `TestHelpFlag` (test all three: `--help`, `-h`, `help`)

Every test runs in microseconds — no goroutines, no shell-out, no file I/O.

No new integration tests. The CLI doesn't introduce any new shell-out paths (it reuses the existing `applescript.Client`), so the existing `internal/music/applescript/client_integration_test.go` covers the real-world AppleScript behaviour.

## 10. Architectural decisions log

| Decision | Why |
|---|---|
| Subcommands of the existing `goove` binary, not a separate `goovectl` | One binary to install. Bubble Tea's link cost (~5MB) is irrelevant on a Mac. Subcommand dispatch is ~10 lines. |
| Six subcommands (no separate `play` and `pause`) | `toggle` covers the common case. Adding `play` and `pause` would require widening `music.Client` — a larger change for a feature most users won't need. |
| `--json` flag for `status` only | The other commands have no output to format. `--json` adds ~10 lines. Status-bar tools want structured data; humans want one-liners. |
| New `internal/cli/` package as a sibling of `app` | Symmetric with the existing TUI frontend. Pure unit-testable via `bytes.Buffer` + `fake.Client`. |
| CLI mode skips `setupLogging` | A `goove status` polled from a status bar would spam the log file. CLI errors go to stderr only. |
| `errorExit` helper centralises sentinel-to-message mapping | Every handler hits the same three error cases (`ErrPermission`, `ErrNotRunning`, other). Single point of truth for the messages and exit codes. |
| Volume clamping silent (matches AppleScript client behaviour) | Predictable behaviour from the existing layer. Documented in `--help`. |
| `--help` exits 0 to stdout; unknown command exits 1 to stderr | Standard CLI convention (e.g., `git`, `kubectl`). Help is a successful invocation; an unknown command is a user error. |
| Stdout silent on success for transport commands | Unix convention. Tests for these commands assert on call counters (`fake.PlayPauseCalls`) and exit codes, not on stdout text. |

## 11. Scope notes

This spec stands alone. It modifies one existing file (`cmd/goove/main.go`) and adds one new package (`internal/cli`). It does not touch the music package, the domain package, the art package, or the app/TUI package.

If a future iteration wants `goove play` / `goove pause` as separate verbs, that's a separate spec that widens `music.Client` and its two implementations — modest work, but distinct from this CLI mode addition. The same applies to a `Volume()` method that would let `goove status --json` report volume in the Idle state — `music.Client` widening, separate spec.
