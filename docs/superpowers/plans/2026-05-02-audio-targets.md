# goove Audio Target Switching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add v1 audio-target switching: a TUI modal picker (new `o` keybind) and a CLI subcommand group (`goove targets list|get|set <name>`) that select the AirPlay device Music routes audio to.

**Architecture:** `music.Client` widens with three methods (`AirPlayDevices`, `CurrentAirPlayDevice`, `SetAirPlayDevice`) plus two sentinel errors (`ErrDeviceNotFound`, `ErrAmbiguousDevice`) and a new `domain.AudioDevice` value type. The `applescript` impl uses two new scripts (`scriptAirPlayDevices`, `scriptSetAirPlay`); the `fake` impl gains a single-slot device list with a test hook. The TUI gains a modal-overlay state (`Model.picker *pickerState`) that suppresses transport keys when open. The CLI gains a two-level dispatcher (`cmdTargets` → `list` / `get` / `set`).

**Tech Stack:** Go 1.24, stdlib only (`encoding/json`, `errors`, `strings`, etc.) plus the existing Bubble Tea / Lipgloss already in use. No new external dependencies. Spec: `docs/superpowers/specs/2026-05-02-audio-targets-design.md`.

---

## File Structure

The plan modifies eight existing files and adds five new ones across four packages. Every task references exact paths from this list.

```
goove/
└── internal/
    ├── domain/
    │   └── audio_device.go                         # T2 — AudioDevice value type
    ├── music/
    │   ├── client.go                               # T3 — ADD: 3 interface methods + 2 sentinels
    │   ├── applescript/
    │   │   ├── scripts.go                          # T5 — ADD: scriptAirPlayDevices + scriptSetAirPlay
    │   │   ├── parse.go                            # T6 — ADD: parseAirPlayDevices
    │   │   ├── parse_test.go                       # T6, T7 — ADD: parser tests + matcher tests
    │   │   ├── client.go                           # T7, T8 — ADD: matchAirPlayDevice + 3 client methods
    │   │   ├── client_test.go                      # T7, T8 — ADD: client method tests
    │   │   └── client_integration_test.go          # T9 — ADD: TestIntegrationAirPlayDevicesRoundtrip
    │   └── fake/
    │       ├── client.go                           # T4 — ADD: SetDevices test hook + 3 method impls
    │       └── client_test.go                      # T4 — ADD: fake-side tests
    ├── app/
    │   ├── messages.go                             # T11 — ADD: devicesMsg, deviceSetMsg
    │   ├── tick.go                                 # T11 — ADD: fetchDevices Cmd factory
    │   ├── model.go                                # T12 — ADD: pickerState struct + Model.picker field
    │   ├── update.go                               # T13, T14 — MODIFY: handleKey 'o' case + handlePickerKey + msg cases
    │   ├── update_test.go                          # T13, T14 — ADD: 14 picker-related tests
    │   ├── view.go                                 # T15 — MODIFY: render picker overlay + footer "o: output"
    │   └── picker.go                               # T15 — NEW: renderPicker function
    └── cli/
        ├── targets.go                              # T16, T17, T18 — NEW: cmdTargets dispatcher + list/get/set + JSON formatters
        ├── cli.go                                  # T16 — MODIFY: add 'targets' case + update usageText
        └── cli_test.go                             # T16, T17, T18 — ADD: cmdTargets* test suite
```

`cmd/goove/main.go` is untouched — the existing dispatcher already routes any subcommand to `cli.Run`.

## Naming and signature contract

Used identically across tasks:

| Symbol | Definition |
|---|---|
| `domain.AudioDevice` | `struct{ Name, Kind string; Available, Active, Selected bool }` |
| `music.Client.AirPlayDevices(ctx) ([]domain.AudioDevice, error)` | New interface method |
| `music.Client.CurrentAirPlayDevice(ctx) (domain.AudioDevice, error)` | New interface method |
| `music.Client.SetAirPlayDevice(ctx, name string) error` | New interface method |
| `music.ErrDeviceNotFound` | Sentinel: `errors.New("music: airplay device not found")` |
| `music.ErrAmbiguousDevice` | Sentinel: `errors.New("music: airplay device name matches multiple devices")` |
| `applescript.scriptAirPlayDevices` | Const; returns `NOT_RUNNING` or tab-separated lines (`name\tkind\tavailable\tactive\tselected`) |
| `applescript.scriptSetAirPlay` | Format-string const with `%s` for exact device name; returns `OK` / `NOT_RUNNING` / `NOT_FOUND` |
| `applescript.parseAirPlayDevices(raw string) ([]domain.AudioDevice, error)` | Pure parser |
| `applescript.matchAirPlayDevice(devices, name) (domain.AudioDevice, error)` | Pure matcher (exact > case-insensitive substring) |
| `fake.Client.SetDevices(devices)` | Test hook to seed the fake's device list |
| `app.devicesMsg{devices, err}` | Message |
| `app.deviceSetMsg{err}` | Message |
| `app.fetchDevices(client) tea.Cmd` | Cmd factory |
| `app.pickerState{loading, devices, cursor, err}` | Model field type |
| `app.Model.picker *pickerState` | nil ⇒ picker not open |
| `app.Model.handlePickerKey(msg) (Model, tea.Cmd)` | Picker key dispatcher |
| `app.renderPicker(p *pickerState) string` | Picker rendering function (in `picker.go`) |
| `cli.cmdTargets(args, client, stdout, stderr) int` | Two-level dispatcher |
| `cli.cmdTargetsList(args, client, stdout, stderr) int` | List subcommand |
| `cli.cmdTargetsGet(args, client, stdout, stderr) int` | Get subcommand |
| `cli.cmdTargetsSet(args, client, stderr) int` | Set subcommand (no stdout output) |
| `cli.deviceJSON` struct | JSON wire format with snake-case fields |

---

## Phase 1 — Bootstrap

### Task 1: Create feature branch and verify clean starting state

**No files modified.**

- [ ] **Step 1: Create the feature branch from main**

Run:
```bash
git checkout main
git checkout -b feature/audio-targets
```

If the local main is behind origin (e.g. PR #4 was merged on GitHub but not pulled), STOP and report — the user will need to pull manually.

- [ ] **Step 2: Confirm spec/plan are present and tree is clean**

Run:
```bash
ls docs/superpowers/specs/2026-05-02-audio-targets-design.md
ls docs/superpowers/plans/2026-05-02-audio-targets.md
ls internal/cli/cli.go
ls internal/art/renderer.go
git status
git log -5 --format='%h %s'
```

Expected:
- All four `ls` commands succeed (`internal/cli/cli.go` and `internal/art/renderer.go` confirm we're on post-CLI-mode and post-album-art main)
- `git status` reports clean (or only `.claude/` untracked)

- [ ] **Step 3: Verify the existing test suite passes**

Run:
```bash
go test ./...
go vet ./...
go build ./...
```

Expected: all green.

No commit for this task.

---

## Phase 2 — Domain + interface widening

### Task 2: Add `domain.AudioDevice`

**Files:**
- Create: `internal/domain/audio_device.go`

Pure value type. No methods, no logic. No tests needed.

- [ ] **Step 1: Create the file**

Create `internal/domain/audio_device.go`:

```go
package domain

// AudioDevice is a Music.app AirPlay output target.
//
// Selected indicates "Music will route here" (may not currently be producing
// audio); Active indicates "currently producing audio." Both can be true at
// once when a track is playing through the selected device.
type AudioDevice struct {
	Name      string
	Kind      string // "computer", "speaker", "AirPlay" — opaque, for display
	Available bool   // false ⇒ device offline / out of range
	Active    bool   // true ⇒ currently producing audio
	Selected  bool   // true ⇒ Music will route to this
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./internal/domain/...
go test ./internal/domain/...
```

Expected: build silent; existing domain tests still pass.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/audio_device.go
git commit -m "domain: AudioDevice value type"
```

---

### Task 3: Widen `music.Client` interface + add sentinels

**Files:**
- Modify: `internal/music/client.go`

This commit INTENTIONALLY breaks the build — `applescript.Client` and `fake.Client` will fail their compile-time interface guards until Tasks 4 and 8 add the new methods.

- [ ] **Step 1: Read the current `client.go`**

Run:
```bash
cat internal/music/client.go
```

Confirm the existing 8-method `Client` interface (`IsRunning`, `Launch`, `Status`, `PlayPause`, `Next`, `Prev`, `SetVolume`, `Artwork`) and 5 sentinel errors.

- [ ] **Step 2: Add three methods to the interface and two sentinels**

Edit `internal/music/client.go`. Inside the `Client` interface, AFTER `Artwork`, add:

```go
	AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
	CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
	SetAirPlayDevice(ctx context.Context, name string) error
```

Inside the `var (...)` sentinel block, AFTER `ErrNoArtwork`, add:

```go
	ErrDeviceNotFound  = errors.New("music: airplay device not found")
	ErrAmbiguousDevice = errors.New("music: airplay device name matches multiple devices")
```

The interface now has 11 methods; the sentinel block now has 7 errors.

- [ ] **Step 3: Verify the build fails as expected**

Run:
```bash
go build ./...
```

Expected: errors mentioning `*Client does not implement music.Client (missing method AirPlayDevices)` for both `applescript.Client` and `fake.Client`. This is the contract working as intended.

- [ ] **Step 4: Commit**

```bash
git add internal/music/client.go
git commit -m "music: add AirPlay device methods + sentinels to Client interface"
```

---

## Phase 3 — Fake client

### Task 4: `fake.Client` AirPlay support

**Files:**
- Modify: `internal/music/fake/client.go`
- Modify: `internal/music/fake/client_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/music/fake/client_test.go`:

```go
func TestSetDevicesPopulatesList(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	devices := []domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true},
	}
	c.SetDevices(devices)

	got, err := c.AirPlayDevices(context.Background())
	if err != nil {
		t.Fatalf("AirPlayDevices err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[0].Name != "Computer" || got[1].Name != "Kitchen Sonos" {
		t.Errorf("got names = %q, %q", got[0].Name, got[1].Name)
	}
}

func TestAirPlayDevicesNotRunning(t *testing.T) {
	c := New()
	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestCurrentAirPlayDeviceReturnsSelected(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: false},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Selected: true},
	})

	got, err := c.CurrentAirPlayDevice(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestCurrentAirPlayDeviceNoneSelectedReturnsErrDeviceNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
	})

	_, err := c.CurrentAirPlayDevice(context.Background())
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestSetAirPlayDeviceUpdatesSelectedFlag(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos", Selected: false},
	})

	if err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos"); err != nil {
		t.Fatalf("err = %v", err)
	}
	got, _ := c.AirPlayDevices(context.Background())
	if got[0].Selected {
		t.Errorf("Computer.Selected = true; want false")
	}
	if !got[1].Selected {
		t.Errorf("Kitchen Sonos.Selected = false; want true")
	}
}

func TestSetAirPlayDeviceUnknownReturnsErrDeviceNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})

	err := c.SetAirPlayDevice(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestAirPlayDevicesHonoursForcedErr(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	c.SimulateError(music.ErrPermission)

	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrPermission) {
		t.Fatalf("err = %v; want ErrPermission", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/music/fake/...
```

Expected: build fails — `Client has no field or method SetDevices` and similar for the three new interface methods. The compile-time guard `var _ music.Client = (*Client)(nil)` also fails.

- [ ] **Step 3: Implement on fake.Client**

In `internal/music/fake/client.go`:

a. Add a new field to the `Client` struct (after the existing `artworkErr` field, before the counters block):

```go
	devices []domain.AudioDevice
```

b. Add the test hook + three interface methods. The conventional location is grouped with the other test hooks (`SetTrack`, `SimulateError`, `SetArtwork`), so place these AFTER `SetArtworkErr`:

```go
// SetDevices supplies the AirPlay device list the next AirPlayDevices call returns.
// SetAirPlayDevice mutates the Selected flag on entries in this list.
func (c *Client) SetDevices(devices []domain.AudioDevice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices = devices
}

// AirPlayDevices implements music.Client.
func (c *Client) AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	// Return a copy so callers can't mutate our internal slice.
	out := make([]domain.AudioDevice, len(c.devices))
	copy(out, c.devices)
	return out, nil
}

// CurrentAirPlayDevice implements music.Client. Returns the device with
// Selected=true, or ErrDeviceNotFound if no device is selected.
func (c *Client) CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return domain.AudioDevice{}, c.forcedErr
	}
	if !c.running {
		return domain.AudioDevice{}, music.ErrNotRunning
	}
	for _, d := range c.devices {
		if d.Selected {
			return d, nil
		}
	}
	return domain.AudioDevice{}, music.ErrDeviceNotFound
}

// SetAirPlayDevice implements music.Client. Updates the Selected flag in-place:
// the named device becomes Selected=true, all others become Selected=false.
// Returns ErrDeviceNotFound if no device with the exact name exists.
func (c *Client) SetAirPlayDevice(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	found := false
	for i := range c.devices {
		if c.devices[i].Name == name {
			c.devices[i].Selected = true
			found = true
		} else {
			c.devices[i].Selected = false
		}
	}
	if !found {
		return music.ErrDeviceNotFound
	}
	return nil
}
```

The compile-time guard `var _ music.Client = (*Client)(nil)` at the bottom of `fake/client.go` will now compile cleanly.

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test -race ./internal/music/fake/...
```

Expected: all existing tests + the 7 new `TestSetDevices*` / `TestAirPlayDevices*` / `TestCurrentAirPlayDevice*` / `TestSetAirPlayDevice*` tests pass with the race detector on.

- [ ] **Step 5: Commit**

```bash
git add internal/music/fake/client.go internal/music/fake/client_test.go
git commit -m "music/fake: SetDevices test hook + AirPlay method impls"
```

---

## Phase 4 — AppleScript client

### Task 5: AppleScript constants

**Files:**
- Modify: `internal/music/applescript/scripts.go`

- [ ] **Step 1: Append the two new script constants**

Append to `internal/music/applescript/scripts.go` (after the existing `scriptArtwork` constant):

```go
// scriptAirPlayDevices returns one tab-separated line per AirPlay device:
//   name\tkind\tavailable\tactive\tselected
// Empty list ⇒ empty stdout. Returns "NOT_RUNNING" if Music isn't running.
//
// NOTE: device names containing literal tab characters (vanishingly unlikely —
// names come from Apple's UI which doesn't permit tabs) would corrupt parsing.
const scriptAirPlayDevices = `tell application "Music"
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
end tell`

// scriptSetAirPlay sets the current AirPlay devices to the single named device.
// %s is the EXACT device name (matched on the Go side first via matchAirPlayDevice).
// Returns "OK" on success, "NOT_RUNNING" if Music isn't running, "NOT_FOUND" if
// no device with the exact name exists (race window guard: device disappeared
// between the list call and the set call).
const scriptSetAirPlay = `tell application "Music"
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
end tell`
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./internal/music/applescript/...
```

Expected: build still fails because `applescript.Client` doesn't yet implement the three new interface methods. The error should mention `Artwork` is implemented but `AirPlayDevices` isn't — confirming this task added the constants without breaking anything else.

- [ ] **Step 3: Commit**

```bash
git add internal/music/applescript/scripts.go
git commit -m "music/applescript: scriptAirPlayDevices + scriptSetAirPlay constants"
```

---

### Task 6: `parseAirPlayDevices` parser

**Files:**
- Create: `internal/music/applescript/parse_test.go` (the file already exists from MVP — append)
- Modify: `internal/music/applescript/parse.go` (already exists — append)

- [ ] **Step 1: Append failing parser tests**

Append to `internal/music/applescript/parse_test.go`:

```go
func TestParseAirPlayDevicesEmpty(t *testing.T) {
	got, err := parseAirPlayDevices("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParseAirPlayDevicesNotRunning(t *testing.T) {
	_, err := parseAirPlayDevices("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParseAirPlayDevicesSingle(t *testing.T) {
	raw := "Computer\tcomputer\ttrue\tfalse\ttrue\n"
	got, err := parseAirPlayDevices(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	want := domain.AudioDevice{Name: "Computer", Kind: "computer", Available: true, Active: false, Selected: true}
	if got[0] != want {
		t.Errorf("got = %+v; want %+v", got[0], want)
	}
}

func TestParseAirPlayDevicesMultiple(t *testing.T) {
	raw := "Computer\tcomputer\ttrue\tfalse\ttrue\n" +
		"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n" +
		"Office\tAirPlay\tfalse\tfalse\tfalse"
	got, err := parseAirPlayDevices(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[0].Name != "Computer" || got[1].Name != "Kitchen Sonos" || got[2].Name != "Office" {
		t.Errorf("names = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	if got[2].Available {
		t.Errorf("Office.Available = true; want false")
	}
}

func TestParseAirPlayDevicesParsesBoolFields(t *testing.T) {
	raw := "X\tspeaker\tfalse\ttrue\tfalse\n"
	got, _ := parseAirPlayDevices(raw)
	if got[0].Available || !got[0].Active || got[0].Selected {
		t.Errorf("got = %+v", got[0])
	}
}

func TestParseAirPlayDevicesMalformedReturnsErrUnavailable(t *testing.T) {
	// Only 3 fields instead of 5
	raw := "X\tspeaker\ttrue\n"
	_, err := parseAirPlayDevices(raw)
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want ErrUnavailable", err)
	}
}
```

You'll need `"github.com/themoderngeek/goove/internal/domain"` in the test file's imports if not already present.

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/music/applescript/ -run TestParseAirPlayDevices
```

Expected: build fails — `undefined: parseAirPlayDevices`.

- [ ] **Step 3: Append the parser to `parse.go`**

Append to `internal/music/applescript/parse.go`:

```go
// parseAirPlayDevices parses the tab-separated output of scriptAirPlayDevices.
// Special sentinel NOT_RUNNING maps to music.ErrNotRunning. Empty input
// (Music shows zero AirPlay devices — legitimate state) returns an empty slice.
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

The existing `parse.go` already imports `"fmt"`, `"strings"`, `"github.com/themoderngeek/goove/internal/domain"`, and `"github.com/themoderngeek/goove/internal/music"` (from `parseStatus`). No new imports needed.

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test ./internal/music/applescript/ -run TestParseAirPlayDevices
```

Expected: 6 sub-tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/music/applescript/parse.go internal/music/applescript/parse_test.go
git commit -m "music/applescript: parseAirPlayDevices parser"
```

---

### Task 7: `matchAirPlayDevice` matcher + `AirPlayDevices` / `CurrentAirPlayDevice` methods

**Files:**
- Modify: `internal/music/applescript/parse.go` (or new file — see step 3 note)
- Modify: `internal/music/applescript/parse_test.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write failing matcher tests**

Append to `internal/music/applescript/parse_test.go`:

```go
func TestMatchAirPlayDeviceExactWins(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Living Room"},
		{Name: "Living Room Speakers"},
	}
	got, err := matchAirPlayDevice(devices, "Living Room")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Living Room" {
		t.Errorf("got = %q; want exact 'Living Room'", got.Name)
	}
}

func TestMatchAirPlayDeviceCaseInsensitiveSubstring(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Mark's Mac mini"},
		{Name: "Kitchen Sonos"},
	}
	got, err := matchAirPlayDevice(devices, "kitchen")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestMatchAirPlayDeviceNotFoundReturnsErrDeviceNotFound(t *testing.T) {
	devices := []domain.AudioDevice{{Name: "Computer"}}
	_, err := matchAirPlayDevice(devices, "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestMatchAirPlayDeviceAmbiguousReturnsErrAmbiguousDevice(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Kitchen Sonos"},
		{Name: "Office Sonos"},
	}
	_, err := matchAirPlayDevice(devices, "sonos")
	if !errors.Is(err, music.ErrAmbiguousDevice) {
		t.Fatalf("err = %v; want ErrAmbiguousDevice", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/music/applescript/ -run TestMatchAirPlayDevice
```

Expected: build fails — `undefined: matchAirPlayDevice`.

- [ ] **Step 3: Implement matcher**

Append to `internal/music/applescript/parse.go` (the matcher conceptually belongs alongside the parser — both are pure helpers that don't touch `osascript`):

```go
// matchAirPlayDevice picks the single device whose Name matches the user's input.
// Exact match (case-sensitive) wins immediately; otherwise case-insensitive
// substring match. Returns ErrDeviceNotFound if no matches; ErrAmbiguousDevice
// if multiple substring matches.
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

- [ ] **Step 4: Run matcher tests, verify pass**

Run:
```bash
go test ./internal/music/applescript/ -run TestMatchAirPlayDevice
```

Expected: 4 sub-tests pass.

- [ ] **Step 5: Write failing tests for AirPlayDevices and CurrentAirPlayDevice on the Client**

Append to `internal/music/applescript/client_test.go`:

```go
func TestAirPlayDevicesRunsScript(t *testing.T) {
	r := &fakeRunner{out: []byte("")}
	c := New(r)
	c.AirPlayDevices(context.Background())
	if r.script != scriptAirPlayDevices {
		t.Errorf("ran %q; want scriptAirPlayDevices", r.script)
	}
}

func TestAirPlayDevicesParsesOutput(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\ttrue\n")}
	c := New(r)

	devices, err := c.AirPlayDevices(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "Computer" {
		t.Errorf("got = %+v", devices)
	}
}

func TestAirPlayDevicesNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestCurrentAirPlayDeviceReturnsSelected(t *testing.T) {
	r := &fakeRunner{out: []byte(
		"Computer\tcomputer\ttrue\tfalse\tfalse\n" +
			"Kitchen Sonos\tAirPlay\ttrue\tfalse\ttrue\n",
	)}
	c := New(r)

	got, err := c.CurrentAirPlayDevice(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestCurrentAirPlayDeviceNoneSelectedReturnsErrDeviceNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\tfalse\n")}
	c := New(r)
	_, err := c.CurrentAirPlayDevice(context.Background())
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}
```

- [ ] **Step 6: Run, verify failure**

Run:
```bash
go test ./internal/music/applescript/ -run "TestAirPlayDevices|TestCurrentAirPlayDevice"
```

Expected: build fails — `Client has no field or method AirPlayDevices`.

- [ ] **Step 7: Implement AirPlayDevices and CurrentAirPlayDevice on Client**

Append to `internal/music/applescript/client.go` (BEFORE the existing `var _ music.Client = (*Client)(nil)` guard at the bottom):

```go
// AirPlayDevices implements music.Client.
func (c *Client) AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error) {
	out, err := c.run(ctx, scriptAirPlayDevices)
	if err != nil {
		return nil, err
	}
	return parseAirPlayDevices(string(out))
}

// CurrentAirPlayDevice implements music.Client. Returns the device with
// Selected=true, or music.ErrDeviceNotFound if no device is selected.
func (c *Client) CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error) {
	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		return domain.AudioDevice{}, err
	}
	for _, d := range devices {
		if d.Selected {
			return d, nil
		}
	}
	return domain.AudioDevice{}, music.ErrDeviceNotFound
}
```

The compile-time guard will still fail because `SetAirPlayDevice` isn't implemented yet (Task 8). That's expected.

- [ ] **Step 8: Run tests, verify pass**

Run:
```bash
go test ./internal/music/applescript/ -run "TestMatchAirPlayDevice|TestAirPlayDevices|TestCurrentAirPlayDevice|TestParseAirPlayDevices"
```

Expected: all 14 sub-tests pass (6 parser + 4 matcher + 4 client). The package overall still fails to build (interface guard), which is normal until Task 8.

- [ ] **Step 9: Commit**

```bash
git add internal/music/applescript/parse.go internal/music/applescript/parse_test.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: matchAirPlayDevice + AirPlayDevices/CurrentAirPlayDevice"
```

---

### Task 8: `SetAirPlayDevice` method

**Files:**
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

This is the final piece that unbreaks the build.

- [ ] **Step 1: Write failing tests**

Append to `internal/music/applescript/client_test.go`:

```go
// twoCallFakeRunner records every script invocation, so we can verify
// SetAirPlayDevice does the list-then-set sequence.
type twoCallFakeRunner struct {
	scripts []string
	outs    [][]byte
	errs    []error
	idx     int
}

func (f *twoCallFakeRunner) Run(ctx context.Context, script string) ([]byte, error) {
	f.scripts = append(f.scripts, script)
	if f.idx >= len(f.outs) {
		return nil, errors.New("no more outputs scripted")
	}
	out, err := f.outs[f.idx], f.errs[f.idx]
	f.idx++
	return out, err
}

func TestSetAirPlayDeviceCallsListThenSet(t *testing.T) {
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Computer\tcomputer\ttrue\tfalse\ttrue\n" +
				"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n"),
			[]byte("OK\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)

	err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.scripts) != 2 {
		t.Fatalf("script call count = %d; want 2", len(r.scripts))
	}
	if r.scripts[0] != scriptAirPlayDevices {
		t.Errorf("first script = %q; want scriptAirPlayDevices", r.scripts[0])
	}
	if !strings.Contains(r.scripts[1], "Kitchen Sonos") {
		t.Errorf("second script did not contain device name: %q", r.scripts[1])
	}
}

func TestSetAirPlayDeviceUsesExactNameForSetCall(t *testing.T) {
	// User passes substring "kitchen"; matcher resolves to "Kitchen Sonos";
	// the set script should be called with the exact name "Kitchen Sonos".
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Computer\tcomputer\ttrue\tfalse\ttrue\n" +
				"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n"),
			[]byte("OK\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)

	c.SetAirPlayDevice(context.Background(), "kitchen")
	if !strings.Contains(r.scripts[1], "Kitchen Sonos") {
		t.Errorf("set script did not contain exact name 'Kitchen Sonos': %q", r.scripts[1])
	}
}

func TestSetAirPlayDeviceNotFoundReturnsErrDeviceNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\ttrue\n")}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestSetAirPlayDeviceAmbiguousReturnsErrAmbiguousDevice(t *testing.T) {
	r := &fakeRunner{out: []byte(
		"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n" +
			"Office Sonos\tAirPlay\ttrue\tfalse\tfalse\n",
	)}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "sonos")
	if !errors.Is(err, music.ErrAmbiguousDevice) {
		t.Fatalf("err = %v; want ErrAmbiguousDevice", err)
	}
}

func TestSetAirPlayDeviceRaceReturnsErrDeviceNotFound(t *testing.T) {
	// List succeeds; set returns NOT_FOUND (the device disappeared between calls).
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Kitchen Sonos\tAirPlay\ttrue\tfalse\ttrue\n"),
			[]byte("NOT_FOUND\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/music/applescript/ -run TestSetAirPlayDevice
```

Expected: build fails — `Client has no field or method SetAirPlayDevice`.

- [ ] **Step 3: Implement SetAirPlayDevice**

Append to `internal/music/applescript/client.go` (BEFORE the `var _ music.Client = (*Client)(nil)` guard):

```go
// SetAirPlayDevice implements music.Client. Resolves the user's name input
// against the AirPlay device list (exact match first, then case-insensitive
// substring), then runs scriptSetAirPlay with the matched device's exact name.
func (c *Client) SetAirPlayDevice(ctx context.Context, name string) error {
	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		return err
	}
	match, err := matchAirPlayDevice(devices, name)
	if err != nil {
		return err
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
		return music.ErrDeviceNotFound
	default:
		return fmt.Errorf("%w: unexpected scriptSetAirPlay output: %q", music.ErrUnavailable, out)
	}
}
```

- [ ] **Step 4: Run all package tests, verify pass**

Run:
```bash
go test ./internal/music/applescript/...
```

Expected: every existing test + every new test passes. The compile-time guard `var _ music.Client = (*Client)(nil)` should now compile cleanly.

Run also:
```bash
go build ./...
```

Expected: succeeds across the whole module — Tasks 3-7 had intentionally broken it; this is the task that finishes unbreaking it.

- [ ] **Step 5: Commit**

```bash
git add internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: SetAirPlayDevice via list-match-set composition"
```

---

### Task 9: AppleScript integration test

**Files:**
- Modify: `internal/music/applescript/client_integration_test.go`

- [ ] **Step 1: Append the integration test**

Append to `internal/music/applescript/client_integration_test.go`:

```go
func TestIntegrationAirPlayDevicesRoundtrip(t *testing.T) {
	c := NewDefault()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, err := c.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Skip("Music.app is not running; cannot exercise AirPlayDevices")
	}

	devices, err := c.AirPlayDevices(ctx)
	if err != nil {
		t.Fatalf("AirPlayDevices err = %v", err)
	}
	t.Logf("Music reports %d AirPlay device(s):", len(devices))
	for _, d := range devices {
		t.Logf("  - %s (kind=%s available=%v active=%v selected=%v)",
			d.Name, d.Kind, d.Available, d.Active, d.Selected)
	}

	current, err := c.CurrentAirPlayDevice(ctx)
	if err != nil {
		t.Logf("CurrentAirPlayDevice returned %v (acceptable if no device selected)", err)
	} else {
		t.Logf("Currently selected: %s", current.Name)
	}

	// Read-only by design — this test does NOT call SetAirPlayDevice
	// because that would disrupt the user's actual audio routing.
}
```

- [ ] **Step 2: Verify default test run does NOT pick this up**

Run:
```bash
go test ./internal/music/applescript/...
```

Expected: passes — integration test is gated behind `//go:build darwin && integration`.

- [ ] **Step 3: Commit**

```bash
git add internal/music/applescript/client_integration_test.go
git commit -m "music/applescript: integration test for AirPlay devices (read-only)"
```

---

## Phase 5 — App layer

### Task 10: messages and Cmd factory

**Files:**
- Modify: `internal/app/messages.go`
- Modify: `internal/app/tick.go`

- [ ] **Step 1: Add the two new message types**

Append to `internal/app/messages.go`:

```go
// devicesMsg is the result of a fetchDevices Cmd — populates the picker.
type devicesMsg struct {
	devices []domain.AudioDevice
	err     error
}

// deviceSetMsg is the result of a SetAirPlayDevice call from inside the picker.
// On success, the picker closes; on error, the picker stays open and shows the error.
type deviceSetMsg struct {
	err error
}
```

- [ ] **Step 2: Add fetchDevices Cmd factory**

Append to `internal/app/tick.go`:

```go
// fetchDevices runs AirPlayDevices in a goroutine and emits a devicesMsg.
// Used by the picker on open.
func fetchDevices(client music.Client) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.AirPlayDevices(context.Background())
		return devicesMsg{devices: devices, err: err}
	}
}
```

`tick.go` already imports `context`, `time`, `tea`, and `music` — no new imports needed.

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./internal/app/...
```

Expected: silent. The existing tests still pass since this is additive.

- [ ] **Step 4: Commit**

```bash
git add internal/app/messages.go internal/app/tick.go
git commit -m "app: devicesMsg + deviceSetMsg + fetchDevices Cmd factory"
```

---

### Task 11: Model fields — `pickerState` struct + `Model.picker`

**Files:**
- Modify: `internal/app/model.go`

- [ ] **Step 1: Add pickerState struct + Model field**

In `internal/app/model.go`, add the `pickerState` struct AFTER the `artState` struct definition but BEFORE the `Model` struct:

```go
// pickerState is the modal device-picker overlay state.
// nil on Model means "picker not open"; non-nil means "picker is showing."
// While loading is true, only esc/q are honoured (cancel cancels both fetch and set).
type pickerState struct {
	loading bool
	devices []domain.AudioDevice
	cursor  int
	err     error
}
```

Then ADD a new field to `Model` (after the existing `art artState` and `renderer art.Renderer` fields):

```go
	picker *pickerState // nil ⇒ picker not open (modal overlay state)
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./internal/app/...
go test ./internal/app/...
```

Expected: silent build; existing tests pass (the field is unused by current code paths).

- [ ] **Step 3: Commit**

```bash
git add internal/app/model.go
git commit -m "app: pickerState struct + Model.picker field"
```

---

### Task 12: `o` keybind opens picker + handlePickerKey

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write failing tests for the `o` keybind**

Append to `internal/app/update_test.go`:

```go
func TestOKeyOpensPickerInConnected(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	m := New(c, nil)
	m.state = Connected{Now: domain.NowPlaying{Track: domain.Track{Title: "T"}}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)

	if got.picker == nil {
		t.Fatal("expected picker to be open after 'o' keypress")
	}
	if !got.picker.loading {
		t.Error("expected picker.loading = true while fetch is in flight")
	}
	if cmd == nil {
		t.Error("expected a fetchDevices Cmd")
	}
}

func TestOKeyOpensPickerInIdle(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.state = Idle{Volume: 50}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker == nil {
		t.Fatal("expected picker to be open in Idle state")
	}
}

func TestOKeyIsNoOpInDisconnected(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	// Default state is Disconnected{}.

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil (suppressed in Disconnected)", updated.(Model).picker)
	}
}

func TestOKeyIsNoOpWhenPermissionDenied(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.permissionDenied = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil (suppressed when permissionDenied)", updated.(Model).picker)
	}
}

func TestPickerArrowsNavigateCursor(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{
			{Name: "A"}, {Name: "B"}, {Name: "C"},
		},
		cursor: 0,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).picker.cursor != 1 {
		t.Errorf("cursor after down = %d; want 1", updated.(Model).picker.cursor)
	}

	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor after up = %d; want 0", updated.(Model).picker.cursor)
	}
}

func TestPickerArrowsClampAtBoundaries(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}
	// Up at top — stays at 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor at top after up = %d; want 0", updated.(Model).picker.cursor)
	}
	// Down past bottom — clamps to last
	m = updated.(Model)
	for range 5 {
		tmp, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = tmp.(Model)
	}
	if m.picker.cursor != 1 {
		t.Errorf("cursor after spam-down = %d; want 1 (clamped)", m.picker.cursor)
	}
}

func TestPickerVIKeysAlsoNavigate(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.(Model).picker.cursor != 1 {
		t.Errorf("cursor after j = %d; want 1", updated.(Model).picker.cursor)
	}
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor after k = %d; want 0", updated.(Model).picker.cursor)
	}
}

func TestPickerEscClosesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after esc", updated.(Model).picker)
	}
}

func TestPickerQAlsoCloses(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after q", updated.(Model).picker)
	}
}

func TestPickerEnterTriggersSetAirPlayDevice(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	m := New(c, nil)
	m.picker = &pickerState{
		devices: []domain.AudioDevice{
			{Name: "Computer", Selected: true},
			{Name: "Kitchen Sonos"},
		},
		cursor: 1, // pointing at Kitchen Sonos
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.picker.loading {
		t.Error("expected loading=true after enter")
	}
	if cmd == nil {
		t.Fatal("expected a Cmd from enter")
	}
	out := cmd()
	dsm, ok := out.(deviceSetMsg)
	if !ok {
		t.Fatalf("cmd returned %T; want deviceSetMsg", out)
	}
	if dsm.err != nil {
		t.Errorf("deviceSetMsg.err = %v; want nil", dsm.err)
	}
}

func TestPickerWhileLoadingOnlyEscWorks(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  0,
	}

	// Down should be ignored.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).picker.cursor != 0 {
		t.Errorf("cursor moved while loading = %d; want 0", updated.(Model).picker.cursor)
	}
	// Esc still closes.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).picker != nil {
		t.Error("esc did not close picker while loading")
	}
}

func TestTransportKeysSuppressedWhilePickerOpen(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	m := New(c, nil)
	m.picker = &pickerState{devices: []domain.AudioDevice{{Name: "A"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.PlayPauseCalls != 0 {
		t.Errorf("PlayPauseCalls = %d; want 0 (suppressed by picker)", c.PlayPauseCalls)
	}
	// Picker still open after the suppressed key.
	if updated.(Model).picker == nil {
		t.Error("picker closed unexpectedly")
	}
}
```

You'll need `context` and `domain` already imported in update_test.go (they are, from prior tasks).

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/app/...
```

Expected: most of the new tests fail. The TestOKey* ones fail because the `o` keybind doesn't open the picker yet (falls through to the default no-op). The TestPicker* ones fail because key handling doesn't route to `handlePickerKey`.

- [ ] **Step 3: Add the `o` case + handlePickerKey to update.go**

In `internal/app/update.go`, modify `handleKey` to:
- Route to `handlePickerKey` when `m.picker != nil`
- Add the `o` case for opening the picker

The full new `handleKey` (replacing the existing one):

```go
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.permissionDenied {
		if msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.picker != nil {
		return m.handlePickerKey(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case " ":
		if _, ok := m.state.(Disconnected); ok {
			return m, doAction(m.client.Launch)
		}
		return m, doAction(m.client.PlayPause)

	case "n":
		return m, doAction(m.client.Next)

	case "p":
		return m, doAction(m.client.Prev)

	case "+", "=":
		return m.applyVolumeDelta(+5)

	case "-":
		return m.applyVolumeDelta(-5)

	case "o":
		// Open the device picker. Suppressed in Disconnected — the AirPlay
		// device list requires Music to be running. permissionDenied is also
		// suppressed (handled at the top of this function).
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		m.picker = &pickerState{loading: true}
		return m, fetchDevices(m.client)
	}
	return m, nil
}

// handlePickerKey routes keystrokes when the picker overlay is open.
// Transport keys are suppressed by virtue of routing through this function
// instead of the normal switch.
func (m Model) handlePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.picker.loading {
		// Only esc/q work while loading.
		if msg.String() == "esc" || msg.String() == "q" {
			m.picker = nil
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		m.picker = nil
		return m, nil

	case "up", "k":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
		return m, nil

	case "down", "j":
		if m.picker.cursor < len(m.picker.devices)-1 {
			m.picker.cursor++
		}
		return m, nil

	case "enter":
		if len(m.picker.devices) == 0 {
			return m, nil
		}
		target := m.picker.devices[m.picker.cursor].Name
		m.picker.loading = true
		client := m.client
		return m, func() tea.Msg {
			err := client.SetAirPlayDevice(context.Background(), target)
			return deviceSetMsg{err: err}
		}
	}
	return m, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test ./internal/app/...
```

Expected: all new tests pass + every existing test still passes.

- [ ] **Step 5: Commit**

```bash
git add internal/app/update.go internal/app/update_test.go
git commit -m "app: 'o' opens picker + handlePickerKey routes picker input"
```

---

### Task 13: devicesMsg + deviceSetMsg handlers

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write failing tests for the message handlers**

Append to `internal/app/update_test.go`:

```go
func TestDevicesMsgPopulatesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{loading: true}

	devices := []domain.AudioDevice{
		{Name: "Computer", Selected: false},
		{Name: "Kitchen Sonos", Selected: true},
	}
	updated, _ := m.Update(devicesMsg{devices: devices, err: nil})
	got := updated.(Model)

	if got.picker.loading {
		t.Error("loading still true after devicesMsg")
	}
	if len(got.picker.devices) != 2 {
		t.Errorf("len = %d; want 2", len(got.picker.devices))
	}
	// Cursor should land on the currently-selected device.
	if got.picker.cursor != 1 {
		t.Errorf("cursor = %d; want 1 (Kitchen Sonos has Selected=true)", got.picker.cursor)
	}
}

func TestDevicesMsgErrorShownInPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{loading: true}

	updated, _ := m.Update(devicesMsg{err: music.ErrUnavailable})
	got := updated.(Model)

	if got.picker.loading {
		t.Error("loading still true after error devicesMsg")
	}
	if got.picker.err == nil {
		t.Error("expected picker.err set")
	}
}

func TestDevicesMsgIgnoredWhenPickerClosed(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	// picker is nil — user esc'd before fetch landed.

	updated, _ := m.Update(devicesMsg{devices: []domain.AudioDevice{{Name: "A"}}})
	if updated.(Model).picker != nil {
		t.Error("picker should remain nil; stale devicesMsg should be discarded")
	}
}

func TestDeviceSetMsgSuccessClosesPicker(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}},
	}

	updated, _ := m.Update(deviceSetMsg{err: nil})
	if updated.(Model).picker != nil {
		t.Errorf("picker = %+v; want nil after successful set", updated.(Model).picker)
	}
}

func TestDeviceSetMsgErrorKeepsPickerOpen(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.picker = &pickerState{
		loading: true,
		devices: []domain.AudioDevice{{Name: "A"}, {Name: "B"}},
		cursor:  1,
	}

	updated, _ := m.Update(deviceSetMsg{err: music.ErrDeviceNotFound})
	got := updated.(Model)

	if got.picker == nil {
		t.Fatal("picker closed on error; want it to stay open")
	}
	if got.picker.loading {
		t.Error("loading still true after error deviceSetMsg")
	}
	if got.picker.err == nil {
		t.Error("expected picker.err set")
	}
	if got.picker.cursor != 1 {
		t.Errorf("cursor changed unexpectedly to %d", got.picker.cursor)
	}
}

func TestDeviceSetMsgIgnoredWhenPickerClosed(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	// picker is nil — user esc'd before set landed.

	updated, _ := m.Update(deviceSetMsg{err: nil})
	if updated.(Model).picker != nil {
		t.Error("picker should remain nil")
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/app/...
```

Expected: tests fail because Update doesn't have cases for devicesMsg and deviceSetMsg yet.

- [ ] **Step 3: Add the message handlers to Update**

In `internal/app/update.go`, add two new cases to the type switch in `Update` (BEFORE the closing brace of the switch, after the existing cases):

```go
	case devicesMsg:
		if m.picker == nil {
			return m, nil // user esc'd before fetch returned — discard
		}
		m.picker.loading = false
		m.picker.err = msg.err
		m.picker.devices = msg.devices
		// Land cursor on currently-selected device, if any.
		for i, d := range msg.devices {
			if d.Selected {
				m.picker.cursor = i
				break
			}
		}
		return m, nil

	case deviceSetMsg:
		if m.picker == nil {
			return m, nil // user esc'd before set returned — discard
		}
		if msg.err != nil {
			m.picker.loading = false
			m.picker.err = msg.err
			return m, nil
		}
		// Success: close the picker. Next 1Hz status tick re-renders the player view.
		m.picker = nil
		return m, nil
```

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test ./internal/app/...
```

Expected: all 6 new message-handler tests pass + every existing test still passes.

- [ ] **Step 5: Commit**

```bash
git add internal/app/update.go internal/app/update_test.go
git commit -m "app: devicesMsg + deviceSetMsg handlers (close picker on success)"
```

---

### Task 14: View renders picker overlay + footer update

**Files:**
- Modify: `internal/app/view.go`
- Create: `internal/app/picker.go`

No new tests in this task — visual rendering is verified by running the binary in the final smoke test.

- [ ] **Step 1: Create picker.go with renderPicker**

Create `internal/app/picker.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPicker is the modal overlay shown when m.picker != nil.
// Replaces the player view entirely (no side-by-side composition).
func renderPicker(p *pickerState) string {
	var body strings.Builder

	if p.loading {
		body.WriteString("Loading devices...")
	} else if len(p.devices) == 0 {
		body.WriteString("(no AirPlay devices visible)")
	} else {
		// Compute the longest name for left-alignment.
		maxName := 0
		for _, d := range p.devices {
			if len(d.Name) > maxName {
				maxName = len(d.Name)
			}
		}
		for i, d := range p.devices {
			selectedMark := " "
			if d.Selected {
				selectedMark = "*"
			}
			cursorMark := " "
			if i == p.cursor {
				cursorMark = "▶"
			}
			line := fmt.Sprintf("  %s%s %-*s (%s)",
				selectedMark, cursorMark, maxName, d.Name, d.Kind)
			if !d.Available {
				line += "  unavailable"
			}
			body.WriteString(line)
			if i < len(p.devices)-1 {
				body.WriteString("\n")
			}
		}
	}

	if p.err != nil {
		body.WriteString("\n\n")
		body.WriteString(errorStyle.Render("error: " + p.err.Error()))
	}

	header := titleStyle.Render("Pick an output device")
	card := cardStyle.Render(header + "\n\n" + body.String())

	var footerText string
	if p.loading {
		footerText = " esc cancel"
	} else {
		footerText = " ↑/↓ navigate   enter select   esc cancel"
	}
	footer := footerStyle.Render(footerText)

	return lipgloss.NewStyle().Margin(0, 2).Render(card + "\n" + footer)
}
```

The `errorStyle`, `titleStyle`, `cardStyle`, `footerStyle` package-level lipgloss styles are defined in `view.go` — no need to redefine, they're accessible from `picker.go` (same package).

- [ ] **Step 2: Modify View() to render picker when open**

In `internal/app/view.go`, modify `View()`. Add a check at the very top — BEFORE the `permissionDenied` check (the picker overlay is the highest-priority render except for nothing; but we want permission-denied to still take precedence as it's a more critical error state). Actually: we want the order to be **permissionDenied → picker → state-based render**. So add the picker check between permissionDenied and the compactThreshold check:

```go
func (m Model) View() string {
	if m.permissionDenied {
		return renderPermissionDenied()
	}
	if m.picker != nil {
		return renderPicker(m.picker)
	}
	if m.width > 0 && m.width < compactThreshold {
		return renderCompact(m)
	}
	switch s := m.state.(type) {
	// ... existing cases unchanged ...
	}
	return ""
}
```

- [ ] **Step 3: Update the keybind footer to include "o: output"**

In `internal/app/view.go`, find the existing `connectedKeybindsText` constant (added during the album-art branch's full-width-footer refactor) and update it to include the new `o: output`:

```go
const connectedKeybindsText = " space: play/pause   n: next   p: prev   +/-: vol   o: output   q: quit"
```

If the existing footer was different (the implementer should diff against what's actually there), preserve all existing keys and add `o: output` between the volume and quit indicators.

- [ ] **Step 4: Verify build + tests**

Run:
```bash
go build ./...
go test ./...
go vet ./...
```

Expected: all green. View tests don't exist for the picker (visual correctness is manual), so the existing tests cover the unchanged paths.

- [ ] **Step 5: Build the binary**

Run:
```bash
go build -o goove ./cmd/goove
```

Expected: produces a `goove` binary.

- [ ] **Step 6: Commit**

```bash
git add internal/app/picker.go internal/app/view.go
git commit -m "app: View — render picker overlay + 'o: output' in keybind footer"
```

---

## Phase 6 — CLI surface

### Task 15: `cmdTargets` dispatcher + `cmdTargetsList` (plain + JSON)

**Files:**
- Create: `internal/cli/targets.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/cli_test.go`:

```go
func TestTargetsListPlainConnected(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Active: true},
		{Name: "Office", Kind: "AirPlay", Available: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "Computer") {
		t.Errorf("stdout missing Computer: %q", got)
	}
	if !strings.Contains(got, "*") {
		t.Errorf("stdout missing selected marker '*': %q", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("stdout missing active marker '▶': %q", got)
	}
	if !strings.Contains(got, "unavailable") {
		t.Errorf("stdout missing 'unavailable' annotation for Office: %q", got)
	}
}

func TestTargetsListJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list", "--json"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0]["name"] != "Computer" {
		t.Errorf("name = %v; want Computer", got[0]["name"])
	}
	if got[0]["selected"] != true {
		t.Errorf("selected = %v; want true", got[0]["selected"])
	}
}

func TestTargetsListEmptyPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{}) // empty list
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if !strings.Contains(stdout.String(), "(no AirPlay devices visible)") {
		t.Errorf("stdout missing empty marker: %q", stdout.String())
	}
}

func TestTargetsListEmptyJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{})
	var stdout, stderr bytes.Buffer

	Run([]string{"targets", "list", "--json"}, c, &stdout, &stderr)
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Errorf("stdout = %q; want '[]'", stdout.String())
	}
}

func TestTargetsListNotRunningExit1(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestTargetsNoSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a subcommand") {
		t.Errorf("stderr missing 'requires a subcommand': %q", stderr.String())
	}
}

func TestTargetsUnknownSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "frobnicate"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Errorf("stderr missing unknown subcommand name: %q", stderr.String())
	}
}

func TestTargetsHelpFlag(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		t.Run(arg, func(t *testing.T) {
			c := fake.New()
			var stdout, stderr bytes.Buffer

			code := Run([]string{"targets", arg}, c, &stdout, &stderr)
			if code != 0 {
				t.Errorf("exit = %d; want 0", code)
			}
			if !strings.Contains(stdout.String(), "manage Music's AirPlay") {
				t.Errorf("stdout missing targets-specific help: %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/cli/... -run TestTargets
```

Expected: tests fail — `targets` falls through to "unknown command".

- [ ] **Step 3: Implement cmdTargets + cmdTargetsList**

Create `internal/cli/targets.go`:

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// Note: `errors` is added to imports in Task 17 when cmdTargetsSet uses errors.Is.

// deviceJSON is the wire format for `goove targets list --json` and
// `goove targets get --json`. snake_case to match other CLI JSON shapes.
type deviceJSON struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Selected  bool   `json:"selected"`
}

func toDeviceJSON(d domain.AudioDevice) deviceJSON {
	return deviceJSON{
		Name:      d.Name,
		Kind:      d.Kind,
		Available: d.Available,
		Active:    d.Active,
		Selected:  d.Selected,
	}
}

// cmdTargets is the two-level dispatcher for `goove targets <subcommand>`.
func cmdTargets(args []string, client music.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: targets requires a subcommand: list, get, set")
		return 1
	}
	switch args[0] {
	case "list":
		return cmdTargetsList(args[1:], client, stdout, stderr)
	case "get":
		return cmdTargetsGet(args[1:], client, stdout, stderr)
	case "set":
		return cmdTargetsSet(args[1:], client, stderr)
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, "goove targets — manage Music's AirPlay output device")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Usage:")
		fmt.Fprintln(stdout, "  goove targets list [--json]   List all AirPlay devices")
		fmt.Fprintln(stdout, "  goove targets get [--json]    Print the currently-selected device")
		fmt.Fprintln(stdout, "  goove targets set <name>      Set the AirPlay device by name")
		return 0
	default:
		fmt.Fprintf(stderr, "goove: unknown targets subcommand: %s\n", args[0])
		fmt.Fprintln(stderr, "       valid subcommands: list, get, set")
		return 1
	}
}

func cmdTargetsList(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	devices, err := client.AirPlayDevices(context.Background())
	if err != nil {
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		out := make([]deviceJSON, 0, len(devices))
		for _, d := range devices {
			out = append(out, toDeviceJSON(d))
		}
		if err := json.NewEncoder(stdout).Encode(out); err != nil {
			return 1
		}
		return 0
	}

	if len(devices) == 0 {
		fmt.Fprintln(stdout, "(no AirPlay devices visible)")
		return 0
	}

	// Compute the longest name for left alignment.
	maxName := 0
	for _, d := range devices {
		if len(d.Name) > maxName {
			maxName = len(d.Name)
		}
	}
	for _, d := range devices {
		sel := " "
		if d.Selected {
			sel = "*"
		}
		act := " "
		if d.Active {
			act = "▶"
		}
		line := fmt.Sprintf("%s%s %-*s (%s)", sel, act, maxName, d.Name, d.Kind)
		if !d.Available {
			line += "  [unavailable]"
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// Forward decls so the file compiles before T16/T17 add the impls.
// Bodies are intentionally empty — neither is dispatched-to by any T15 test.
// (Go does not require unused parameters to be discarded.)
func cmdTargetsGet(args []string, client music.Client, stdout, stderr io.Writer) int {
	return 1 // Implemented in Task 16.
}

func cmdTargetsSet(args []string, client music.Client, stderr io.Writer) int {
	return 1 // Implemented in Task 17.
}
```

The forward declarations of `cmdTargetsGet` and `cmdTargetsSet` keep the `targets.go` file compilable while we incrementally fill them in. They'll be replaced in Tasks 16 and 17.

In `internal/cli/cli.go`, add `case "targets": return cmdTargets(args[1:], client, stdout, stderr)` to the switch BEFORE the `case "help"` line. The switch should now look like:

```go
	switch args[0] {
	case "status":
		return cmdStatus(args[1:], client, stdout, stderr)
	case "toggle":
		return cmdToggle(client, stderr)
	case "next":
		return cmdNext(client, stderr)
	case "prev":
		return cmdPrev(client, stderr)
	case "launch":
		return cmdLaunch(client, stderr)
	case "volume":
		return cmdVolume(args[1:], client, stderr)
	case "targets":
		return cmdTargets(args[1:], client, stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usageText)
		return 0
	default:
		fmt.Fprintf(stderr, "goove: unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, usageText)
		return 1
	}
```

Also update `usageText` in `cli.go` to include the new line. Find the line `goove launch                Launch Apple Music if not running` and add AFTER it:

```
  goove targets list|get|set [name]   Inspect or change the AirPlay device
```

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test ./internal/cli/... -run TestTargets
```

Expected: 7 sub-tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/targets.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "cli: targets dispatcher + targets list (plain + JSON)"
```

---

### Task 16: `cmdTargetsGet` (plain + JSON)

**Files:**
- Modify: `internal/cli/targets.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/cli_test.go`:

```go
func TestTargetsGetPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
		{Name: "Kitchen Sonos", Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if strings.TrimSpace(stdout.String()) != "Kitchen Sonos" {
		t.Errorf("stdout = %q; want 'Kitchen Sonos'", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestTargetsGetJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get", "--json"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if got["name"] != "Kitchen Sonos" {
		t.Errorf("name = %v; want Kitchen Sonos", got["name"])
	}
	if got["selected"] != true {
		t.Errorf("selected = %v; want true", got["selected"])
	}
}

func TestTargetsGetNoneSelectedExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1 (no device selected)", code)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/cli/... -run TestTargetsGet
```

Expected: tests fail — `cmdTargetsGet` is the placeholder that returns 1 with no output.

- [ ] **Step 3: Implement cmdTargetsGet**

In `internal/cli/targets.go`, REPLACE the placeholder `cmdTargetsGet` with:

```go
func cmdTargetsGet(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	device, err := client.CurrentAirPlayDevice(context.Background())
	if err != nil {
		// ErrDeviceNotFound is a meaningful state report ("nothing selected"),
		// but for `get` we treat it as a 1-exit since there's no name to print.
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(toDeviceJSON(device)); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, device.Name)
	return 0
}
```

Also DELETE the unused `errors` import if it's only there for the old placeholder's `errors.New("not implemented")` — go build will tell you.

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test ./internal/cli/...
```

Expected: 3 new TestTargetsGet* tests pass + every existing test still passes.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/targets.go internal/cli/cli_test.go
git commit -m "cli: targets get (plain + JSON)"
```

---

### Task 17: `cmdTargetsSet` (silent on success, error mapping for not-found / ambiguous)

**Files:**
- Modify: `internal/cli/targets.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/cli_test.go`:

```go
func TestTargetsSetSuccess(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Kitchen Sonos"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	// Verify side effect on the fake.
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Kitchen Sonos" {
		t.Errorf("current = %q; want Kitchen Sonos", cur.Name)
	}
}

func TestTargetsSetMissingNameExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a device name") {
		t.Errorf("stderr missing 'requires a device name': %q", stderr.String())
	}
}

func TestTargetsSetNotFoundExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Atlantis"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "airplay device not found: Atlantis") {
		t.Errorf("stderr missing 'not found: Atlantis': %q", stderr.String())
	}
}

func TestTargetsSetAmbiguousExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Kitchen Sonos"},
		{Name: "Office Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "sonos"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "matches multiple") {
		t.Errorf("stderr missing 'matches multiple': %q", got)
	}
	if !strings.Contains(got, "Kitchen Sonos") || !strings.Contains(got, "Office Sonos") {
		t.Errorf("stderr should list both matches: %q", got)
	}
}

func TestTargetsSetExactMatchPriority(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Living Room", Selected: false},
		{Name: "Living Room Speakers", Selected: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Living Room"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0 (exact match should win, no ambiguity)", code)
	}
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Living Room" {
		t.Errorf("current = %q; want exact 'Living Room'", cur.Name)
	}
}

func TestTargetsSetSubstringMatch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "kitchen"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Kitchen Sonos" {
		t.Errorf("current = %q; want Kitchen Sonos (resolved from 'kitchen')", cur.Name)
	}
}
```

**Important caveat:** the ambiguous-set test asserts that the stderr lists BOTH matches. Currently `errorExit` only handles `ErrAmbiguousDevice` with a generic "<err>" message. The CLI's `cmdTargetsSet` needs to produce a more detailed message specifically for the ambiguous case (listing which devices matched). That logic lives in `cmdTargetsSet` itself, NOT in `errorExit`.

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/cli/... -run TestTargetsSet
```

Expected: tests fail — `cmdTargetsSet` is still the placeholder.

- [ ] **Step 3: Implement cmdTargetsSet**

In `internal/cli/targets.go`, REPLACE the placeholder `cmdTargetsSet` with:

```go
func cmdTargetsSet(args []string, client music.Client, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: targets set requires a device name")
		return 1
	}
	name := args[0]

	err := client.SetAirPlayDevice(context.Background(), name)
	if err == nil {
		return 0
	}

	// Ambiguous needs a richer message — list all matching device names.
	if errors.Is(err, music.ErrAmbiguousDevice) {
		devices, listErr := client.AirPlayDevices(context.Background())
		if listErr == nil {
			fmt.Fprintf(stderr, "goove: %q matches multiple devices:\n", name)
			lower := strings.ToLower(name)
			for _, d := range devices {
				if strings.Contains(strings.ToLower(d.Name), lower) {
					fmt.Fprintf(stderr, "  %s\n", d.Name)
				}
			}
			return 1
		}
		// Fallback if we can't re-fetch — generic message.
		fmt.Fprintf(stderr, "goove: %q matches multiple devices\n", name)
		return 1
	}

	// Not-found gets a name-tagged message.
	if errors.Is(err, music.ErrDeviceNotFound) {
		fmt.Fprintf(stderr, "goove: airplay device not found: %s\n", name)
		return 1
	}

	// Everything else through the standard helper.
	return errorExit(err, stderr, true)
}
```

- [ ] **Step 4: Run all package tests, verify pass**

Run:
```bash
go test ./internal/cli/...
```

Expected: every test passes — 6 new TestTargetsSet* tests + everything else.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/targets.go internal/cli/cli_test.go
git commit -m "cli: targets set with ambiguous-match listing + not-found message"
```

---

## Phase 7 — Final verification

### Task 18: Full project verification + smoke test

- [ ] **Run the full unit test suite**

Run:
```bash
go test -count=1 ./...
```

Expected: every package passes.

- [ ] **Run -race**

Run:
```bash
go test -count=1 -race ./...
```

Expected: every package passes under the race detector.

- [ ] **Run vet**

Run:
```bash
go vet ./...
```

Expected: no output.

- [ ] **Run integration tests against live Music.app (optional, requires permission)**

Run:
```bash
go test -count=1 -tags=integration ./...
```

Expected: passes. The new `TestIntegrationAirPlayDevicesRoundtrip` logs the device list and current selection without changing audio routing.

- [ ] **Build and smoke-test the binary**

Run:
```bash
go build -o goove ./cmd/goove
./goove --help          # verify usage line includes "goove targets list|get|set"
./goove targets list    # plain output with markers
./goove targets list --json | jq .
./goove targets get     # name of currently-selected device
./goove targets get --json | jq .
./goove targets set "<name of a real device>"   # silent, exit 0
```

Then back in the TUI:

```bash
./goove                 # launch TUI; press 'o' to open picker
                        # arrow keys / j-k to navigate
                        # enter to select
                        # esc to cancel
                        # transport keys (space/n/p/+/-) should be ignored while picker is open
                        # q closes picker (does NOT quit the app while picker is open)
```

Verify visually:
- Picker title "Pick an output device" appears
- Currently-selected device has the `*` marker
- Cursor (▶) lands on the currently-selected device when picker opens
- Cursor moves with arrow keys / j-k
- Picker closes on enter (after a successful set) or esc/q
- After enter on a different device, the next 1Hz tick reflects the change
- Footer at the bottom of the player view now includes `o: output`

If the keybind footer wraps in a way that looks awkward at 80 cols, bump `compactThreshold` in `view.go` from 50 to ~65 (or whatever makes the footer fit). Decision based on observation, not a pre-committed value.

- [ ] **Confirm branch is ready**

Run:
```bash
git log main..feature/audio-targets --oneline
git status
```

Expected: ~16 commits ahead of main (one per task in Phases 2-7), clean working tree.

## Pause-point debrief (for the implementer)

The user is in pause-point mode and is comfortable with all existing patterns. ONE concept in this plan is genuinely new and worth a brief debrief after Task 14 (when the picker becomes visible end-to-end):

**Modal-overlay state in Bubble Tea.** `m.picker != nil` is the single source of truth for "is the picker open." When non-nil, View renders the picker INSTEAD of the player view, and `handleKey` routes ALL keystrokes through `handlePickerKey` (suppressing transport keys completely). This is the pattern any future modal in goove (settings menu, search, queue editor) should follow:

1. A pointer-typed field on `Model` (`*modalState`) that's nil when closed.
2. View checks the modal field BEFORE the regular state-based render path.
3. `handleKey` checks the modal field and routes to a modal-specific dispatcher BEFORE the regular key switch.
4. Messages tagged with the modal's purpose (`devicesMsg`, `deviceSetMsg`) include their own discard logic when the modal is closed (user esc'd before async work completed).

The pattern is roughly equivalent to React's "modal portal + key trap" or Vim's modal command-line — the data-flow shape is the same: input is routed differently based on which mode you're in, and exiting the mode is always one keystroke (`esc`).

## Out of scope for v1 (deliberately)

- Multi-device selection (party mode)
- macOS system audio output via SwitchAudioSource
- Per-device volume control
- Persisted favourites
- A "switching to X..." toast in the TUI after a successful set
- `o` keybind launching Music when in `Disconnected` state
- TUI picker rendering inside the existing card layout (it's full-screen for v1)
- Tab completion for `goove targets set <tab>`
