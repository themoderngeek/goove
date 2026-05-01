# goove — Album Art Panel Design

**Date:** 2026-05-01
**Status:** Approved
**Builds on:** [goove MVP design](2026-04-30-goove-mvp-design.md)
**Module path:** `github.com/themoderngeek/goove`

## 1. Summary

Adds an album-art panel to goove's `Connected` state. The panel sits to the left of the existing now-playing card (~20 cell-pixels × 10 character-rows of half-block ANSI), rendered by shelling out to `chafa` from PNG bytes pulled directly out of Music.app via AppleScript. In-memory cache keyed by `(title|artist|album)` means we only fetch + render on track change, not per status sync. Every failure mode (chafa missing, track has no artwork, AppleScript fault) silently degrades to the existing no-art card layout.

This expands the v1 MVP's explicit non-goal "Album art rendering" — but only that single non-goal. The other v1 non-goals (search, playlists, audio-target switching) remain out of scope.

## 2. Goals

**This iteration must do:**

- Fetch the current track's PNG artwork bytes from Music.app's local data
- Render those bytes through `chafa` half-block to a fixed 20×10-cell ANSI string
- Composite that string side-by-side with the existing now-playing card via `lipgloss.JoinHorizontal`
- Cache the rendered string in memory keyed by track identity; refetch only when the track changes
- Detect `chafa` availability once at startup; if absent, never fetch art and log the install hint
- Detect `track has no artwork`; degrade to no-art layout for that track without retry
- Show art only in the `Connected` state

**This iteration deliberately does not do:**

- Configurable layout (the user wants this later — accepted, but not now)
- Configurable art size (fixed 20×10 cells)
- Disk cache that survives goove restarts
- Pixel-perfect rendering protocols (kitty / sixel / iTerm2 inline) — half-block only
- A `--no-art` flag (chafa-not-installed is the implicit opt-out)
- Pre-fetching for queued tracks
- Art in `Idle` state (e.g. last-played artwork while no track is loaded)

## 3. Architecture

The MVP's four-layer architecture stays. This design adds:

1. **A widening of the `music.Client` interface** — one new method `Artwork(ctx) ([]byte, error)`, plus a new sentinel `music.ErrNoArtwork`.
2. **A new sibling package `internal/art`** — independent of `music`, holding the chafa-renderer abstraction and a package-level `Available()` helper.
3. **App-layer wiring** — the `Update` handler detects track changes, the `View` composes the side-by-side layout when art is available.

```
┌──────────────────────────────────────────────────────────────┐
│  TUI layer  (Bubble Tea)                                     │
│    handleStatus: detect track change → fire fetchArtwork     │
│    artworkMsg:   store rendered string keyed by track        │
│    View:         lipgloss.JoinHorizontal(art, card) when     │
│                  art is ready and width >= 70                │
└────────────────────────┬─────────────────────────────────────┘
                         │
       ┌─────────────────┴─────────────────┐
       ▼                                   ▼
┌──────────────────────┐       ┌──────────────────────────────┐
│  music.Client        │       │  art.Renderer                │
│    (gain Artwork)    │       │    Render(ctx, bytes, w, h)  │
│                      │       │      → ANSI string           │
│  applescript.Client  │       │                              │
│    runs scriptArtwork│       │  ChafaRenderer (real)        │
│    + reads cache file│       │    pipes bytes to chafa stdin│
│                      │       │  fakeChafaRunner (test seam) │
│  fake.Client         │       │                              │
│    SetArtwork()/     │       │  art.Available() — once on   │
│    Artwork()         │       │    startup; nil renderer if  │
└──────────────────────┘       │    chafa absent              │
                               └──────────────────────────────┘
```

**The wiring rule:** the app layer owns orchestration. Neither `music.Client` nor `art.Renderer` knows about the other. App calls them in sequence inside a single `tea.Cmd` (`fetchArtwork`).

### Key design choices

| Concern | Choice | Alternatives considered |
|---|---|---|
| Source of artwork | **Local Music.app via AppleScript (`raw data of artwork 1 of current track`)** | iTunes Search API (network-dependent, lookup ambiguity); MusicKit (overkill, requires Apple Developer signing); each rejected for the obvious reasons. |
| Render protocol | **Half-block via `chafa`** | Pixel-perfect (kitty/sixel/iTerm2 inline) — high fidelity but Bubble Tea's renderer doesn't understand inline-image escapes; layout integration is fragile. ASCII-only — too abstract to be useful. Half-block is the sweet spot. |
| Layout | **Side-by-side; card grows to ~70 cells** | Stacked above (no width growth, but pushes the card vertically); compact thumbnail (art too small to be useful). |
| Cache invalidation | **Keyed by `(title \| artist \| album)`; in-memory single-slot** | Persistent ID from Music (works but adds an extra AppleScript field); LRU multi-slot (we only show one track's art at a time — YAGNI); disk cache (rarely useful, not worth the complexity). |
| Render dependency | **Shell out to `chafa`** | Pure-Go image rendering libraries exist but reproducing chafa's quality is a project of its own. `chafa` is a single Homebrew dep, well-maintained, single binary. |

## 4. Module additions

```
goove/
└── internal/
    ├── art/                                        # NEW package
    │   ├── renderer.go                             # Renderer interface + ChafaRunner indirection + Available()
    │   ├── chafa.go                                # ChafaRenderer real impl
    │   ├── chafa_test.go                           # uses fakeChafaRunner mock
    │   └── chafa_integration_test.go               # //go:build integration: real chafa + fixture PNG
    ├── app/
    │   ├── messages.go                             # ADD: artworkMsg
    │   ├── tick.go                                 # ADD: fetchArtwork Cmd factory + artWidth/artHeight + artLayoutThreshold
    │   ├── model.go                                # ADD: artState struct, Model.art, Model.renderer; CHANGE: New(client, renderer)
    │   ├── update.go                               # ADD: handleStatus track-change detection; ADD: artworkMsg handler
    │   ├── update_test.go                          # ADD: full suite of artwork-related tests
    │   └── view.go                                 # MODIFY: renderConnected uses lipgloss.JoinHorizontal when art present
    └── music/
        ├── client.go                               # ADD: Artwork(ctx) ([]byte, error); ADD: ErrNoArtwork sentinel
        ├── applescript/
        │   ├── scripts.go                          # ADD: scriptArtwork constant
        │   ├── client.go                           # ADD: Artwork(ctx) impl
        │   ├── client_test.go                      # ADD: TestArtwork* family
        │   └── client_integration_test.go          # ADD: TestIntegrationArtwork
        └── fake/
            ├── client.go                           # ADD: SetArtwork(bytes), SetArtworkErr(err), Artwork(ctx)
            └── client_test.go                      # ADD: TestArtwork* tests
```

`cmd/goove/main.go` also gains four lines to construct and pass the renderer:

```go
var renderer art.Renderer
if art.Available() {
    renderer = art.NewChafaRenderer()
} else {
    slog.Info("chafa not found in PATH; album art disabled (install with: brew install chafa)")
}
model := app.New(client, renderer)
```

## 5. Key types

### `internal/art`

```go
package art

// Renderer turns image bytes into an ANSI string suitable for embedding directly
// in a Bubble Tea View.
type Renderer interface {
    Render(ctx context.Context, image []byte, width, height int) (string, error)
}

// Available reports whether chafa is in PATH. Cheap, but should be invoked once
// at startup; main caches the result.
func Available() bool

// ChafaRunner is the test seam — same indirection pattern as music/applescript.Runner.
type ChafaRunner interface {
    Run(ctx context.Context, image []byte, width, height int) ([]byte, error)
}

type ChafaRenderer struct{ runner ChafaRunner }

func NewChafaRenderer() *ChafaRenderer  // uses execChafaRunner (the real impl)
func New(runner ChafaRunner) *ChafaRenderer  // injectable for tests
```

`ChafaRenderer.Render` wraps the runner with `context.WithTimeout(ctx, renderTimeout)` (constant `renderTimeout = 2 * time.Second`).

### `internal/music`

```go
type Client interface {
    // existing seven methods unchanged
    Artwork(ctx context.Context) ([]byte, error)
}

var ErrNoArtwork = errors.New("music: track has no artwork")
```

### `internal/music/applescript`

```go
// scriptArtwork writes the current track's artwork bytes to a fixed cache file
// at ~/Library/Caches/goove/artwork.bin and returns:
//   - "NOT_RUNNING" if Music isn't running
//   - "NO_ART"      if the current track has no artwork
//   - "OK"          on success
const scriptArtwork = `tell application "Music"
    if not running then return "NOT_RUNNING"
    try
        set theArt to artwork 1 of current track
    on error
        return "NO_ART"
    end try
    set artData to (raw data of theArt)
    set fileRef to open for access POSIX file "%s" with write permission
    try
        set eof of fileRef to 0
        write artData to fileRef
        close access fileRef
    on error errMsg
        try
            close access fileRef
        end try
        error errMsg
    end try
    return "OK"
end tell`
```

`Client.Artwork(ctx)`:
1. Computes `~/Library/Caches/goove/artwork.bin`, ensures parent dir exists
2. Runs `fmt.Sprintf(scriptArtwork, path)` via the existing `c.run` helper
3. Switches on stdout: `NOT_RUNNING` → `ErrNotRunning`; `NO_ART` → `ErrNoArtwork`; `OK` → reads the file → returns the bytes

The `raw data of` form is verified to produce direct PNG bytes (validated against macOS 26.4.1's Music.app — 800×800 PNG, byte-identical to the official `data of` form, chafa renders it cleanly).

### `internal/music/fake`

```go
func (c *Client) SetArtwork(bytes []byte)
func (c *Client) SetArtworkErr(err error)  // for forcing ErrNoArtwork etc.
func (c *Client) Artwork(ctx context.Context) ([]byte, error)
```

The existing compile-time guard `var _ music.Client = (*Client)(nil)` catches the new method's signature.

### `internal/app`

```go
// In messages.go:
type artworkMsg struct {
    key    string
    output string
    err    error
}

// In tick.go:
const (
    artWidth           = 20
    artHeight          = 10
    artLayoutThreshold = 70  // terminal width below this → no-art card layout
)

func fetchArtwork(client music.Client, renderer art.Renderer, key string) tea.Cmd

// In model.go:
type artState struct {
    key      string
    output   string
    fetching bool
}

type Model struct {
    // existing fields...
    art      artState
    renderer art.Renderer  // nil when chafa is unavailable
}

func New(client music.Client, renderer art.Renderer) Model

// Pure helpers (in view.go alongside existing pure helpers):
func trackKey(t domain.Track) string
func (m Model) currentArtKey() string
```

`trackKey` returns `""` for an all-zero `Track` (so "no track" doesn't accidentally hash to the same key as a real one). For a real track it returns `title + "|" + artist + "|" + album`.

## 6. Runtime behaviour

### Track-change detection

In `handleStatus`, after the existing state transition logic, add:

```go
// Defer this until after the state transition so we operate on the new track.
newKey := trackKey(msg.now.Track)
if m.renderer != nil &&
   newKey != "" &&
   newKey != m.art.key &&
   !m.art.fetching {
    m.art = artState{key: newKey, fetching: true}
    cmd = tea.Batch(cmd, fetchArtwork(m.client, m.renderer, newKey))
}
```

Four conjuncts: `renderer != nil` (chafa available), `newKey != ""` (real track), `newKey != m.art.key` (it's a new track), `!m.art.fetching` (not already in flight).

### artworkMsg handler

```go
case artworkMsg:
    currentKey := m.currentArtKey()
    if msg.key != currentKey {
        return m, nil  // stale — discard silently
    }
    if msg.err != nil {
        slog.Debug("artwork unavailable", "track", msg.key, "err", msg.err)
    }
    m.art = artState{
        key:    msg.key,
        output: msg.output,  // "" on any error path
    }
    return m, nil
```

`fetching` is implicitly cleared because we replace `m.art` with a fresh `artState` that has `fetching: false` (zero value).

### View composition

```go
func (m Model) View() string {
    if m.permissionDenied { return renderPermissionDenied() }
    if m.width > 0 && m.width < compactThreshold { return renderCompact(m) }

    switch s := m.state.(type) {
    case Connected:
        card := renderConnected(s, m.errFooter())  // existing function, unchanged
        if m.width >= artLayoutThreshold &&
           m.art.output != "" &&
           m.art.key == trackKey(s.Now.Track) {
            return lipgloss.JoinHorizontal(lipgloss.Top, m.art.output, "  ", card)
        }
        return card
    // Idle / Disconnected unchanged
    }
    return ""
}
```

`renderConnected` is unchanged — it still returns the card body the way it does today. The art-vs-no-art branch lives in `View` itself, not a new helper, so this is a strictly additive change to the file.

The triple-condition guard on the side-by-side branch:
- `m.width >= artLayoutThreshold` — terminal is wide enough
- `m.art.output != ""` — we have rendered art (not "no art for this track")
- `m.art.key == trackKey(s.Now.Track)` — defensive: never show stale art for a new track

Width thresholds:

| Terminal width | Layout |
|---|---|
| `< compactThreshold` (50) | compact one-liner (existing) |
| `compactThreshold ≤ width < artLayoutThreshold` (50–69) | full card without art |
| `≥ artLayoutThreshold` (70) | full card with art beside it |

### Cost profile

- Per status sync (1Hz): one extra string compare and zero-or-one Cmd dispatch.
- Per track change: one `fetchArtwork` Cmd that runs serially in a goroutine. Empirically: AppleScript artwork-write ~80ms, chafa render ~50ms, total ~130ms.
- Per View render (4Hz repaint): zero — re-uses the cached `m.art.output` string.

## 7. Error handling and edge cases

The artwork pipeline is supplementary to the core TUI: every failure path produces "no-art card layout for this track."

### Error matrix

| Error | Cause | Behaviour |
|---|---|---|
| `music.ErrNotRunning` | Music quit between status sync and artwork fetch | Art slot empty. The next status tick handles the actual `Disconnected` transition. |
| `music.ErrNoArtwork` | Track has no embedded artwork (some streamed tracks, podcasts, etc.) | Art slot empty. **No retry for this track.** Next track triggers a fresh fetch. |
| `music.ErrPermission` | Permission denied between status sync and artwork fetch | Art slot empty. The next status tick handles the actual `permissionDenied` transition. |
| `music.ErrUnavailable` | AppleScript fault, file-IO error | Art slot empty. Logged at DEBUG. |
| any error from `art.Renderer.Render` (wrapped chafa error) | chafa exited non-zero, timed out, output unparseable | Art slot empty. Logged at DEBUG. |

### Edge cases

| Situation | Behaviour |
|---|---|
| User skips quickly: A → B → C while A's fetch is in flight | Stale `artworkMsg{key:"A"}` arrives, `currentArtKey() == "C"` — discarded. The C status tick re-detects "art for C not present" and fires a fresh fetch. |
| Same track plays twice in a row | `trackKey` matches, no fetch fires; cached art remains visible seamlessly. |
| chafa not installed | `art.Available() == false` at startup; `main.go` passes `nil` renderer; track-change check short-circuits on `m.renderer != nil`; never fetches. One INFO log at startup. View permanently shows the no-art layout. |
| chafa installed but errors at runtime (corrupt image, weird format) | `m.art.output == ""`; View falls through to no-art layout for this track. |
| Two in-flight fetches | Suppressed by the `!m.art.fetching` guard. The second status sync sees `fetching: true` and skips. |
| Terminal width 50–69 cells (between thresholds) | Card without art shown — no clipping, no half-rendered art. |

## 8. Testing strategy

Three layers, mirroring the architecture.

### `internal/art` — unit tests with fakeChafaRunner

```go
type fakeChafaRunner struct { script []byte; w, h int; out []byte; err error }
```

Tests:
- `TestRenderPipesBytesAndCallsChafaWithSize` — bytes flow through; runner sees w/h
- `TestRenderReturnsRunnerOutput`
- `TestRenderTimeoutHonoured`
- `TestRenderRunnerErrorWrapped`
- `TestAvailableReturnsTrueWhenChafaInPath` (uses `exec.LookPath`)
- `TestAvailableReturnsFalseWhenChafaMissing` (manipulates a fake `PATH`)

Plus `chafa_integration_test.go` (`//go:build integration`): a small fixture PNG checked into `testdata/`, real chafa, asserts non-empty ANSI output containing escape codes.

### `internal/music/applescript` — unit tests with the existing fakeRunner

Tests added to `client_test.go`:
- `TestArtworkRunsArtworkScript` — assert the formatted scriptArtwork is what got run
- `TestArtworkOnOKReadsCacheFile` — pre-populate the cache file with fixture bytes, assert returned `[]byte` matches
- `TestArtworkOnNoArtSentinelReturnsErrNoArtwork`
- `TestArtworkOnNotRunningReturnsErrNotRunning`

Plus an integration test: `TestIntegrationArtwork` — live Music.app, log byte count, decode via `image.DecodeConfig` to verify the bytes are a valid image.

### `internal/music/fake` — unit tests

- `TestArtworkAfterSetArtworkReturnsBytes`
- `TestArtworkWithoutSetReturnsErrNoArtwork`
- `TestSetArtworkErrOverridesBytes`

### `internal/app` — extend `update_test.go`

- `TestStatusMsgWithNewTrackFiresFetchArtwork`
- `TestStatusMsgWithSameTrackDoesNotRefireFetchArtwork`
- `TestStatusMsgFiresNothingWhenRendererNil`
- `TestStatusMsgWithEmptyTrackDoesNotFireFetchArtwork`
- `TestArtworkMsgStoresOutput`
- `TestArtworkMsgWithStaleKeyDiscarded`
- `TestArtworkMsgWithErrorClearsFetchingAndLeavesOutputEmpty`
- `TestTrackKeyReturnsEmptyForZeroTrack`

No View tests for the side-by-side rendering itself — visual correctness is verified by running goove. A golden-file test could be added later if regressions become an issue.

## 9. Architectural decisions log

| Decision | Why |
|---|---|
| Local Music.app artwork over iTunes Search API or MusicKit | Always matches what Music shows; offline; no API key; no track-match guessing. |
| Half-block via chafa over pixel-perfect protocols | Drops cleanly into Bubble Tea View as plain ANSI text; no inline-image escape sequences to confuse the renderer's layout math; works in every Mac terminal. |
| Side-by-side over stacked or compact layouts | The whole point of adding art is the visual; it deserves the real estate. The 70-cell trade-off is acceptable because compact-mode already exists for narrow terminals. |
| Separate `art` package over folding into `music/applescript` | Rendering bytes to ANSI has nothing to do with the source. Same testability seam as `music/applescript.Runner`. Reusable if we ever swap Music.app for another backend. |
| Single-slot in-memory cache over LRU or disk | We display one track's art at a time. No persistent need: reopening goove is rare and a 130ms fetch is fine. |
| Track key as `title\|artist\|album` over Music's persistent ID | Simpler. Collision-safe enough for personal use. Doesn't require an extra AppleScript field. |
| `Renderer` interface returns `string` (not `[]byte`) | The only consumer is the Bubble Tea View, which wants strings. No double-conversion. |
| Stale-result handling at the message handler, not the Cmd | The Cmd is fire-and-forget. The handler knows what's "current." Tagging messages with the requested `key` lets the handler decide. |
| `m.renderer == nil` semantics | Single source of truth for "chafa unavailable." Checked once in `handleStatus`; no per-render branching. |
| 70-cell width threshold | The art (20) + padding (2) + the existing card (~46-56) ≈ 68-78. 70 is the safest floor that keeps layout clean. |

## 10. Scope notes

This spec stands alone from the MVP spec. It modifies a small number of MVP files and adds one new package; it does not touch the keybind layer, error footer, tick model, or any other MVP subsystem.

If the user later wants configurable layout (stacked vs side-by-side), configurable art size, or a `--no-art` flag, those become their own specs. The architecture here doesn't paint us into a corner: the `View`'s composition logic is local to a single function, and the renderer/cache state is well-contained on the Model.
