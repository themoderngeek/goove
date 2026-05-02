# goove — Audio Target Switching Design

**Date:** 2026-05-02
**Status:** Approved
**Builds on:** [MVP](2026-04-30-goove-mvp-design.md), [Album Art](2026-05-01-album-art-design.md), [CLI Mode](2026-05-02-cli-mode-design.md)
**Module path:** `github.com/themoderngeek/goove`

## 1. Summary

Add v1 audio-target switching: pick the AirPlay device Music routes to (HomePods, AirPlay speakers, AirPods, the Mac's built-in speakers). Surfaces the feature through both the existing TUI (new `o` keybind opens a modal device picker) and the existing CLI (new `goove targets list|get|set` subcommands).

Verified during brainstorming: macOS 26.4.1's Music.app exposes the `AirPlay devices` collection through AppleScript with rich properties (`name`, `kind`, `available`, `active`, `selected`, `sound volume`, etc.) plus a settable `current AirPlay devices` collection. This means the entire feature ships through the existing AppleScript path — no MediaRemote, no CGo, no entitlements, no new binary dependencies.

## 2. Goals

**v1 must do:**

- Add a `domain.AudioDevice` value type (name, kind, available, active, selected)
- Widen `music.Client` with three new methods: `AirPlayDevices`, `CurrentAirPlayDevice`, `SetAirPlayDevice`
- Add two new sentinel errors: `ErrDeviceNotFound`, `ErrAmbiguousDevice`
- Implement those methods in `music/applescript.Client` and `music/fake.Client`
- Three CLI subcommands under `goove targets`: `list`, `get`, `set <name>` (each with `--json` where output makes sense)
- TUI device picker triggered by a new `o` keybind, navigable with arrow keys / vi-style j/k, confirmed with enter, cancelled with esc

**v1 deliberately does not do:**

- Multi-device selection (party mode). `current AirPlay devices` is plural in Music; v1 always treats it as a one-element list.
- macOS system audio output (selecting the system-wide default device, e.g. via `SwitchAudioSource`). Separate concern, separate spec.
- Per-device volume control. Music exposes `sound volume` on each AirPlay device; v1 ignores it — the existing global volume control is sufficient.
- Persisted favourites or quick-pick hotkeys (e.g. `goove targets set @kitchen` aliases).
- Auto-fallback when the current device disappears (e.g. HomePod powered off mid-playback). v1 surfaces the error and lets the user re-pick.
- A "switching to X..." toast in the TUI after a successful set. The picker just closes.
- The `o` keybind launching Music when in `Disconnected` state. The keybind is a no-op there.

## 3. Architecture

The four-layer architecture stays. This feature widens the music client interface and adds a new TUI sub-state. No new packages.

```
internal/
├── app/      ← TUI gains: pickerState struct, devicesMsg, deviceSetMsg,
│                          new "o" keybind handler, picker rendering branch in View
├── cli/      ← gains: cmdTargets dispatcher + subcommands (list/get/set)
├── domain/   ← gains: AudioDevice value type
├── music/    ← gains: AirPlayDevices, CurrentAirPlayDevice, SetAirPlayDevice on Client interface
│              + new sentinels: ErrDeviceNotFound, ErrAmbiguousDevice
└── art/      ← unchanged
```

The picker is a **modal overlay** — orthogonal to `AppState` (Disconnected/Idle/Connected). When open, transport keys (space/n/p/+/-) are suppressed; only picker keys (arrows, enter, esc, q) are honoured.

The CLI's `targets set` and the TUI picker's enter-to-select both go through the same `music.Client.SetAirPlayDevice` method — single point of behaviour.

### Key technology choices

| Concern | Choice | Alternatives considered |
|---|---|---|
| Backend | **Music.app AppleScript via the existing `osascript` runner** | MediaRemote private framework + CGo (unnecessary — AppleScript exposes everything we need); SwitchAudioSource shell-out (different concern: system audio, not Music routing) |
| Cardinality | **Single device only for v1** | Multi-device party mode (real Apple feature, real implementation cost — multi-select TUI widget, variadic CLI args; deferred) |
| Match semantics for `set <name>` | **Exact match wins; otherwise case-insensitive substring** | Always-exact (brittle for shell quoting); always-substring (less predictable when user knows the exact name) |
| TUI keybind | **`o` for "output"** | `d` for "device" (also reasonable, no strong reason to prefer one) |
| TUI overlay model | **Full-screen modal that replaces the player view** | Side-panel / split-view (more layout work; transport keys would have to coexist with picker keys) |

## 4. AppleScript scripts and Go-side matching

Two new scripts cover everything. `CurrentAirPlayDevice` is derived from the list (the device with `selected: true`) — no separate script needed.

### `scriptAirPlayDevices`

```applescript
tell application "Music"
    if not running then return "NOT_RUNNING"
    set out to ""
    repeat with d in AirPlay devices
        set ln to (name of d) & tab & (kind of d as text) & tab & ¬
                  (available of d as text) & tab & (active of d as text) & tab & ¬
                  (selected of d as text)
        if out is "" then
            set out to ln
        else
            set out to out & linefeed & ln
        end if
    end repeat
    return out
end tell
```

Returns one tab-separated line per device: `name\tkind\tavailable\tactive\tselected`. Booleans are emitted as `"true"` / `"false"`. Empty list → empty stdout (a legitimate "Music shows zero devices" state).

Known limitation: device names containing literal tab characters (vanishingly unlikely — names come from Apple's UI which doesn't permit tabs) would break parsing.

### `scriptSetAirPlay`

```applescript
tell application "Music"
    if not running then return "NOT_RUNNING"
    set targetName to "%s"
    set matches to {}
    repeat with d in AirPlay devices
        if (name of d) is equal to targetName then
            set end of matches to d
        end if
    end repeat
    if (count of matches) is 0 then return "NOT_FOUND"
    set current AirPlay devices to {item 1 of matches}
    return "OK"
end tell
```

Format-string `%s` is the **exact** device name (Go side resolves substring/case-insensitive matching first, then passes the unique device's `Name` here). The script's own `NOT_FOUND` check is a safety net for the race window where the device disappears between Go's `AirPlayDevices` call and this `set` call.

### Go-side matching

`matchAirPlayDevice` is a pure function in `internal/music/applescript`:

```go
// matchAirPlayDevice picks the single device whose Name matches the user's input.
// Exact match (case-sensitive) wins; otherwise case-insensitive substring match.
// Returns ErrDeviceNotFound if no matches; ErrAmbiguousDevice if multiple substring matches.
func matchAirPlayDevice(devices []domain.AudioDevice, name string) (domain.AudioDevice, error) {
    for _, d := range devices {
        if d.Name == name {
            return d, nil
        }
    }
    lower := strings.ToLower(name)
    var matches []domain.AudioDevice
    for _, d := range devices {
        if strings.Contains(strings.ToLower(d.Name), lower) {
            matches = append(matches, d)
        }
    }
    if len(matches) == 0 {
        return domain.AudioDevice{}, music.ErrDeviceNotFound
    }
    if len(matches) > 1 {
        return domain.AudioDevice{}, music.ErrAmbiguousDevice
    }
    return matches[0], nil
}
```

Pure, fully unit-testable from constructed `[]domain.AudioDevice` slices.

### `applescript.Client.SetAirPlayDevice`

Composition: list → match → set, with each step's errors flowing through:

```go
func (c *Client) SetAirPlayDevice(ctx context.Context, name string) error {
    devices, err := c.AirPlayDevices(ctx)
    if err != nil {
        return err
    }
    match, err := matchAirPlayDevice(devices, name)
    if err != nil {
        return err  // ErrDeviceNotFound or ErrAmbiguousDevice
    }
    out, err := c.run(ctx, fmt.Sprintf(scriptSetAirPlay, match.Name))
    if err != nil {
        return err
    }
    switch strings.TrimSpace(string(out)) {
    case "OK":
        return nil
    case "NOT_RUNNING":
        return music.ErrNotRunning
    case "NOT_FOUND":
        return music.ErrDeviceNotFound  // race: device disappeared between list and set
    default:
        return fmt.Errorf("%w: unexpected scriptSetAirPlay output: %q", music.ErrUnavailable, out)
    }
}
```

### Parser

`parseAirPlayDevices(raw string) ([]domain.AudioDevice, error)` — pure parser similar to the existing `parseStatus`:

```go
func parseAirPlayDevices(raw string) ([]domain.AudioDevice, error) {
    trimmed := strings.TrimSpace(raw)
    if trimmed == "NOT_RUNNING" {
        return nil, music.ErrNotRunning
    }
    if trimmed == "" {
        return []domain.AudioDevice{}, nil
    }
    var devices []domain.AudioDevice
    for _, line := range strings.Split(trimmed, "\n") {
        fields := strings.Split(line, "\t")
        if len(fields) != 5 {
            return nil, fmt.Errorf("%w: device line has %d fields, want 5: %q",
                music.ErrUnavailable, len(fields), line)
        }
        devices = append(devices, domain.AudioDevice{
            Name:      fields[0],
            Kind:      fields[1],
            Available: fields[2] == "true",
            Active:    fields[3] == "true",
            Selected:  fields[4] == "true",
        })
    }
    return devices, nil
}
```

## 5. CLI surface

Three subcommands under `goove targets`. Two-level dispatch — `goove targets <subcommand>` mirrors `git remote add`, `git stash list`, etc.

### `goove targets list`

**Plain output:**

```
*▶ Mark's Mac mini    (computer)
   Mark's AirPods Pro (computer)
   Kitchen Sonos      (AirPlay)
   Living Room        (AirPlay)
   Office             (AirPlay)  [unavailable]
```

Per-line markers:
- Column 1: `*` if `Selected`, blank otherwise
- Column 2: `▶` if `Active` (currently producing audio), blank otherwise
- Trailing `[unavailable]` after the kind when not `Available`

Names are left-aligned to the longest device name's width. Kind is shown in parentheses.

**JSON output (`--json` / `-j`):**

```json
[{"name":"Mark's Mac mini","kind":"computer","available":true,"active":false,"selected":true},...]
```

Single-line array of device objects.

### `goove targets get`

**Plain output:** the bare name of the currently selected device:

```
Mark's Mac mini
```

**JSON output (`--json` / `-j`):** single object, same shape as `list`'s elements.

### `goove targets set <name>`

Silent on success. `<name>` resolved via `matchAirPlayDevice`.

```bash
$ goove targets set "Kitchen Sonos"
$ echo $?
0
```

### Edge-case behaviour

| Situation | Stderr | Exit |
|---|---|---|
| Music not running | `goove: Apple Music isn't running (run 'goove launch' first)` | 1 |
| Permission denied | `goove: not authorised to control Music — System Settings → Privacy & Security → Automation` | 2 |
| `set <name>` no match | `goove: airplay device not found: <name>` | 1 |
| `set <name>` ambiguous | `goove: 'name' matches multiple devices:\n  Kitchen Sonos\n  Office Sonos` (one match per indented line) | 1 |
| `set` with no name | `goove: targets set requires a device name` | 1 |
| `targets` with no/unknown subcommand | `goove: targets requires a subcommand: list, get, set` + targets-specific help | 1 |
| `list` returns zero devices | Plain: `(no AirPlay devices visible)` to stdout, exit 0; JSON: `[]` to stdout, exit 0 | 0 |

### `goove --help` update

The existing usage block gains:

```
  goove targets list|get|set [name]   Inspect or change the AirPlay device
```

Plus a longer-form `goove targets --help` (or `goove targets help`) that prints just the targets-specific subcommand help.

## 6. TUI picker

The picker is a modal overlay. `m.picker != nil` is the single source of truth for "is the picker open."

### State

```go
type Model struct {
    // ... existing fields ...
    picker *pickerState
}

type pickerState struct {
    loading bool                  // true while devicesMsg is in flight
    devices []domain.AudioDevice  // populated by devicesMsg success
    cursor  int                   // highlighted row index
    err     error                 // load error or set error
}
```

### Trigger

A new `o` keybind opens the picker. Allowed only in `Connected` and `Idle` states (the AirPlay device list requires Music to be running). No-op in `Disconnected` and when `permissionDenied` is set.

### Picker key dispatch

When `m.picker != nil`, all keys route through `handlePickerKey`:

| Key | Effect |
|---|---|
| `up` / `k` | Cursor up (clamped at 0) |
| `down` / `j` | Cursor down (clamped at `len(devices) - 1`) |
| `enter` | Trigger `SetAirPlayDevice` Cmd, mark `loading: true` |
| `esc` / `q` | Close picker (`m.picker = nil`) |
| any other | Ignored (transport keys suppressed) |

While `loading: true`, only `esc` / `q` are honoured (cancel the in-flight set / load).

### Message handlers

```go
case devicesMsg:
    if m.picker == nil { return m, nil }  // user closed before result arrived
    m.picker.loading = false
    m.picker.err = msg.err
    m.picker.devices = msg.devices
    for i, d := range msg.devices {
        if d.Selected {
            m.picker.cursor = i
            break
        }
    }
    return m, nil

case deviceSetMsg:
    if m.picker == nil { return m, nil }
    if msg.err != nil {
        m.picker.loading = false
        m.picker.err = msg.err
        return m, nil
    }
    m.picker = nil  // success → close
    return m, nil
```

### Layout

Modal card-style overlay, full-screen (replaces the player view). Per-line markers inside the picker:
- Column 1: `*` if `Selected`
- Column 2: `▶` if cursor (NOT the active marker — inside the picker, `▶` reads as cursor since cursor matters more during selection)
- Trailing `unavailable` annotation when not `Available`

```
┌─ Pick an output device ──────────────────────────┐
│                                                  │
│   ▶ Mark's Mac mini       (computer)             │
│     Mark's AirPods Pro    (computer)             │
│   *▶ Kitchen Sonos          (AirPlay)            │
│     Living Room            (AirPlay)             │
│     Office                 (AirPlay)  unavailable│
│                                                  │
└──────────────────────────────────────────────────┘
 ↑/↓ navigate   enter select   esc cancel
```

Loading state:

```
┌─ Pick an output device ──────────────────────────┐
│                                                  │
│   Loading devices...                             │
│                                                  │
└──────────────────────────────────────────────────┘
 esc cancel
```

Error state: device list shown if available, with an inline error footer line. If the initial fetch failed entirely, show only the error.

The marker semantics differ between picker (`▶` = cursor) and CLI list (`▶` = active). The divergence is deliberate — different contexts surface different concerns.

### Keybind footer

The bottom-of-screen footer in the player view gains `o: output`:

```
 space: play/pause   n: next   p: prev   +/-: vol   o: output   q: quit
```

Roughly 15 chars longer than today; the `compactThreshold` (50 cols) may need to bump to ~65 to avoid wrapping. Decide empirically during smoke test.

## 7. Module structure

```
goove/
└── internal/
    ├── domain/
    │   └── audio_device.go                          # NEW — AudioDevice value type
    ├── music/
    │   ├── client.go                                # MODIFY — add 3 methods + 2 sentinels
    │   ├── applescript/
    │   │   ├── scripts.go                           # ADD — scriptAirPlayDevices, scriptSetAirPlay
    │   │   ├── client.go                            # ADD — AirPlayDevices, CurrentAirPlayDevice, SetAirPlayDevice + matchAirPlayDevice
    │   │   ├── parse.go                             # ADD — parseAirPlayDevices
    │   │   ├── client_test.go                       # ADD — unit tests
    │   │   ├── parse_test.go                        # ADD — table-driven parser + matcher tests
    │   │   └── client_integration_test.go           # ADD — TestIntegrationAirPlayDevices (read-only)
    │   └── fake/
    │       ├── client.go                            # ADD — SetDevices test hook + 3 method impls
    │       └── client_test.go                       # ADD — fake-side tests
    ├── app/
    │   ├── messages.go                              # ADD — devicesMsg, deviceSetMsg
    │   ├── tick.go                                  # ADD — fetchDevices Cmd factory
    │   ├── model.go                                 # ADD — pickerState struct, Model.picker field
    │   ├── update.go                                # MODIFY — handleKey adds "o", handlePickerKey, devicesMsg+deviceSetMsg cases
    │   ├── update_test.go                           # ADD — picker open/close, key dispatch, msg handlers, transport suppression
    │   ├── view.go                                  # MODIFY — render picker overlay when m.picker != nil; keybind footer gains "o: output"
    │   └── picker.go                                # NEW — renderPicker function (kept separate from view.go)
    └── cli/
        ├── targets.go                               # NEW — cmdTargets dispatch + subcommands + formatters
        ├── cli.go                                   # MODIFY — add "targets" case + update usageText
        └── cli_test.go                              # ADD — cmdTargets* test suite

cmd/goove/main.go                                    # UNCHANGED (existing dispatch shim handles any subcommand)
```

`internal/app/picker.go` keeps the picker-rendering logic out of the already-busy `view.go`. Same precedent as `art` (separate sibling) and the broader pattern of "complex sub-renderers get their own file."

## 8. Key types and signatures

### `internal/domain`

```go
// AudioDevice is a Music.app AirPlay output target.
type AudioDevice struct {
    Name      string
    Kind      string  // "computer", "speaker", "AirPlay" — opaque string from AppleScript, used for display
    Available bool    // false ⇒ device offline / out of range
    Active    bool    // true ⇒ currently producing audio
    Selected  bool    // true ⇒ Music will route to this (may not be Active yet)
}
```

### `internal/music`

```go
type Client interface {
    // existing 8 methods unchanged...
    AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
    CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
    SetAirPlayDevice(ctx context.Context, name string) error
}

var (
    ErrDeviceNotFound  = errors.New("music: airplay device not found")
    ErrAmbiguousDevice = errors.New("music: airplay device name matches multiple devices")
)
```

`CurrentAirPlayDevice` returns `ErrDeviceNotFound` if no device has `Selected=true` (defensive; in practice Music always selects at least Computer).

### `internal/music/fake`

```go
func (c *Client) SetDevices(devices []domain.AudioDevice)  // test hook
func (c *Client) AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
func (c *Client) CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
func (c *Client) SetAirPlayDevice(ctx context.Context, name string) error
```

`fake.SetAirPlayDevice` updates the `Selected` flag on the matching device in its internal list (so subsequent reads reflect the change). Unknown device returns `ErrDeviceNotFound`.

### `internal/app`

```go
// In messages.go:
type devicesMsg struct {
    devices []domain.AudioDevice
    err     error
}

type deviceSetMsg struct {
    err error
}

// In tick.go:
func fetchDevices(client music.Client) tea.Cmd

// In model.go:
type pickerState struct {
    loading bool
    devices []domain.AudioDevice
    cursor  int
    err     error
}
// Model.picker *pickerState  // nil ⇒ picker not open

// In picker.go:
func renderPicker(p *pickerState) string

// In update.go (existing handleKey gains):
//   case "o": if !m.permissionDenied && !isDisconnected(m.state) { open picker }
// Plus a new helper:
func (m Model) handlePickerKey(msg tea.KeyMsg) (Model, tea.Cmd)
```

### `internal/cli`

```go
// In targets.go:
func cmdTargets(args []string, client music.Client, stdout, stderr io.Writer) int
func cmdTargetsList(args []string, client music.Client, stdout, stderr io.Writer) int
func cmdTargetsGet(args []string, client music.Client, stdout, stderr io.Writer) int
func cmdTargetsSet(args []string, client music.Client, stdout, stderr io.Writer) int

// JSON wire format for list/get
type deviceJSON struct {
    Name      string `json:"name"`
    Kind      string `json:"kind"`
    Available bool   `json:"available"`
    Active    bool   `json:"active"`
    Selected  bool   `json:"selected"`
}
```

`cmdTargets` is the two-level dispatcher that routes to `list`/`get`/`set` based on `args[0]`.

## 9. Error handling matrix

| Error path | TUI behaviour | CLI behaviour |
|---|---|---|
| `ErrNotRunning` while opening picker | Shouldn't happen (picker is suppressed in Disconnected). If it does anyway: picker shows error, user esc-cancels. | "Apple Music isn't running" + hint, exit 1 |
| `ErrPermission` while opening picker | Permission-denied screen takes over (existing behaviour) | "not authorised…" exit 2 |
| `ErrUnavailable` (AppleScript fault) | Picker shows error inline | "goove: <err>", exit 1 |
| `SetAirPlayDevice` race → `ErrDeviceNotFound` | Picker stays open, error shown inline, user re-picks | "airplay device not found: <name>", exit 1 |
| `SetAirPlayDevice` → `ErrAmbiguousDevice` (CLI only) | N/A (TUI picks an exact device by reference) | "name matches multiple devices: <list>", exit 1 |
| Empty device list | Picker shows "(no AirPlay devices visible)" with esc-to-cancel | Plain: `(no AirPlay devices visible)` exit 0; JSON: `[]` exit 0 |

The picker NEVER produces a "ghost" set — if `SetAirPlayDevice` fails, the picker stays open, the user can adjust and retry. Esc always closes.

## 10. Testing strategy

Three layers, mirroring the existing pattern.

### `internal/domain` — none

`AudioDevice` is a value type with no methods. Nothing to test.

### `internal/music/applescript` — pure parser + pure matcher tests + mock-runner client tests

Parser tests (`parse_test.go`):
- `TestParseAirPlayDevicesEmpty` — `""` → `[]`
- `TestParseAirPlayDevicesNotRunning` — `"NOT_RUNNING"` → `ErrNotRunning`
- `TestParseAirPlayDevicesSingle` — 1 line → 1 device
- `TestParseAirPlayDevicesMultiple` — 3 lines → 3 devices
- `TestParseAirPlayDevicesMalformedReturnsErrUnavailable` — wrong field count
- `TestParseAirPlayDevicesParsesBoolFields` — `"true"` / `"false"` → bool

Matcher tests (`parse_test.go`):
- `TestMatchAirPlayDeviceExactWins` — exact > substring
- `TestMatchAirPlayDeviceCaseInsensitiveSubstring`
- `TestMatchAirPlayDeviceNotFoundReturnsErrDeviceNotFound`
- `TestMatchAirPlayDeviceAmbiguousReturnsErrAmbiguousDevice`

Client tests (`client_test.go`, using the existing `fakeRunner`):
- `TestAirPlayDevicesRunsScript`
- `TestAirPlayDevicesParsesOutput`
- `TestAirPlayDevicesNotRunning`
- `TestCurrentAirPlayDeviceReturnsSelected`
- `TestCurrentAirPlayDeviceReturnsErrDeviceNotFoundWhenNoneSelected`
- `TestSetAirPlayDeviceCallsListThenSet`
- `TestSetAirPlayDeviceUsesExactNameForSetCall`
- `TestSetAirPlayDeviceRaceReturnsErrDeviceNotFound`

Integration test (`client_integration_test.go`, `//go:build darwin && integration`):
- `TestIntegrationAirPlayDevicesRoundtrip` — list devices, log them; get current, log it. **Read-only by design** — does NOT call `SetAirPlayDevice` because that would disrupt the user's actual audio routing.

### `internal/music/fake` — extend

- `TestSetDevicesPopulates`
- `TestAirPlayDevicesReturnsSet`
- `TestCurrentReturnsSelected`
- `TestSetAirPlayDeviceUpdatesSelectedFlag`
- `TestSetAirPlayDeviceUnknownReturnsErrDeviceNotFound`

### `internal/cli` — extend

- `TestTargetsListPlain`, `TestTargetsListJSON`, `TestTargetsListEmpty`, `TestTargetsListEmptyJSON`, `TestTargetsListNotRunning`
- `TestTargetsGetPlain`, `TestTargetsGetJSON`
- `TestTargetsSetSuccess`, `TestTargetsSetExactMatchPriority`, `TestTargetsSetSubstringMatch`
- `TestTargetsSetNotFound`, `TestTargetsSetAmbiguous`, `TestTargetsSetMissingName`
- `TestTargetsUnknownSubcommand`, `TestTargetsHelp`

### `internal/app` — extend `update_test.go`

- `TestOKeyOpensPicker`
- `TestOKeyIsNoOpInDisconnected`
- `TestOKeyIsNoOpWhenPermissionDenied`
- `TestPickerArrowsNavigateCursor`
- `TestPickerVIKeysAlsoNavigate`
- `TestPickerEnterTriggersSetAirPlayDevice`
- `TestPickerEscClosesPicker`
- `TestPickerQAlsoCloses`
- `TestDevicesMsgPopulatesPicker`
- `TestDevicesMsgErrorShownInPicker`
- `TestDevicesMsgIgnoredWhenPickerClosed`
- `TestDeviceSetMsgSuccessClosesPicker`
- `TestDeviceSetMsgErrorKeepsPickerOpen`
- `TestTransportKeysSuppressedWhilePickerOpen`

No View tests for the picker rendering itself — visual correctness is verified manually.

## 11. Architectural decisions log

| Decision | Why |
|---|---|
| AppleScript over MediaRemote / CGo | Verified via direct probing — Music.app's AppleScript dictionary on macOS 26.4.1 exposes everything we need (`AirPlay devices` collection with rich properties, settable `current AirPlay devices`). MediaRemote / CGo would be unnecessary complexity. |
| Single-device for v1 (no party mode) | Multi-select picker UI is real work; party mode is a rare use case. Backend supports it; only UI deferred. |
| Substring + case-insensitive matching with exact priority for `set <name>` | Names like "Mark's Mac mini" are awkward to type/quote in shells. Substring lowers friction. Exact wins to avoid surprises when names overlap (e.g. "Living Room" vs "Living Room Speakers"). |
| `o` keybind for the TUI picker | Mnemonic for "output." Doesn't collide with any existing TUI key. |
| Modal overlay (full-screen) for the picker | Simplest layout: replace the player view entirely. No worry about coexisting transport keys. |
| `▶` means different things in CLI list vs TUI picker | In CLI: ▶ is "active" (currently producing audio) — informational. In picker: ▶ is "cursor" (where you are right now) — actionable. Each surface highlights the most relevant concept for that context. |
| Picker NEVER closes on a failed set | User stays in the picker to retry. Surprise close-on-error would lose the user's place. |
| No "switching to X" toast after success | Simpler. The next 1Hz status sync will reflect the new state in the player view anyway. |
| `o` keybind suppressed in Disconnected | The AirPlay device list requires Music to be running. Opening a picker that immediately errors is bad UX. |
| Two scripts only (no `scriptCurrentAirPlay`) | `CurrentAirPlayDevice` derives from the list (the device with `Selected=true`). Fewer scripts = less surface area. |
| Tab-separated output for `scriptAirPlayDevices` | Simple, easy to parse. Names containing tabs would break — accepted as known limitation. |
| Two-level CLI dispatch (`goove targets list`) | Mirrors `git remote add`, `git stash list`. Idiomatic for grouped subcommands. Same dispatch pattern Run already uses. |
| `targets list` returns JSON array; `targets get` returns single object | Different data shapes for different use cases. `list` is naturally a collection; `get` is naturally a singleton. |

## 12. Scope notes

This spec stands alone. It widens `music.Client` (3 methods + 2 sentinels), adds one new domain type, adds two new app messages, modifies the TUI key handler and View, modifies the CLI dispatcher. It does not touch the art package or the existing music client implementations beyond adding the three new methods.

If a future iteration wants:
- **Multi-device party mode** — a separate spec that adjusts the picker UI (multi-select), the CLI's `set` semantics (variadic args), and the AppleScript script (no change — `current AirPlay devices` is already plural).
- **System audio routing via SwitchAudioSource** — a separate spec that introduces a new sibling package (`internal/audio`?) and a new CLI namespace (`goove output`?) distinct from `targets` (which belongs to Music).
- **Per-device volume control** — a separate spec that adds `SetAirPlayDeviceVolume(name, percent)` to `music.Client`. Trivial AppleScript (`set sound volume of d to N`).
- **Persisted favourites** — needs the persistent-config feature to land first.
