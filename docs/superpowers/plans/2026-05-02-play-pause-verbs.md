# goove Play/Pause Verbs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `goove play` and `goove pause` as separate CLI subcommands alongside the existing `goove toggle`. Widens `music.Client` with `Play()` and `Pause()` methods.

**Architecture:** Mirrors the existing `toggle`/`next`/`prev` shape exactly. Two new interface methods, two new AppleScript constants, two new fake counters/methods, two new CLI handlers, two new dispatch cases. No new files, no new patterns. Smallest spec in the codebase.

**Tech Stack:** Go 1.24, stdlib only. Spec: `docs/superpowers/specs/2026-05-02-play-pause-verbs-design.md`.

---

## File Structure

```
goove/
└── internal/
    ├── music/
    │   ├── client.go                                  # T2 — ADD: Play, Pause to interface
    │   ├── applescript/
    │   │   ├── scripts.go                             # T3 — ADD: scriptPlay, scriptPause constants
    │   │   ├── client.go                              # T3 — ADD: Play, Pause methods
    │   │   └── client_test.go                         # T3 — ADD: 2 tests
    │   └── fake/
    │       ├── client.go                              # T4 — ADD: PlayCalls/PauseCalls counters + methods
    │       └── client_test.go                         # T4 — ADD: 4 tests
    └── cli/
        ├── transport.go                               # T5 — ADD: cmdPlay, cmdPause
        ├── cli.go                                     # T5 — MODIFY: dispatcher + usageText
        └── cli_test.go                                # T5 — ADD: 6 tests
```

`cmd/goove/main.go` is untouched.

## Naming and signature contract

| Symbol | Definition |
|---|---|
| `music.Client.Play(ctx) error` | New interface method |
| `music.Client.Pause(ctx) error` | New interface method |
| `applescript.scriptPlay` | `tell application "Music" to play` |
| `applescript.scriptPause` | `tell application "Music" to pause` |
| `applescript.Client.Play(ctx) error` | Wraps c.run with scriptPlay |
| `applescript.Client.Pause(ctx) error` | Wraps c.run with scriptPause |
| `fake.Client.PlayCalls` | Exported int counter |
| `fake.Client.PauseCalls` | Exported int counter |
| `fake.Client.Play(ctx) error` | Honours forcedErr → running → counter++ |
| `fake.Client.Pause(ctx) error` | Same shape |
| `cli.cmdPlay(client, stderr) int` | Wraps Play; errorExit on failure |
| `cli.cmdPause(client, stderr) int` | Same shape |

---

## Phase 1 — Bootstrap

### Task 1: Create feature branch and verify clean starting state

**No files modified.**

- [ ] **Step 1: Create the feature branch from main**

Run:
```bash
git checkout main
git checkout -b feature/play-pause-verbs
```

DO NOT run `git pull` — local main may be ahead of origin (the play/pause spec hasn't been pushed yet).

- [ ] **Step 2: Confirm spec/plan are present and tree is clean**

Run:
```bash
ls docs/superpowers/specs/2026-05-02-play-pause-verbs-design.md
ls docs/superpowers/plans/2026-05-02-play-pause-verbs.md
git status
git log -3 --format='%h %s'
```

Expected: both files present; tree clean (or only `.claude/`).

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

## Phase 2 — Interface widening

### Task 2: Add `Play` and `Pause` to `music.Client`

**Files:**
- Modify: `internal/music/client.go`

This commit INTENTIONALLY breaks the build — `applescript.Client` and `fake.Client` will fail their compile-time interface guards until Tasks 3 and 4 add the new methods.

- [ ] **Step 1: Add the two methods to the interface**

In `internal/music/client.go`, inside the `Client` interface, AFTER `SetAirPlayDevice` (the last method added in audio-targets), add:

```go
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
```

The interface now has 13 methods.

- [ ] **Step 2: Verify build fails as expected**

Run:
```bash
go build ./...
```

Expected: errors mentioning `*Client does not implement music.Client (missing method Play)` for both `applescript.Client` and `fake.Client`.

- [ ] **Step 3: Commit**

```bash
git add internal/music/client.go
git commit -m "music: add Play and Pause to Client interface"
```

---

## Phase 3 — AppleScript implementation

### Task 3: scriptPlay/scriptPause constants + Client.Play/Pause methods + tests

**Files:**
- Modify: `internal/music/applescript/scripts.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Append the two script constants**

Append to `internal/music/applescript/scripts.go`:

```go
const scriptPlay  = `tell application "Music" to play`
const scriptPause = `tell application "Music" to pause`
```

- [ ] **Step 2: Write failing tests**

Append to `internal/music/applescript/client_test.go`:

```go
func TestPlayRunsPlayScript(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	if err := c.Play(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.script != scriptPlay {
		t.Errorf("ran %q; want scriptPlay", r.script)
	}
}

func TestPauseRunsPauseScript(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	if err := c.Pause(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.script != scriptPause {
		t.Errorf("ran %q; want scriptPause", r.script)
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run:
```bash
go test ./internal/music/applescript/ -run "TestPlay|TestPause"
```

Expected: build fails — `Client has no field or method Play`.

- [ ] **Step 4: Implement Play and Pause on Client**

Append to `internal/music/applescript/client.go` (BEFORE the `var _ music.Client = (*Client)(nil)` guard at the bottom):

```go
func (c *Client) Play(ctx context.Context) error {
	_, err := c.run(ctx, scriptPlay)
	return err
}

func (c *Client) Pause(ctx context.Context) error {
	_, err := c.run(ctx, scriptPause)
	return err
}
```

- [ ] **Step 5: Run tests, verify pass**

Run:
```bash
go test ./internal/music/applescript/...
```

Expected: every existing test + the 2 new ones pass. The compile-time guard for applescript.Client now compiles cleanly. (fake.Client still fails — Task 4 fixes it.)

Run also:
```bash
go build ./internal/music/applescript/...
```

Expected: succeeds (fake.Client is in a different package so its failure doesn't block applescript building).

- [ ] **Step 6: Commit**

```bash
git add internal/music/applescript/scripts.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: scriptPlay/scriptPause constants + Client methods"
```

---

## Phase 4 — Fake implementation

### Task 4: `fake.Client.Play` and `fake.Client.Pause` + counters + tests

**Files:**
- Modify: `internal/music/fake/client.go`
- Modify: `internal/music/fake/client_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/music/fake/client_test.go`:

```go
func TestPlayIncrementsCounter(t *testing.T) {
	c := New()
	c.Launch(context.Background())

	if err := c.Play(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PlayCalls != 1 {
		t.Errorf("PlayCalls = %d; want 1", c.PlayCalls)
	}
}

func TestPauseIncrementsCounter(t *testing.T) {
	c := New()
	c.Launch(context.Background())

	if err := c.Pause(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PauseCalls != 1 {
		t.Errorf("PauseCalls = %d; want 1", c.PauseCalls)
	}
}

func TestPlayNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New() // not launched
	err := c.Play(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestPauseNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New() // not launched
	err := c.Pause(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/music/fake/...
```

Expected: build fails — `Client has no field or method Play` (and similar for Pause, PlayCalls, PauseCalls).

- [ ] **Step 3: Implement Play and Pause on fake.Client**

In `internal/music/fake/client.go`:

a. ADD two new fields to the `Client` struct alongside the existing counters (PlayPauseCalls, NextCalls, PrevCalls, SetVolumeCalls, LaunchCalls):

```go
	PlayCalls       int
	PauseCalls      int
```

b. ADD two new methods. Place them adjacent to the existing `PlayPause` method (so all play-state methods live together):

```go
// Play implements music.Client.
func (c *Client) Play(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PlayCalls++
	return nil
}

// Pause implements music.Client.
func (c *Client) Pause(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	c.PauseCalls++
	return nil
}
```

The compile-time guard `var _ music.Client = (*Client)(nil)` at the bottom of `fake/client.go` will now compile cleanly.

- [ ] **Step 4: Run tests, verify pass**

Run:
```bash
go test -race ./internal/music/fake/...
```

Expected: every existing test + the 4 new ones pass with the race detector on.

Run also:
```bash
go build ./...
```

Expected: succeeds across the whole module — Tasks 2-3 had broken the build; this task finishes unbreaking it.

- [ ] **Step 5: Commit**

```bash
git add internal/music/fake/client.go internal/music/fake/client_test.go
git commit -m "music/fake: Play/Pause methods + counters"
```

---

## Phase 5 — CLI surface

### Task 5: `cmdPlay` + `cmdPause` + dispatch + usageText + tests

**Files:**
- Modify: `internal/cli/transport.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/cli_test.go`:

```go
func TestPlaySuccessSilentExit0(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if c.PlayCalls != 1 {
		t.Errorf("PlayCalls = %d; want 1", c.PlayCalls)
	}
}

func TestPlayNotRunningExit1WithHint(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "goove launch") {
		t.Errorf("stderr missing 'goove launch' hint: %q", stderr.String())
	}
}

func TestPlayPermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "not authorised") {
		t.Errorf("stderr missing permission message: %q", stderr.String())
	}
}

func TestPauseSuccessSilentExit0(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if c.PauseCalls != 1 {
		t.Errorf("PauseCalls = %d; want 1", c.PauseCalls)
	}
}

func TestPauseNotRunningExit1WithHint(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestPausePermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
}
```

`setupRunningClient`, `bytes`, `strings`, `context`, `music`, `fake` are all already imported in this file from prior tasks. No new imports.

- [ ] **Step 2: Run, verify failure**

Run:
```bash
go test ./internal/cli/... -run "TestPlay|TestPause"
```

Expected: tests fail — `play` and `pause` fall through to "unknown command".

- [ ] **Step 3: Add cmdPlay and cmdPause handlers**

Append to `internal/cli/transport.go`:

```go
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

- [ ] **Step 4: Add dispatch cases + update usageText**

In `internal/cli/cli.go`:

a. ADD two new cases to the switch in `Run`. Place them BEFORE `case "toggle":`:

```go
	case "play":
		return cmdPlay(client, stderr)
	case "pause":
		return cmdPause(client, stderr)
	case "toggle":
		return cmdToggle(client, stderr)
```

b. UPDATE `usageText` to include the two new lines. Find the existing `goove toggle` line and ADD TWO LINES BEFORE it:

```
  goove play                  Start playback (no-op if already playing)
  goove pause                 Pause playback (no-op if already paused)
  goove toggle                Play/pause toggle
```

(The existing toggle line stays; you're adding play and pause above it.)

- [ ] **Step 5: Run all tests, verify pass**

Run:
```bash
go test ./internal/cli/...
```

Expected: every existing test + the 6 new ones pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/transport.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "cli: play and pause subcommands"
```

---

## Phase 6 — Final verification

### Task 6: Full project verification + smoke test

- [ ] **Run vet, test, race**

```bash
go vet ./...
go test -count=1 ./...
go test -count=1 -race ./...
```

Expected: every check passes.

- [ ] **Run integration tests against live Music.app**

```bash
go test -count=1 -tags=integration ./...
```

Expected: passes (no new integration tests; this just confirms nothing regressed).

- [ ] **Build the binary and smoke-test the new commands**

```bash
go build -o goove ./cmd/goove
./goove --help     # verify usage block now includes 'goove play' and 'goove pause' lines
```

With Music.app open and a track loaded:

```bash
./goove play       # start playback (silent, exit 0); audibly verify Music starts/continues
./goove play       # second call — still silent, exit 0 (idempotent at AppleScript level)
./goove pause      # pause (silent, exit 0); audibly verify Music pauses
./goove pause      # idempotent — silent, exit 0
./goove toggle     # toggles between play/pause as before
```

With Music quit:

```bash
./goove play       # exit 1, stderr "isn't running (run 'goove launch' first)"
./goove pause      # same
```

If anything's off, inspect the cmd handler and fix before merging.

- [ ] **Confirm branch is ready**

```bash
git log main..feature/play-pause-verbs --oneline
git status
```

Expected: 4 commits ahead of main (T2 through T5), clean working tree.

## Out of scope (deliberately)

- TUI changes — space-bar toggle stays the natural in-app gesture
- A `--idempotent-check` flag — AppleScript already handles the no-op case
- Removing or deprecating `goove toggle` — it stays
