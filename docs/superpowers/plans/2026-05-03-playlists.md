# Playlists Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring Apple Music user + subscription playlists into goove via both a new CLI verb (`goove playlists list/tracks/play`) and a new TUI browser view reachable from now-playing via the `l` keybind.

**Architecture:** Three new methods on the existing `music.Client` interface (`Playlists`, `PlaylistTracks`, `PlayPlaylist`), implemented in both `music/applescript` (real osascript) and `music/fake` (in-memory). Two frontends consume the interface independently. The TUI gains a second top-level mode (`modeBrowser`) alongside the existing now-playing card; transport keys stay live in both modes.

**Tech Stack:** Go, Bubble Tea, AppleScript via `osascript`. No new third-party dependencies.

**Spec:** `docs/superpowers/specs/2026-05-03-playlists-design.md`

**Working branch:** `feature/playlists` (already created; spec already committed there).

---

## File map

**New files:**
- `internal/domain/playlist.go` — `Playlist` struct
- `internal/cli/playlists.go` — CLI subcommand handlers
- `internal/app/browser.go` — browser state, key handling, view rendering, fetch commands

**Modified files:**
- `internal/domain/nowplaying.go` — add `Duration` field to existing `Track`
- `internal/music/client.go` — add three methods to interface; add `ErrPlaylistNotFound`
- `internal/music/applescript/scripts.go` — three new script constants
- `internal/music/applescript/parse.go` — `parsePlaylists`, `parsePlaylistTracks`
- `internal/music/applescript/parse_test.go` — tests for new parsers
- `internal/music/applescript/client.go` — three new methods on `*Client`
- `internal/music/applescript/client_test.go` — tests for new methods
- `internal/music/applescript/client_integration_test.go` — one read-only integration test
- `internal/music/fake/client.go` — three new methods, in-memory storage, counters
- `internal/music/fake/client_test.go` — tests for the fake methods
- `internal/cli/cli.go` — dispatch `playlists`/`playlist` cases; update `usageText`
- `internal/cli/cli_test.go` — tests for CLI playlists subcommands
- `internal/app/messages.go` — three new message types
- `internal/app/model.go` — add `viewMode` enum, `mode` field, `browser` field
- `internal/app/update.go` — dispatch by mode; handle `l` to open browser
- `internal/app/view.go` — dispatch by mode
- `internal/app/update_test.go` — browser state transition tests
- `README.md` — document new CLI verbs and the `l` keybind

---

### Task 1: Add `Duration` field to `domain.Track`

**Why:** Playlist tracks need a duration. The existing `Track` struct is reused (rather than introducing a parallel `PlaylistTrack` type) — see spec, "Domain types".

**Files:**
- Modify: `internal/domain/nowplaying.go`
- Test: `internal/domain/nowplaying_test.go` (add a case)

- [ ] **Step 1: Read existing test file to learn the style**

Run: `cat internal/domain/nowplaying_test.go`

(No assertion — just orienting.)

- [ ] **Step 2: Write the failing test**

Append to `internal/domain/nowplaying_test.go`:

```go
func TestTrackDurationDefaultsToZero(t *testing.T) {
	tr := Track{Title: "X"}
	if tr.Duration != 0 {
		t.Errorf("Duration zero-value = %v; want 0", tr.Duration)
	}
}

func TestTrackCarriesDuration(t *testing.T) {
	tr := Track{Title: "X", Duration: 90 * time.Second}
	if tr.Duration != 90*time.Second {
		t.Errorf("Duration = %v; want 90s", tr.Duration)
	}
}
```

If `time` isn't already imported in this test file, add it.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestTrackCarriesDuration -v`
Expected: compile error (`Duration` not a field of `Track`).

- [ ] **Step 4: Add the `Duration` field**

In `internal/domain/nowplaying.go`, change:

```go
type Track struct {
	Title  string
	Artist string
	Album  string
}
```

to:

```go
type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration // populated by playlist tracks; left zero for NowPlaying.Track
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/ -v`
Expected: PASS for `TestTrackDurationDefaultsToZero` and `TestTrackCarriesDuration`. All other tests in the package still pass.

- [ ] **Step 6: Verify nothing else breaks**

Run: `go build ./...`
Expected: clean compile.

Run: `go test ./...`
Expected: all existing tests pass. (`Duration` zero in NowPlaying contexts is fine because nothing reads `Track.Duration` there yet.)

- [ ] **Step 7: Commit**

```bash
git add internal/domain/nowplaying.go internal/domain/nowplaying_test.go
git commit -m "domain: add Duration field to Track"
```

---

### Task 2: Add `domain.Playlist` type

**Why:** New domain type for the playlist list.

**Files:**
- Create: `internal/domain/playlist.go`
- Test: `internal/domain/playlist_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/domain/playlist_test.go`:

```go
package domain

import "testing"

func TestPlaylistZeroValue(t *testing.T) {
	var p Playlist
	if p.Name != "" || p.Kind != "" || p.TrackCount != 0 {
		t.Errorf("zero-value Playlist = %+v; want empty", p)
	}
}

func TestPlaylistFieldsAssignable(t *testing.T) {
	p := Playlist{Name: "Liked Songs", Kind: "user", TrackCount: 42}
	if p.Name != "Liked Songs" || p.Kind != "user" || p.TrackCount != 42 {
		t.Errorf("got %+v", p)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestPlaylist -v`
Expected: compile error (`undefined: Playlist`).

- [ ] **Step 3: Create the type**

Create `internal/domain/playlist.go`:

```go
package domain

// Playlist is a user or subscription playlist surfaced by the music client.
// Kind is "user" or "subscription". Smart playlists, system playlists, and
// folders are excluded from the playlists feature scope (see
// docs/superpowers/specs/2026-05-03-playlists-design.md).
type Playlist struct {
	Name       string
	Kind       string
	TrackCount int
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/playlist.go internal/domain/playlist_test.go
git commit -m "domain: add Playlist type"
```

---

### Task 3: Extend `music.Client` interface + add stubs to compile

**Why:** Locks in the new method signatures and the `ErrPlaylistNotFound` sentinel. Both `music/fake` and `music/applescript` need stub method implementations so the package compiles before the real implementations land in later tasks. The compile-time check `var _ music.Client = (*Client)(nil)` would fail otherwise.

**Files:**
- Modify: `internal/music/client.go`
- Modify: `internal/music/fake/client.go` (stub the three methods)
- Modify: `internal/music/applescript/client.go` (stub the three methods)

- [ ] **Step 1: Add the three methods to the interface and the sentinel error**

In `internal/music/client.go`, add to the `Client` interface (anywhere is fine; place after the AirPlay block for grouping):

```go
Playlists(ctx context.Context) ([]domain.Playlist, error)
PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error)
PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error
```

And add to the sentinels block:

```go
ErrPlaylistNotFound = errors.New("music: playlist not found")
```

- [ ] **Step 2: Stub the three methods on the fake**

In `internal/music/fake/client.go`, add (anywhere in the file):

```go
// Playlists implements music.Client. Real impl in Task 4.
func (c *Client) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	return nil, music.ErrUnavailable
}

// PlaylistTracks implements music.Client. Real impl in Task 4.
func (c *Client) PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error) {
	return nil, music.ErrUnavailable
}

// PlayPlaylist implements music.Client. Real impl in Task 4.
func (c *Client) PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error {
	return music.ErrUnavailable
}
```

- [ ] **Step 3: Stub the three methods on the applescript client**

In `internal/music/applescript/client.go`, add (anywhere in the file, before the `var _ music.Client = (*Client)(nil)` line):

```go
// Playlists implements music.Client. Real impl in Task 7.
func (c *Client) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	return nil, music.ErrUnavailable
}

// PlaylistTracks implements music.Client. Real impl in Task 8.
func (c *Client) PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error) {
	return nil, music.ErrUnavailable
}

// PlayPlaylist implements music.Client. Real impl in Task 9.
func (c *Client) PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error {
	return music.ErrUnavailable
}
```

- [ ] **Step 4: Verify compile and existing tests still pass**

Run: `go build ./...`
Expected: clean compile.

Run: `go test ./...`
Expected: all existing tests pass (the stubs aren't called by anyone yet).

- [ ] **Step 5: Commit**

```bash
git add internal/music/client.go internal/music/fake/client.go internal/music/applescript/client.go
git commit -m "music: add Playlists/PlaylistTracks/PlayPlaylist to Client interface"
```

---

### Task 4: Implement fake `Playlists` / `PlaylistTracks` / `PlayPlaylist` with seeded data + counters

**Why:** Frontends need a real fake to drive tests. Mirrors the existing `SetTrack`/`SetDevices` pattern.

**Files:**
- Modify: `internal/music/fake/client.go`
- Modify: `internal/music/fake/client_test.go`

- [ ] **Step 1: Write the failing test (seeded list)**

Add to `internal/music/fake/client_test.go`:

```go
func TestPlaylistsReturnsSeededList(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Liked Songs", Kind: "user", TrackCount: 3},
		{Name: "Workout", Kind: "subscription", TrackCount: 5},
	})

	got, err := c.Playlists(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "Liked Songs" || got[1].Name != "Workout" {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistsNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New()
	_, err := c.Playlists(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

If `errors` isn't imported, add it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/music/fake/ -run TestPlaylists -v`
Expected: compile error (`SetPlaylists` undefined) and runtime FAIL once compile passes.

- [ ] **Step 3: Add storage + `SetPlaylists` + replace stub `Playlists`**

In `internal/music/fake/client.go`:

Add fields to the `Client` struct:

```go
playlists       []domain.Playlist
playlistTracks  map[string][]domain.Track
playPlaylistRecord []playPlaylistCall

PlayPlaylistCalls int
```

Add the type:

```go
// playPlaylistCall records one PlayPlaylist invocation.
type playPlaylistCall struct {
	Name    string
	FromIdx int
}
```

Add the seeders:

```go
// SetPlaylists supplies the playlist list returned by Playlists.
func (c *Client) SetPlaylists(playlists []domain.Playlist) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.playlists = playlists
}

// SetPlaylistTracks supplies the tracks returned by PlaylistTracks(name).
func (c *Client) SetPlaylistTracks(name string, tracks []domain.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.playlistTracks == nil {
		c.playlistTracks = make(map[string][]domain.Track)
	}
	c.playlistTracks[name] = tracks
}

// PlayPlaylistRecord returns a copy of the recorded PlayPlaylist invocations.
func (c *Client) PlayPlaylistRecord() []playPlaylistCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]playPlaylistCall, len(c.playPlaylistRecord))
	copy(out, c.playPlaylistRecord)
	return out
}
```

Replace the `Playlists` stub with:

```go
// Playlists implements music.Client.
func (c *Client) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	out := make([]domain.Playlist, len(c.playlists))
	copy(out, c.playlists)
	return out, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/music/fake/ -run TestPlaylists -v`
Expected: PASS for both new tests.

- [ ] **Step 5: Write the failing test for `PlaylistTracks`**

Append to `client_test.go`:

```go
func TestPlaylistTracksReturnsSeededTracks(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylistTracks("Liked Songs", []domain.Track{
		{Title: "A", Artist: "X", Duration: 90 * time.Second},
		{Title: "B", Artist: "Y", Duration: 120 * time.Second},
	})

	got, err := c.PlaylistTracks(context.Background(), "Liked Songs")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Title != "A" || got[1].Duration != 120*time.Second {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistTracksUnknownNameReturnsErrPlaylistNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	_, err := c.PlaylistTracks(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}
```

If `time` isn't imported, add it.

- [ ] **Step 6: Run to verify failure**

Run: `go test ./internal/music/fake/ -run TestPlaylistTracks -v`
Expected: FAIL (the stub returns `ErrUnavailable`).

- [ ] **Step 7: Replace the `PlaylistTracks` stub**

In `internal/music/fake/client.go`, replace the stub:

```go
// PlaylistTracks implements music.Client.
func (c *Client) PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return nil, c.forcedErr
	}
	if !c.running {
		return nil, music.ErrNotRunning
	}
	tracks, ok := c.playlistTracks[playlistName]
	if !ok {
		return nil, music.ErrPlaylistNotFound
	}
	out := make([]domain.Track, len(tracks))
	copy(out, tracks)
	return out, nil
}
```

- [ ] **Step 8: Run tests**

Run: `go test ./internal/music/fake/ -run TestPlaylistTracks -v`
Expected: PASS.

- [ ] **Step 9: Write the failing test for `PlayPlaylist`**

Append to `client_test.go`:

```go
func TestPlayPlaylistRecordsInvocation(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs", Kind: "user", TrackCount: 3}})

	if err := c.PlayPlaylist(context.Background(), "Liked Songs", 0); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PlayPlaylistCalls != 1 {
		t.Errorf("PlayPlaylistCalls = %d; want 1", c.PlayPlaylistCalls)
	}
	rec := c.PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].Name != "Liked Songs" || rec[0].FromIdx != 0 {
		t.Errorf("record = %+v", rec)
	}
}

func TestPlayPlaylistFromIndexRecorded(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})

	c.PlayPlaylist(context.Background(), "Liked Songs", 4)

	rec := c.PlayPlaylistRecord()
	if rec[0].FromIdx != 4 {
		t.Errorf("FromIdx = %d; want 4", rec[0].FromIdx)
	}
}

func TestPlayPlaylistUnknownNameReturnsErrPlaylistNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})

	err := c.PlayPlaylist(context.Background(), "Atlantis", 0)
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestPlayPlaylistNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New()
	err := c.PlayPlaylist(context.Background(), "Liked Songs", 0)
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

- [ ] **Step 10: Run to verify failure**

Run: `go test ./internal/music/fake/ -run TestPlayPlaylist -v`
Expected: FAIL.

- [ ] **Step 11: Replace the `PlayPlaylist` stub**

In `internal/music/fake/client.go`, replace the stub:

```go
// PlayPlaylist implements music.Client.
func (c *Client) PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	known := false
	for _, p := range c.playlists {
		if p.Name == playlistName {
			known = true
			break
		}
	}
	if !known {
		return music.ErrPlaylistNotFound
	}
	c.PlayPlaylistCalls++
	c.playPlaylistRecord = append(c.playPlaylistRecord, playPlaylistCall{
		Name: playlistName, FromIdx: fromTrackIndex,
	})
	return nil
}
```

- [ ] **Step 12: Run tests**

Run: `go test ./internal/music/fake/ -v`
Expected: PASS for all.

- [ ] **Step 13: Commit**

```bash
git add internal/music/fake/client.go internal/music/fake/client_test.go
git commit -m "music/fake: Playlists/PlaylistTracks/PlayPlaylist + seeders + counters"
```

---

### Task 5: AppleScript `parsePlaylists` + tests

**Why:** Pure parser, easy to test in isolation. Mirrors `parseAirPlayDevices`.

**Files:**
- Modify: `internal/music/applescript/parse.go`
- Modify: `internal/music/applescript/parse_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/music/applescript/parse_test.go`:

```go
func TestParsePlaylistsEmpty(t *testing.T) {
	got, err := parsePlaylists("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParsePlaylistsNotRunning(t *testing.T) {
	_, err := parsePlaylists("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParsePlaylistsSingle(t *testing.T) {
	raw := "Liked Songs\tuser\t42\n"
	got, err := parsePlaylists(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := domain.Playlist{Name: "Liked Songs", Kind: "user", TrackCount: 42}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got = %+v; want [%+v]", got, want)
	}
}

func TestParsePlaylistsMultiple(t *testing.T) {
	raw := "Liked Songs\tuser\t42\n" +
		"Workout\tsubscription\t12\n" +
		"90s Mix\tuser\t30"
	got, err := parsePlaylists(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[1].Kind != "subscription" || got[1].TrackCount != 12 {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestParsePlaylistsSkipsEmptyName(t *testing.T) {
	raw := "\tuser\t0\n" + "Liked Songs\tuser\t42\n"
	got, _ := parsePlaylists(raw)
	if len(got) != 1 || got[0].Name != "Liked Songs" {
		t.Errorf("got = %+v; expected empty-name row to be skipped", got)
	}
}

func TestParsePlaylistsSkipsMalformedRow(t *testing.T) {
	raw := "Bad Row\tuser\n" + "Liked Songs\tuser\t42\n"
	got, _ := parsePlaylists(raw)
	if len(got) != 1 || got[0].Name != "Liked Songs" {
		t.Errorf("got = %+v; expected malformed row to be skipped", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/music/applescript/ -run TestParsePlaylists -v`
Expected: compile error (`undefined: parsePlaylists`).

- [ ] **Step 3: Implement `parsePlaylists`**

Add to `internal/music/applescript/parse.go`:

```go
// parsePlaylists parses the tab-separated output of scriptPlaylists. Each line
// has three fields: name, kind ("user" | "subscription"), track_count.
//
// NOT_RUNNING maps to music.ErrNotRunning. Empty input returns an empty slice
// (legitimate state — Music has no playlists). Rows with empty names are
// skipped (Music permits a "" playlist name but `play playlist ""` errors).
// Rows with the wrong field count are skipped defensively.
func parsePlaylists(raw string) ([]domain.Playlist, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "NOT_RUNNING" {
		return nil, music.ErrNotRunning
	}
	if trimmed == "" {
		return []domain.Playlist{}, nil
	}
	var playlists []domain.Playlist
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		if fields[0] == "" {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil {
			continue
		}
		playlists = append(playlists, domain.Playlist{
			Name:       fields[0],
			Kind:       fields[1],
			TrackCount: count,
		})
	}
	return playlists, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/music/applescript/ -run TestParsePlaylists -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/music/applescript/parse.go internal/music/applescript/parse_test.go
git commit -m "music/applescript: parsePlaylists + tests"
```

---

### Task 6: AppleScript `parsePlaylistTracks` + tests

**Why:** Second parser for the tracks-of-playlist script.

**Files:**
- Modify: `internal/music/applescript/parse.go`
- Modify: `internal/music/applescript/parse_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `parse_test.go`:

```go
func TestParsePlaylistTracksEmpty(t *testing.T) {
	got, err := parsePlaylistTracks("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParsePlaylistTracksNotRunning(t *testing.T) {
	_, err := parsePlaylistTracks("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParsePlaylistTracksNotFound(t *testing.T) {
	_, err := parsePlaylistTracks("NOT_FOUND\n")
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestParsePlaylistTracksSingle(t *testing.T) {
	raw := "Stairway to Heaven\tLed Zeppelin\tLed Zeppelin IV\t482\n"
	got, err := parsePlaylistTracks(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	want := domain.Track{
		Title:    "Stairway to Heaven",
		Artist:   "Led Zeppelin",
		Album:    "Led Zeppelin IV",
		Duration: 482 * time.Second,
	}
	if got[0] != want {
		t.Errorf("got[0] = %+v; want %+v", got[0], want)
	}
}

func TestParsePlaylistTracksMultiple(t *testing.T) {
	raw := "A\tArtist\tAlbum\t100\n" +
		"B\tArtist\tAlbum\t200\n" +
		"C\tArtist\tAlbum\t300"
	got, err := parsePlaylistTracks(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[2].Duration != 300*time.Second {
		t.Errorf("got[2].Duration = %v; want 300s", got[2].Duration)
	}
}

func TestParsePlaylistTracksSkipsMalformedRow(t *testing.T) {
	raw := "BadRow\tArtist\tAlbum\n" + "Good\tArtist\tAlbum\t100\n"
	got, _ := parsePlaylistTracks(raw)
	if len(got) != 1 || got[0].Title != "Good" {
		t.Errorf("got = %+v; expected malformed row to be skipped", got)
	}
}
```

If `time` isn't imported in this test file, add it.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/music/applescript/ -run TestParsePlaylistTracks -v`
Expected: compile error (`undefined: parsePlaylistTracks`).

- [ ] **Step 3: Implement `parsePlaylistTracks`**

Add to `internal/music/applescript/parse.go`:

```go
// parsePlaylistTracks parses the tab-separated output of scriptPlaylistTracks.
// Each line has four fields: title, artist, album, duration_seconds.
//
// NOT_RUNNING → music.ErrNotRunning. NOT_FOUND → music.ErrPlaylistNotFound.
// Empty input returns an empty slice. Malformed rows (wrong field count or
// non-numeric duration) are skipped defensively.
func parsePlaylistTracks(raw string) ([]domain.Track, error) {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "NOT_RUNNING":
		return nil, music.ErrNotRunning
	case "NOT_FOUND":
		return nil, music.ErrPlaylistNotFound
	}
	if trimmed == "" {
		return []domain.Track{}, nil
	}
	var tracks []domain.Track
	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		secs, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err != nil {
			continue
		}
		tracks = append(tracks, domain.Track{
			Title:    fields[0],
			Artist:   fields[1],
			Album:    fields[2],
			Duration: time.Duration(secs * float64(time.Second)),
		})
	}
	return tracks, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/music/applescript/ -run TestParsePlaylistTracks -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/music/applescript/parse.go internal/music/applescript/parse_test.go
git commit -m "music/applescript: parsePlaylistTracks + tests"
```

---

### Task 7: AppleScript `Client.Playlists` + `scriptPlaylists` + tests

**Why:** First end-to-end-tested method on the real client. Mirrors `AirPlayDevices`.

**Files:**
- Modify: `internal/music/applescript/scripts.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/music/applescript/client_test.go`:

```go
func TestPlaylistsRunsScript(t *testing.T) {
	r := &fakeRunner{out: []byte("")}
	c := New(r)
	c.Playlists(context.Background())
	if r.script != scriptPlaylists {
		t.Errorf("ran %q; want scriptPlaylists", r.script)
	}
}

func TestPlaylistsParsesOutput(t *testing.T) {
	r := &fakeRunner{out: []byte("Liked Songs\tuser\t3\nWorkout\tsubscription\t5\n")}
	c := New(r)

	got, err := c.Playlists(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "Liked Songs" || got[1].Kind != "subscription" {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistsNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.Playlists(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/music/applescript/ -run TestPlaylists$ -v` (note the `$`)
Expected: compile error (`scriptPlaylists` undefined) and FAIL once compile passes (the stub returns `ErrUnavailable`).

Actually run with no `$`:

Run: `go test ./internal/music/applescript/ -run "TestPlaylistsRunsScript|TestPlaylistsParsesOutput|TestPlaylistsNotRunning" -v`
Expected: compile error.

- [ ] **Step 3: Add the script constant**

In `internal/music/applescript/scripts.go`, append:

```go
// scriptPlaylists returns one tab-separated line per user/subscription playlist:
//
//	name\tkind\ttrack_count
//
// kind is "user" or "subscription". Iterates user playlists then subscription
// playlists in a single tell block. Empty list ⇒ empty stdout. Returns
// "NOT_RUNNING" if Music isn't running.
//
// NOTE: playlist names containing tabs or linefeeds would corrupt parsing —
// Apple's UI does not permit either, accepted MVP limitation (matches
// scriptAirPlayDevices).
const scriptPlaylists = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set out to ""
	repeat with p in user playlists
		set ln to (name of p) & tab & "user" & tab & ((count of tracks of p) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	repeat with p in subscription playlists
		set ln to (name of p) & tab & "subscription" & tab & ((count of tracks of p) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	return out
end tell`
```

- [ ] **Step 4: Replace the `Playlists` stub with the real implementation**

In `internal/music/applescript/client.go`, replace the stub:

```go
// Playlists implements music.Client.
func (c *Client) Playlists(ctx context.Context) ([]domain.Playlist, error) {
	out, err := c.run(ctx, scriptPlaylists)
	if err != nil {
		return nil, err
	}
	return parsePlaylists(string(out))
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/music/applescript/ -run "TestPlaylistsRunsScript|TestPlaylistsParsesOutput|TestPlaylistsNotRunning" -v`
Expected: PASS.

Run: `go test ./...`
Expected: full suite passes.

- [ ] **Step 6: Commit**

```bash
git add internal/music/applescript/scripts.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: scriptPlaylists + Client.Playlists"
```

---

### Task 8: AppleScript `Client.PlaylistTracks` + `scriptPlaylistTracks` + tests

**Files:**
- Modify: `internal/music/applescript/scripts.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `client_test.go`:

```go
func TestPlaylistTracksRunsScriptWithName(t *testing.T) {
	r := &fakeRunner{out: []byte("")}
	c := New(r)
	c.PlaylistTracks(context.Background(), "Liked Songs")
	if !strings.Contains(r.script, "Liked Songs") {
		t.Errorf("ran %q; expected playlist name in script", r.script)
	}
}

func TestPlaylistTracksParsesOutput(t *testing.T) {
	r := &fakeRunner{out: []byte("A\tArtist\tAlbum\t100\nB\tArtist\tAlbum\t200\n")}
	c := New(r)

	got, err := c.PlaylistTracks(context.Background(), "Liked Songs")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Title != "A" {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistTracksNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_FOUND\n")}
	c := New(r)
	_, err := c.PlaylistTracks(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestPlaylistTracksNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.PlaylistTracks(context.Background(), "Liked Songs")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/music/applescript/ -run "TestPlaylistTracks" -v`
Expected: compile error (`scriptPlaylistTracks` undefined).

- [ ] **Step 3: Add the script constant**

In `scripts.go`, append:

```go
// scriptPlaylistTracks returns one tab-separated line per track of the named
// playlist:
//
//	title\tartist\talbum\tduration_seconds
//
// %s is the EXACT playlist name. Returns "NOT_RUNNING" if Music isn't running,
// "NOT_FOUND" if no playlist with that name exists.
//
// NOTE: track names containing tabs or linefeeds would corrupt parsing —
// accepted MVP limitation.
const scriptPlaylistTracks = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set targetName to "%s"
	set matches to {}
	repeat with p in user playlists
		if (name of p) is equal to targetName then
			set end of matches to p
		end if
	end repeat
	if (count of matches) is 0 then
		repeat with p in subscription playlists
			if (name of p) is equal to targetName then
				set end of matches to p
			end if
		end repeat
	end if
	if (count of matches) is 0 then return "NOT_FOUND"
	set thePlaylist to item 1 of matches
	set out to ""
	repeat with t in tracks of thePlaylist
		set ln to (name of t) & tab & (artist of t) & tab & (album of t) & tab & ((duration of t) as text)
		if out is "" then
			set out to ln
		else
			set out to out & linefeed & ln
		end if
	end repeat
	return out
end tell`
```

- [ ] **Step 4: Replace the `PlaylistTracks` stub**

In `client.go`, replace the stub:

```go
// PlaylistTracks implements music.Client.
func (c *Client) PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error) {
	out, err := c.run(ctx, fmt.Sprintf(scriptPlaylistTracks, playlistName))
	if err != nil {
		return nil, err
	}
	return parsePlaylistTracks(string(out))
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/music/applescript/ -run "TestPlaylistTracks" -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/music/applescript/scripts.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: scriptPlaylistTracks + Client.PlaylistTracks"
```

---

### Task 9: AppleScript `Client.PlayPlaylist` + `scriptPlayPlaylist` + tests

**Files:**
- Modify: `internal/music/applescript/scripts.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `client_test.go`:

```go
func TestPlayPlaylistFromStartUsesPlayPlaylistForm(t *testing.T) {
	r := &fakeRunner{out: []byte("OK\n")}
	c := New(r)

	err := c.PlayPlaylist(context.Background(), "Liked Songs", 0)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(r.script, "play playlist") {
		t.Errorf("script should use 'play playlist' form for fromIdx=0; got %q", r.script)
	}
	if strings.Contains(r.script, "play track") {
		t.Errorf("script should NOT use 'play track' form for fromIdx=0; got %q", r.script)
	}
	if !strings.Contains(r.script, "Liked Songs") {
		t.Errorf("script missing playlist name: %q", r.script)
	}
}

func TestPlayPlaylistFromIndexUsesPlayTrackForm(t *testing.T) {
	r := &fakeRunner{out: []byte("OK\n")}
	c := New(r)

	err := c.PlayPlaylist(context.Background(), "Liked Songs", 4)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// fromIdx=4 (0-based) should become AppleScript "track 5" (1-based).
	if !strings.Contains(r.script, "play track 5") {
		t.Errorf("script should use 'play track 5' for fromIdx=4; got %q", r.script)
	}
}

func TestPlayPlaylistOKReturnsNil(t *testing.T) {
	r := &fakeRunner{out: []byte("OK\n")}
	c := New(r)
	if err := c.PlayPlaylist(context.Background(), "Liked Songs", 0); err != nil {
		t.Errorf("err = %v; want nil", err)
	}
}

func TestPlayPlaylistNotFoundReturnsErrPlaylistNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_FOUND\n")}
	c := New(r)
	err := c.PlayPlaylist(context.Background(), "Atlantis", 0)
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestPlayPlaylistNotRunningReturnsErrNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	err := c.PlayPlaylist(context.Background(), "Liked Songs", 0)
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/music/applescript/ -run "TestPlayPlaylist" -v`
Expected: compile error (`scriptPlayPlaylist` undefined).

- [ ] **Step 3: Add the script constant**

In `scripts.go`, append:

```go
// scriptPlayPlaylistFromStart starts playback of the named playlist from track 1.
// %s is the EXACT playlist name. Uses the literal `play playlist "<name>"`
// form per the spec. Returns "OK" | "NOT_RUNNING" | "NOT_FOUND".
const scriptPlayPlaylistFromStart = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		play playlist "%s"
	on error
		return "NOT_FOUND"
	end try
	return "OK"
end tell`

// scriptPlayPlaylistFromTrack starts playback of the named playlist from a
// specific 1-based track index. %d is the 1-based track number; %s is the
// EXACT playlist name (note: %d comes BEFORE %s in the format string).
// Uses the literal `play track N of playlist "<name>"` form per the spec.
// Returns "OK" | "NOT_RUNNING" | "NOT_FOUND".
const scriptPlayPlaylistFromTrack = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		play track %d of playlist "%s"
	on error
		return "NOT_FOUND"
	end try
	return "OK"
end tell`
```

- [ ] **Step 4: Replace the `PlayPlaylist` stub**

In `client.go`, replace the stub:

```go
// PlayPlaylist implements music.Client. fromTrackIndex is 0-based; 0 means
// "play from the start" and uses the play-playlist form. Any positive value
// is converted to a 1-based AppleScript track number and uses the play-track
// form.
func (c *Client) PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error {
	var script string
	if fromTrackIndex <= 0 {
		script = fmt.Sprintf(scriptPlayPlaylistFromStart, playlistName)
	} else {
		appleIdx := fromTrackIndex + 1
		script = fmt.Sprintf(scriptPlayPlaylistFromTrack, appleIdx, playlistName)
	}
	out, err := c.run(ctx, script)
	if err != nil {
		return err
	}
	switch strings.TrimSpace(string(out)) {
	case "OK":
		return nil
	case "NOT_RUNNING":
		return music.ErrNotRunning
	case "NOT_FOUND":
		return music.ErrPlaylistNotFound
	default:
		return fmt.Errorf("%w: unexpected scriptPlayPlaylist output: %q", music.ErrUnavailable, out)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/music/applescript/ -run "TestPlayPlaylist" -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add internal/music/applescript/scripts.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "music/applescript: scriptPlayPlaylist + Client.PlayPlaylist"
```

---

### Task 10: AppleScript integration test for `Playlists` (read-only)

**Why:** Verifies the AppleScript actually works against a real Music.app. Read-only — does not trigger playback (too disruptive in an integration suite). Mirrors the existing AirPlay integration test.

**Files:**
- Modify: `internal/music/applescript/client_integration_test.go`

- [ ] **Step 1: Read the existing integration test for the pattern**

Run: `cat internal/music/applescript/client_integration_test.go`

(Just orienting — note the `//go:build integration` tag and how Music is launched.)

- [ ] **Step 2: Add the integration test**

Append to `internal/music/applescript/client_integration_test.go`:

```go
func TestIntegrationPlaylistsListsAtLeastZero(t *testing.T) {
	c := NewDefault()
	ctx := context.Background()

	if err := c.Launch(ctx); err != nil {
		t.Fatalf("Launch err = %v", err)
	}

	got, err := c.Playlists(ctx)
	if err != nil {
		t.Fatalf("Playlists err = %v", err)
	}
	// We don't assert a specific count — the user's library is unknown.
	// Assert only that we got a slice (possibly empty) and that any returned
	// rows have plausible Kind values.
	for _, p := range got {
		if p.Kind != "user" && p.Kind != "subscription" {
			t.Errorf("playlist %q has unexpected kind %q", p.Name, p.Kind)
		}
	}
	t.Logf("found %d playlists", len(got))
}
```

If `context` isn't imported in this file, add it.

- [ ] **Step 3: Run the integration test**

Run: `go test -tags=integration ./internal/music/applescript/ -run TestIntegrationPlaylists -v`
Expected: PASS. The `t.Logf` output should show the count of playlists on this machine.

- [ ] **Step 4: Commit**

```bash
git add internal/music/applescript/client_integration_test.go
git commit -m "music/applescript: integration test for Playlists (read-only)"
```

---

### Task 11: CLI `playlists list` (plain + JSON) + tests

**Why:** First user-visible feature. Establishes the `playlists` two-level dispatcher.

**Files:**
- Create: `internal/cli/playlists.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/cli_test.go`:

```go
func TestPlaylistsListPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Liked Songs", Kind: "user", TrackCount: 42},
		{Name: "Workout", Kind: "subscription", TrackCount: 12},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "list"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "Liked Songs") || !strings.Contains(got, "Workout") {
		t.Errorf("stdout missing playlist names: %q", got)
	}
	if !strings.Contains(got, "user") || !strings.Contains(got, "subscription") {
		t.Errorf("stdout missing kind annotations: %q", got)
	}
	if !strings.Contains(got, "42 tracks") || !strings.Contains(got, "12 tracks") {
		t.Errorf("stdout missing track counts: %q", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestPlaylistsListJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Liked Songs", Kind: "user", TrackCount: 42},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "list", "--json"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if len(got) != 1 || got[0]["name"] != "Liked Songs" || got[0]["kind"] != "user" {
		t.Errorf("got = %+v", got)
	}
	if got[0]["track_count"] != float64(42) {
		t.Errorf("track_count = %v; want 42", got[0]["track_count"])
	}
}

func TestPlaylistsListEmptyPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "list"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if !strings.Contains(stdout.String(), "(no playlists)") {
		t.Errorf("stdout missing empty marker: %q", stdout.String())
	}
}

func TestPlaylistsListEmptyJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{})
	var stdout, stderr bytes.Buffer

	Run([]string{"playlists", "list", "--json"}, c, &stdout, &stderr)
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Errorf("stdout = %q; want '[]'", stdout.String())
	}
}

func TestPlaylistsListNotRunningExit1(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "list"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestPlaylistsNoSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a subcommand") {
		t.Errorf("stderr missing 'requires a subcommand': %q", stderr.String())
	}
}

func TestPlaylistsUnknownSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "frobnicate"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Errorf("stderr missing unknown subcommand name: %q", stderr.String())
	}
}

func TestPlaylistsHelpFlag(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		t.Run(arg, func(t *testing.T) {
			c := fake.New()
			var stdout, stderr bytes.Buffer

			code := Run([]string{"playlists", arg}, c, &stdout, &stderr)
			if code != 0 {
				t.Errorf("exit = %d; want 0", code)
			}
			if !strings.Contains(stdout.String(), "playlists") {
				t.Errorf("stdout missing playlists-specific help: %q", stdout.String())
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/cli/ -run TestPlaylists -v`
Expected: FAIL — `Run` returns 1 with "unknown command: playlists".

- [ ] **Step 3: Create `internal/cli/playlists.go`**

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// playlistJSON is the wire format for `goove playlists list --json`.
type playlistJSON struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	TrackCount int    `json:"track_count"`
}

func toPlaylistJSON(p domain.Playlist) playlistJSON {
	return playlistJSON{Name: p.Name, Kind: p.Kind, TrackCount: p.TrackCount}
}

// cmdPlaylists is the two-level dispatcher for `goove playlists <subcommand>`.
// The singular alias `goove playlist` calls into the same dispatcher.
func cmdPlaylists(args []string, client music.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: playlists requires a subcommand: list, tracks, play")
		return 1
	}
	switch args[0] {
	case "list":
		return cmdPlaylistsList(args[1:], client, stdout, stderr)
	case "tracks":
		return cmdPlaylistsTracks(args[1:], client, stdout, stderr)
	case "play":
		return cmdPlaylistsPlay(args[1:], client, stderr)
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, "goove playlists — list and play user / subscription playlists")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Usage:")
		fmt.Fprintln(stdout, "  goove playlists list [--json]                List user + subscription playlists")
		fmt.Fprintln(stdout, "  goove playlists tracks <name> [--json]       List tracks of the matched playlist")
		fmt.Fprintln(stdout, "  goove playlists play <name> [--track N]      Start playback (--track is 1-based)")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  Singular alias: 'goove playlist <subcommand>' works too.")
		return 0
	default:
		fmt.Fprintf(stderr, "goove: unknown playlists subcommand: %s\n", args[0])
		fmt.Fprintln(stderr, "       valid subcommands: list, tracks, play")
		return 1
	}
}

func cmdPlaylistsList(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	playlists, err := client.Playlists(context.Background())
	if err != nil {
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		out := make([]playlistJSON, 0, len(playlists))
		for _, p := range playlists {
			out = append(out, toPlaylistJSON(p))
		}
		if err := json.NewEncoder(stdout).Encode(out); err != nil {
			return 1
		}
		return 0
	}

	if len(playlists) == 0 {
		fmt.Fprintln(stdout, "(no playlists)")
		return 0
	}

	maxName := 0
	for _, p := range playlists {
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
	}
	for _, p := range playlists {
		fmt.Fprintf(stdout, "%-*s  (%s, %d tracks)\n", maxName, p.Name, p.Kind, p.TrackCount)
	}
	return 0
}

// Forward decls — bodies in Tasks 12, 13. Plan keeps tests for those bodies
// in their own tasks; this stub returns the not-yet-implemented sentinel so the
// dispatcher compiles and the list-only tests pass.

func cmdPlaylistsTracks(args []string, client music.Client, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "goove: playlists tracks not yet implemented")
	return 1
}

func cmdPlaylistsPlay(args []string, client music.Client, stderr io.Writer) int {
	fmt.Fprintln(stderr, "goove: playlists play not yet implemented")
	return 1
}
```

- [ ] **Step 4: Wire `playlists` into the top-level dispatcher**

In `internal/cli/cli.go`, in `Run`'s `switch args[0]` block, add:

```go
case "playlists":
	return cmdPlaylists(args[1:], client, stdout, stderr)
```

(Don't add the singular alias yet — Task 14.)

Update `usageText`: add this line in the same alphabetical-ish position the AirPlay line occupies:

```
  goove playlists list|tracks|play [args]   List/inspect/play playlists
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/cli/ -run TestPlaylists -v`
Expected: PASS for the list/help/dispatch tests. The `tracks` and `play` tests don't exist yet.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/playlists.go internal/cli/cli.go internal/cli/cli_test.go
git commit -m "cli: playlists list (plain + JSON) + dispatcher"
```

---

### Task 12: CLI `playlists tracks <name>` (plain + JSON) + tests

**Why:** Inspect a playlist's contents from the shell. Includes the name-resolution helper that `play` will reuse.

**Files:**
- Modify: `internal/cli/playlists.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cli_test.go`:

```go
func TestPlaylistsTracksPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs", Kind: "user", TrackCount: 2}})
	c.SetPlaylistTracks("Liked Songs", []domain.Track{
		{Title: "Stairway", Artist: "Led Zeppelin", Album: "IV", Duration: 482 * time.Second},
		{Title: "Black Dog", Artist: "Led Zeppelin", Album: "IV", Duration: 296 * time.Second},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "Liked Songs"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "1.") || !strings.Contains(got, "2.") {
		t.Errorf("stdout missing 1-based numbering: %q", got)
	}
	if !strings.Contains(got, "Stairway") || !strings.Contains(got, "Led Zeppelin") {
		t.Errorf("stdout missing track fields: %q", got)
	}
	if !strings.Contains(got, "8:02") {
		t.Errorf("stdout missing duration 8:02: %q", got)
	}
}

func TestPlaylistsTracksJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	c.SetPlaylistTracks("Liked Songs", []domain.Track{
		{Title: "A", Artist: "B", Album: "C", Duration: 100 * time.Second},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "Liked Songs", "--json"}, c, &stdout, &stderr)
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
	if got[0]["index"] != float64(1) {
		t.Errorf("index = %v; want 1", got[0]["index"])
	}
	if got[0]["title"] != "A" {
		t.Errorf("title = %v; want A", got[0]["title"])
	}
	if got[0]["duration_sec"] != float64(100) {
		t.Errorf("duration_sec = %v; want 100", got[0]["duration_sec"])
	}
}

func TestPlaylistsTracksMissingNameExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a playlist name") {
		t.Errorf("stderr missing 'requires a playlist name': %q", stderr.String())
	}
}

func TestPlaylistsTracksNotFoundExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "Atlantis"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "playlist not found: Atlantis") {
		t.Errorf("stderr missing 'playlist not found': %q", stderr.String())
	}
}

func TestPlaylistsTracksAmbiguousExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Workout"},
		{Name: "Workout 2025"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "work"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "matches multiple") {
		t.Errorf("stderr missing 'matches multiple': %q", got)
	}
	if !strings.Contains(got, "Workout") || !strings.Contains(got, "Workout 2025") {
		t.Errorf("stderr should list both matches: %q", got)
	}
}

func TestPlaylistsTracksExactMatchPriority(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Workout"},
		{Name: "Workout 2025"},
	})
	c.SetPlaylistTracks("Workout", []domain.Track{{Title: "OnlyTrack"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "Workout"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0 (exact match should win)", code)
	}
	if !strings.Contains(stdout.String(), "OnlyTrack") {
		t.Errorf("stdout did not show tracks for exact-match playlist: %q", stdout.String())
	}
}

func TestPlaylistsTracksSubstringMatch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Liked Songs"},
		{Name: "Workout"},
	})
	c.SetPlaylistTracks("Liked Songs", []domain.Track{{Title: "T"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "tracks", "liked"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0 (substring should match)", code)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/cli/ -run TestPlaylistsTracks -v`
Expected: FAIL — stub returns "not yet implemented".

- [ ] **Step 3: Implement `cmdPlaylistsTracks` + name resolution helper**

Replace the stub `cmdPlaylistsTracks` in `internal/cli/playlists.go` with:

```go
// trackJSON is the wire format for `goove playlists tracks --json` rows.
// Field names match the existing `goove status --json` track shape.
type trackJSON struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	DurationSec int    `json:"duration_sec"`
}

func cmdPlaylistsTracks(args []string, client music.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: playlists tracks requires a playlist name")
		return 1
	}
	jsonOutput := false
	var name string
	for _, a := range args {
		switch a {
		case "--json", "-j":
			jsonOutput = true
		default:
			if name == "" {
				name = a
			}
		}
	}
	if name == "" {
		fmt.Fprintln(stderr, "goove: playlists tracks requires a playlist name")
		return 1
	}

	resolved, code := resolvePlaylistName(client, name, stderr)
	if code != 0 {
		return code
	}

	tracks, err := client.PlaylistTracks(context.Background(), resolved)
	if err != nil {
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		out := make([]trackJSON, 0, len(tracks))
		for i, t := range tracks {
			out = append(out, trackJSON{
				Index:       i + 1,
				Title:       t.Title,
				Artist:      t.Artist,
				Album:       t.Album,
				DurationSec: int(t.Duration.Seconds()),
			})
		}
		if err := json.NewEncoder(stdout).Encode(out); err != nil {
			return 1
		}
		return 0
	}

	if len(tracks) == 0 {
		fmt.Fprintln(stdout, "(no tracks)")
		return 0
	}
	for i, t := range tracks {
		fmt.Fprintf(stdout, "%d. %s — %s  (%s)  [%s]\n",
			i+1, t.Title, t.Artist, t.Album, formatDuration(int(t.Duration.Seconds())))
	}
	return 0
}

// resolvePlaylistName resolves the user's input to an exact playlist name.
// Exact match wins; otherwise case-insensitive substring; multiple substring
// matches → list candidates and exit 1; zero matches → "playlist not found"
// exit 1. Returns (exactName, 0) on success, ("", nonZero) on failure (the
// helper has already written to stderr in that case).
//
// Mirrors the targets-set name-resolution shape but operates on Playlists.
// Factor into a generic helper if a third caller appears.
func resolvePlaylistName(client music.Client, name string, stderr io.Writer) (string, int) {
	playlists, err := client.Playlists(context.Background())
	if err != nil {
		return "", errorExit(err, stderr, true)
	}
	for _, p := range playlists {
		if p.Name == name {
			return p.Name, 0
		}
	}
	lower := strings.ToLower(name)
	var matches []domain.Playlist
	for _, p := range playlists {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		fmt.Fprintf(stderr, "goove: playlist not found: %s\n", name)
		return "", 1
	case 1:
		return matches[0].Name, 0
	default:
		fmt.Fprintf(stderr, "goove: %q matches multiple playlists:\n", name)
		for _, p := range matches {
			fmt.Fprintf(stderr, "  %s\n", p.Name)
		}
		return "", 1
	}
}
```

Add `"strings"` to the imports if not already present.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -run TestPlaylistsTracks -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/playlists.go internal/cli/cli_test.go
git commit -m "cli: playlists tracks (plain + JSON) + name resolution helper"
```

---

### Task 13: CLI `playlists play <name> [--track N]` + tests

**Files:**
- Modify: `internal/cli/playlists.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cli_test.go`:

```go
func TestPlaylistsPlayFromStartSilentExit0(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "Liked Songs"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if c.PlayPlaylistCalls != 1 {
		t.Errorf("PlayPlaylistCalls = %d; want 1", c.PlayPlaylistCalls)
	}
	rec := c.PlayPlaylistRecord()
	if rec[0].Name != "Liked Songs" || rec[0].FromIdx != 0 {
		t.Errorf("record = %+v; want {Liked Songs, 0}", rec[0])
	}
}

func TestPlaylistsPlayWithTrackConvertsTo0Based(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	c.SetPlaylistTracks("Liked Songs", []domain.Track{
		{Title: "T1"}, {Title: "T2"}, {Title: "T3"}, {Title: "T4"}, {Title: "T5"},
	})
	var stdout, stderr bytes.Buffer

	// User passes 1-based --track 3; fake should record FromIdx 2 (0-based).
	code := Run([]string{"playlists", "play", "Liked Songs", "--track", "3"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	rec := c.PlayPlaylistRecord()
	if rec[0].FromIdx != 2 {
		t.Errorf("FromIdx = %d; want 2 (1-based 3 → 0-based 2)", rec[0].FromIdx)
	}
}

func TestPlaylistsPlayMissingNameExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a playlist name") {
		t.Errorf("stderr missing 'requires a playlist name': %q", stderr.String())
	}
}

func TestPlaylistsPlayNotFoundExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "Atlantis"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "playlist not found: Atlantis") {
		t.Errorf("stderr missing 'playlist not found': %q", stderr.String())
	}
}

func TestPlaylistsPlayEmptyPlaylistExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Empty"}})
	c.SetPlaylistTracks("Empty", []domain.Track{})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "Empty"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "has no tracks") {
		t.Errorf("stderr missing 'has no tracks': %q", stderr.String())
	}
	if c.PlayPlaylistCalls != 0 {
		t.Errorf("PlayPlaylistCalls = %d; want 0 (should not invoke play on empty)", c.PlayPlaylistCalls)
	}
}

func TestPlaylistsPlayTrackOutOfRangeExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Short"}})
	c.SetPlaylistTracks("Short", []domain.Track{{Title: "Only"}, {Title: "Two"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "Short", "--track", "5"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "track index out of range: 5") {
		t.Errorf("stderr missing range message: %q", stderr.String())
	}
	if c.PlayPlaylistCalls != 0 {
		t.Errorf("PlayPlaylistCalls = %d; want 0", c.PlayPlaylistCalls)
	}
}

func TestPlaylistsPlayTrackZeroExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "X"}})
	c.SetPlaylistTracks("X", []domain.Track{{Title: "T"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "X", "--track", "0"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1 (--track is 1-based, 0 invalid)", code)
	}
	if !strings.Contains(stderr.String(), "track index out of range: 0") {
		t.Errorf("stderr missing range message: %q", stderr.String())
	}
}

func TestPlaylistsPlayInvalidTrackArgExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "X"}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "X", "--track", "loud"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid --track value") {
		t.Errorf("stderr missing 'invalid --track value': %q", stderr.String())
	}
}

func TestPlaylistsPlayNotRunningExit1(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlists", "play", "X"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/cli/ -run TestPlaylistsPlay -v`
Expected: FAIL — stub returns "not yet implemented".

- [ ] **Step 3: Implement `cmdPlaylistsPlay`**

Replace the stub `cmdPlaylistsPlay` in `internal/cli/playlists.go` with:

```go
func cmdPlaylistsPlay(args []string, client music.Client, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: playlists play requires a playlist name")
		return 1
	}

	// Parse args: first non-flag positional is the name. --track N consumes
	// the next argument. Anything else is unknown (silently ignored, matching
	// the targets/volume style).
	var name string
	trackOneBased := 1 // default = play from track 1
	trackProvided := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--track":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "goove: --track requires a value")
				return 1
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Fprintf(stderr, "goove: invalid --track value: %s\n", args[i+1])
				return 1
			}
			trackOneBased = n
			trackProvided = true
			i++
		default:
			if name == "" {
				name = a
			}
		}
	}
	if name == "" {
		fmt.Fprintln(stderr, "goove: playlists play requires a playlist name")
		return 1
	}

	resolved, code := resolvePlaylistName(client, name, stderr)
	if code != 0 {
		return code
	}

	// Validate against the playlist's tracks. We always need this fetch:
	//   - to detect empty playlists (refuse to call PlayPlaylist)
	//   - to range-check --track when provided
	tracks, err := client.PlaylistTracks(context.Background(), resolved)
	if err != nil {
		return errorExit(err, stderr, true)
	}
	if len(tracks) == 0 {
		fmt.Fprintf(stderr, "goove: playlist has no tracks: %s\n", resolved)
		return 1
	}
	if trackProvided && (trackOneBased < 1 || trackOneBased > len(tracks)) {
		fmt.Fprintf(stderr, "goove: track index out of range: %d (playlist has %d tracks)\n",
			trackOneBased, len(tracks))
		return 1
	}

	fromIdx := 0
	if trackProvided {
		fromIdx = trackOneBased - 1 // 1-based → 0-based
	}

	if err := client.PlayPlaylist(context.Background(), resolved, fromIdx); err != nil {
		return errorExit(err, stderr, true)
	}
	return 0
}
```

Add `"strconv"` to the imports if not already present.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -run TestPlaylistsPlay -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/playlists.go internal/cli/cli_test.go
git commit -m "cli: playlists play with --track + range checks"
```

---

### Task 14: CLI singular `playlist` alias + `usageText` line

**Why:** `goove playlist play "X"` reads more naturally than `goove playlists play "X"`. One-line alias.

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cli_test.go`:

```go
func TestPlaylistSingularAliasRoutes(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs", Kind: "user", TrackCount: 1}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"playlist", "list"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if !strings.Contains(stdout.String(), "Liked Songs") {
		t.Errorf("singular alias did not route to playlists list: %q", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/cli/ -run TestPlaylistSingularAlias -v`
Expected: FAIL — `Run` returns 1 with "unknown command: playlist".

- [ ] **Step 3: Add the alias case**

In `internal/cli/cli.go`, in the `Run` switch, add a case adjacent to `playlists`:

```go
case "playlists", "playlist":
	return cmdPlaylists(args[1:], client, stdout, stderr)
```

(Replace the existing `case "playlists":` line with the combined form above.)

- [ ] **Step 4: Run test**

Run: `go test ./internal/cli/ -run TestPlaylistSingularAlias -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "cli: singular 'playlist' alias for the playlists dispatcher"
```

---

### Task 15: TUI — add `viewMode`, `Model.mode`, `Model.browser`, message types

**Why:** Foundational scaffolding for the browser. No behaviour change yet.

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/messages.go`

- [ ] **Step 1: Read existing `messages.go` for the message style**

Run: `cat internal/app/messages.go`

(Just orienting.)

- [ ] **Step 2: Add `viewMode`, `browserPane`, `browserState`, and Model fields**

In `internal/app/model.go`:

Above `type Model struct`, add:

```go
type viewMode int

const (
	modeNowPlaying viewMode = iota
	modeBrowser
)

type browserPane int

const (
	leftPane  browserPane = iota // playlists
	rightPane                    // tracks of selected playlist
)

// browserState is the modal browser-view state. nil on Model means "browser
// not open"; non-nil means "browser is showing." Loading flags suppress
// duplicate fetches while a Cmd is in flight.
type browserState struct {
	pane           browserPane
	playlists      []domain.Playlist
	playlistCursor int
	loadingLists   bool
	tracks         []domain.Track // tracks of the playlist at playlistCursor
	tracksFor      string         // name of the playlist tracks were last fetched for
	trackCursor    int
	loadingTracks  bool
	err            error
}
```

In the `Model` struct, add two fields (anywhere is fine; placing alongside `picker` is logical):

```go
mode    viewMode
browser *browserState
```

- [ ] **Step 3: Add the three new message types**

Append to `internal/app/messages.go`:

```go
// playlistsMsg carries the result of a Playlists fetch.
type playlistsMsg struct {
	playlists []domain.Playlist
	err       error
}

// playlistTracksMsg carries the result of a PlaylistTracks fetch.
// name is the playlist the tracks belong to, used to ignore stale results
// (the user may have moved the cursor and triggered another fetch before
// this one completed).
type playlistTracksMsg struct {
	name   string
	tracks []domain.Track
	err    error
}

// playPlaylistMsg carries the result of a PlayPlaylist invocation. The
// existing 1Hz status tick will surface the new now-playing in its next poll;
// this message is just for surfacing errors in the browser.
type playPlaylistMsg struct {
	err error
}
```

If `domain` isn't imported in `messages.go`, add it.

- [ ] **Step 4: Verify compile**

Run: `go build ./...`
Expected: clean.

Run: `go test ./...`
Expected: existing tests still pass (we haven't changed any behaviour yet).

- [ ] **Step 5: Commit**

```bash
git add internal/app/model.go internal/app/messages.go
git commit -m "app: viewMode + browserState + playlist message types"
```

---

### Task 16: TUI — fetch commands

**Why:** Three Bubble Tea `tea.Cmd` factories that wrap the `music.Client` calls. Mirrors `fetchStatus`.

**Files:**
- Create: `internal/app/browser.go`

- [ ] **Step 1: Read an existing fetch command for the pattern**

Run: `grep -n "func fetchStatus" internal/app/*.go`

Then read whichever file contains it (likely `tick.go` or `update.go`).

- [ ] **Step 2: Create `internal/app/browser.go` with the fetch commands**

```go
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/music"
)

// fetchPlaylists returns a Cmd that calls client.Playlists and produces
// a playlistsMsg.
func fetchPlaylists(c music.Client) tea.Cmd {
	return func() tea.Msg {
		playlists, err := c.Playlists(context.Background())
		return playlistsMsg{playlists: playlists, err: err}
	}
}

// fetchPlaylistTracks returns a Cmd that calls client.PlaylistTracks and
// produces a playlistTracksMsg. The name is echoed in the message so the
// update handler can ignore stale results.
func fetchPlaylistTracks(c music.Client, name string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := c.PlaylistTracks(context.Background(), name)
		return playlistTracksMsg{name: name, tracks: tracks, err: err}
	}
}

// playPlaylist returns a Cmd that calls client.PlayPlaylist and produces
// a playPlaylistMsg.
func playPlaylist(c music.Client, name string, fromIdx int) tea.Cmd {
	return func() tea.Msg {
		err := c.PlayPlaylist(context.Background(), name, fromIdx)
		return playPlaylistMsg{err: err}
	}
}
```

- [ ] **Step 3: Verify compile**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/app/browser.go
git commit -m "app: fetchPlaylists / fetchPlaylistTracks / playPlaylist commands"
```

---

### Task 17: TUI — `l` opens the browser; handle `playlistsMsg`

**Why:** First user-visible TUI behaviour for browse. Switches mode + dispatches the fetch + populates state when the message arrives.

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/browser.go` (or a new helper there)
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Read existing key handling to understand the dispatch shape**

Run: `cat internal/app/update.go`

Look at where `KeyMsg` is handled — note the existing per-key routing.

- [ ] **Step 2: Write the failing tests**

Append to `internal/app/update_test.go`:

```go
func TestKeyLOpensBrowserAndDispatchesFetch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	mm := updated.(Model)
	if mm.mode != modeBrowser {
		t.Errorf("mode = %v; want modeBrowser", mm.mode)
	}
	if mm.browser == nil {
		t.Fatal("browser state nil; want non-nil after pressing 'l'")
	}
	if !mm.browser.loadingLists {
		t.Errorf("browser.loadingLists = false; want true")
	}
	if cmd == nil {
		t.Fatal("expected a fetchPlaylists Cmd; got nil")
	}
}

func TestPlaylistsMsgPopulatesState(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{loadingLists: true}

	updated, _ := m.Update(playlistsMsg{
		playlists: []domain.Playlist{{Name: "Liked Songs", Kind: "user", TrackCount: 5}},
	})

	mm := updated.(Model)
	if mm.browser.loadingLists {
		t.Errorf("loadingLists still true after message")
	}
	if len(mm.browser.playlists) != 1 || mm.browser.playlists[0].Name != "Liked Songs" {
		t.Errorf("playlists = %+v", mm.browser.playlists)
	}
	if mm.browser.err != nil {
		t.Errorf("err = %v; want nil", mm.browser.err)
	}
}

func TestPlaylistsMsgErrorStoredInState(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{loadingLists: true}

	updated, _ := m.Update(playlistsMsg{err: music.ErrNotRunning})

	mm := updated.(Model)
	if mm.browser.err == nil {
		t.Errorf("err = nil; want non-nil")
	}
	if mm.browser.loadingLists {
		t.Errorf("loadingLists still true after error")
	}
}
```

If imports are missing: `tea "github.com/charmbracelet/bubbletea"`, `domain`, `music`, `fake` from existing imports in this file.

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/app/ -run "TestKeyLOpens|TestPlaylistsMsg" -v`
Expected: FAIL.

- [ ] **Step 4: Add the key + message handlers**

In `internal/app/update.go`, find the `tea.KeyMsg` block and add (in the appropriate switch on `msg.String()` or rune handling — match the existing style):

For a rune-style key handler, add a case for `'l'`:

```go
case "l":
	if m.mode == modeBrowser {
		// No-op when already in browser (spec: 'l' in browser is a no-op).
		return m, nil
	}
	m.mode = modeBrowser
	m.browser = &browserState{loadingLists: true}
	return m, fetchPlaylists(m.client)
```

Place this alongside the existing single-key cases (e.g. near where `space`/`n`/`p` are handled). If the existing code uses a different key-routing shape (e.g. a `switch msg.Type` with a rune extraction), adapt to that style — the goal is "pressing `l` switches mode and dispatches the fetch."

Also handle the new message types by adding to the top-level `Update` switch (alongside `statusMsg`, `pickerMsg`, etc.):

```go
case playlistsMsg:
	if m.browser != nil {
		m.browser.loadingLists = false
		m.browser.err = msg.err
		if msg.err == nil {
			m.browser.playlists = msg.playlists
			if m.browser.playlistCursor >= len(msg.playlists) {
				m.browser.playlistCursor = 0
			}
		}
	}
	return m, nil
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/app/ -run "TestKeyLOpens|TestPlaylistsMsg" -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add internal/app/update.go internal/app/update_test.go
git commit -m "app: 'l' opens browser + playlistsMsg handler"
```

---

### Task 18: TUI — left pane navigation (j/k/up/down)

**Why:** Move the cursor in the playlists list. Must NOT trigger track fetching (lazy-fetch policy from spec).

**Files:**
- Modify: `internal/app/update.go` (or browser-specific dispatch added there)
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `update_test.go`:

```go
func TestBrowserLeftPaneDownMovesCursor(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "A"}, {Name: "B"}, {Name: "C"}},
	}

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
	} {
		t.Run(key.String(), func(t *testing.T) {
			startCursor := m.browser.playlistCursor
			updated, _ := m.Update(key)
			mm := updated.(Model)
			if mm.browser.playlistCursor != startCursor+1 {
				t.Errorf("cursor = %d; want %d", mm.browser.playlistCursor, startCursor+1)
			}
			m = mm // carry state forward to test the second key
		})
	}
}

func TestBrowserLeftPaneUpMovesCursor(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:           leftPane,
		playlists:      []domain.Playlist{{Name: "A"}, {Name: "B"}, {Name: "C"}},
		playlistCursor: 2,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm := updated.(Model)
	if mm.browser.playlistCursor != 1 {
		t.Errorf("cursor = %d; want 1", mm.browser.playlistCursor)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm = updated.(Model)
	if mm.browser.playlistCursor != 0 {
		t.Errorf("cursor = %d; want 0", mm.browser.playlistCursor)
	}
}

func TestBrowserLeftPaneCursorClampsAtBounds(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "A"}, {Name: "B"}},
	}

	// Up at top stays at 0.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(Model).browser.playlistCursor != 0 {
		t.Errorf("up at 0 should clamp")
	}

	// Down past last stays at last.
	m.browser.playlistCursor = 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(Model).browser.playlistCursor != 1 {
		t.Errorf("down at last should clamp")
	}
}

func TestBrowserLeftPaneNavigationDoesNotFetchTracks(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "A"}, {Name: "B"}})
	c.SetPlaylistTracks("A", []domain.Track{{Title: "TA"}})
	c.SetPlaylistTracks("B", []domain.Track{{Title: "TB"}})
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "A"}, {Name: "B"}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Errorf("cursor move on left pane should not return a Cmd; got %T", cmd())
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app/ -run TestBrowserLeftPane -v`
Expected: FAIL.

- [ ] **Step 3: Implement browser key handling**

In `internal/app/update.go`, where the `KeyMsg` block lives, add a guard at the top that diverts browser-mode keys into a helper. Either:

(a) Add an early `if m.mode == modeBrowser { ... }` block, OR
(b) Add a per-key conditional inside each case.

Recommend (a) for clarity. Add (before the existing key cases):

```go
if m.mode == modeBrowser {
	return handleBrowserKey(m, msg)
}
```

Then add `handleBrowserKey` to `internal/app/browser.go`:

```go
// handleBrowserKey routes key messages while the browser is open. Returns the
// updated model + any Cmd. Transport keys (space, n, p, +, -, q) fall through
// to the now-playing key handler (Task 23). Browser-specific keys are handled
// here.
func handleBrowserKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.browser == nil {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		return browserCursorUp(m), nil
	case "down", "j":
		return browserCursorDown(m), nil
	}
	return m, nil
}

// browserCursorUp moves the cursor up by 1, clamped to 0, in the focused pane.
func browserCursorUp(m Model) Model {
	if m.browser.pane == leftPane {
		if m.browser.playlistCursor > 0 {
			m.browser.playlistCursor--
		}
	} else {
		if m.browser.trackCursor > 0 {
			m.browser.trackCursor--
		}
	}
	return m
}

// browserCursorDown moves the cursor down by 1, clamped to the last item, in
// the focused pane.
func browserCursorDown(m Model) Model {
	if m.browser.pane == leftPane {
		if m.browser.playlistCursor < len(m.browser.playlists)-1 {
			m.browser.playlistCursor++
		}
	} else {
		if m.browser.trackCursor < len(m.browser.tracks)-1 {
			m.browser.trackCursor++
		}
	}
	return m
}
```

If `tea` isn't imported in `browser.go`, it already is from Task 16.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestBrowserLeftPane -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean (the existing now-playing tests should still pass — they run with the default `mode = modeNowPlaying`, so the browser-key diversion doesn't intercept them).

- [ ] **Step 5: Commit**

```bash
git add internal/app/update.go internal/app/browser.go internal/app/update_test.go
git commit -m "app: browser left pane navigation (j/k/up/down)"
```

---

### Task 19: TUI — `tab` / `→` switches pane and fetches tracks lazily

**Why:** Implements the lazy-fetch policy: tracks fetched only on right-pane focus, not on every left-pane cursor move.

**Files:**
- Modify: `internal/app/browser.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `update_test.go`:

```go
func TestBrowserTabSwitchesToRightPaneAndFetchesTracks(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:           leftPane,
		playlists:      []domain.Playlist{{Name: "Liked Songs"}},
		playlistCursor: 0,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)

	if mm.browser.pane != rightPane {
		t.Errorf("pane = %v; want rightPane", mm.browser.pane)
	}
	if !mm.browser.loadingTracks {
		t.Errorf("loadingTracks = false; want true after focusing right pane")
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylistTracks Cmd; got nil")
	}
	// Verify the cmd produces a playlistTracksMsg for the right playlist.
	msg := cmd()
	pmsg, ok := msg.(playlistTracksMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want playlistTracksMsg", msg)
	}
	if pmsg.name != "Liked Songs" {
		t.Errorf("fetched name = %q; want Liked Songs", pmsg.name)
	}
}

func TestBrowserRightArrowAlsoSwitchesPane(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "X"}},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if updated.(Model).browser.pane != rightPane {
		t.Errorf("right arrow did not switch pane")
	}
}

func TestBrowserShiftTabReturnsToLeftPaneNoFetch(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:   rightPane,
		tracks: []domain.Track{{Title: "T"}},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm := updated.(Model)
	if mm.browser.pane != leftPane {
		t.Errorf("pane = %v; want leftPane", mm.browser.pane)
	}
	if cmd != nil {
		t.Errorf("returning to left should not trigger a Cmd")
	}
}

func TestPlaylistTracksMsgPopulatesState(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      rightPane,
		playlists: []domain.Playlist{{Name: "Liked Songs"}},
		loadingTracks: true,
	}

	updated, _ := m.Update(playlistTracksMsg{
		name:   "Liked Songs",
		tracks: []domain.Track{{Title: "A"}, {Title: "B"}},
	})
	mm := updated.(Model)
	if mm.browser.loadingTracks {
		t.Errorf("loadingTracks still true after message")
	}
	if len(mm.browser.tracks) != 2 {
		t.Errorf("tracks = %+v", mm.browser.tracks)
	}
	if mm.browser.tracksFor != "Liked Songs" {
		t.Errorf("tracksFor = %q; want Liked Songs", mm.browser.tracksFor)
	}
}

func TestPlaylistTracksMsgIgnoresStaleResult(t *testing.T) {
	// Cursor moved to playlist B, then B's fetch was issued, but A's
	// older fetch arrives first. We should ignore A's tracks.
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:           rightPane,
		playlists:      []domain.Playlist{{Name: "A"}, {Name: "B"}},
		playlistCursor: 1, // B is currently selected
		loadingTracks:  true,
	}

	updated, _ := m.Update(playlistTracksMsg{
		name:   "A", // stale — cursor is on B now
		tracks: []domain.Track{{Title: "From A"}},
	})
	mm := updated.(Model)
	if len(mm.browser.tracks) != 0 {
		t.Errorf("stale result should not have populated tracks: %+v", mm.browser.tracks)
	}
	if !mm.browser.loadingTracks {
		t.Errorf("loadingTracks should remain true (the right fetch hasn't returned)")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app/ -run "TestBrowser(Tab|Right|Shift)|TestPlaylistTracksMsg" -v`
Expected: FAIL.

- [ ] **Step 3: Add the pane-switch and message handlers**

In `internal/app/browser.go`, extend `handleBrowserKey`:

```go
func handleBrowserKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.browser == nil {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		return browserCursorUp(m), nil
	case "down", "j":
		return browserCursorDown(m), nil
	case "tab", "right":
		return browserFocusRight(m)
	case "shift+tab", "left":
		m.browser.pane = leftPane
		return m, nil
	}
	return m, nil
}

// browserFocusRight switches focus to the right (tracks) pane. If the tracks
// for the currently-selected playlist haven't been fetched yet (or were
// fetched for a different playlist), it dispatches a fetchPlaylistTracks Cmd
// and sets loadingTracks. Otherwise it's a pure focus change.
func browserFocusRight(m Model) (Model, tea.Cmd) {
	m.browser.pane = rightPane
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	if m.browser.tracksFor == current {
		return m, nil // already have these tracks
	}
	m.browser.loadingTracks = true
	m.browser.tracks = nil
	m.browser.trackCursor = 0
	return m, fetchPlaylistTracks(m.client, current)
}
```

In the top-level `Update` (in `update.go`), add a handler for `playlistTracksMsg`:

```go
case playlistTracksMsg:
	if m.browser != nil && len(m.browser.playlists) > 0 {
		current := m.browser.playlists[m.browser.playlistCursor].Name
		if msg.name != current {
			// Stale result — the cursor has moved since this fetch was issued.
			return m, nil
		}
		m.browser.loadingTracks = false
		m.browser.err = msg.err
		if msg.err == nil {
			m.browser.tracks = msg.tracks
			m.browser.tracksFor = msg.name
			m.browser.trackCursor = 0
		}
	}
	return m, nil
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run "TestBrowser|TestPlaylistTracksMsg" -v`
Expected: PASS for the new tests + the prior browser tests still pass.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/app/browser.go internal/app/update.go internal/app/update_test.go
git commit -m "app: tab/arrow pane switching + lazy track fetch"
```

---

### Task 20: TUI — `enter` plays (left = whole playlist; right = from track)

**Files:**
- Modify: `internal/app/browser.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `update_test.go`:

```go
func TestBrowserEnterOnLeftPlaysWholePlaylist(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "Liked Songs"}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected playPlaylist Cmd; got nil")
	}
	msg := cmd()
	if _, ok := msg.(playPlaylistMsg); !ok {
		t.Fatalf("cmd produced %T; want playPlaylistMsg", msg)
	}
	rec := c.PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].Name != "Liked Songs" || rec[0].FromIdx != 0 {
		t.Errorf("record = %+v; want one call with Name=Liked Songs FromIdx=0", rec)
	}
}

func TestBrowserEnterOnRightPlaysFromTrack(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:           rightPane,
		playlists:      []domain.Playlist{{Name: "Liked Songs"}},
		tracks:         []domain.Track{{Title: "T1"}, {Title: "T2"}, {Title: "T3"}},
		tracksFor:      "Liked Songs",
		trackCursor:    2,
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected playPlaylist Cmd; got nil")
	}
	cmd()
	rec := c.PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].FromIdx != 2 {
		t.Errorf("record = %+v; want one call with FromIdx=2", rec)
	}
}

func TestBrowserEnterOnRightWithEmptyTracksIsNoOp(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Empty"}})
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      rightPane,
		playlists: []domain.Playlist{{Name: "Empty"}},
		tracks:    []domain.Track{},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("enter on empty tracks should be a no-op; got Cmd")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app/ -run TestBrowserEnter -v`
Expected: FAIL.

- [ ] **Step 3: Implement Enter handling**

In `internal/app/browser.go`, extend `handleBrowserKey`:

```go
case "enter":
	return handleBrowserEnter(m)
```

Add the helper:

```go
// handleBrowserEnter starts playback. From the left pane, it plays the
// highlighted playlist from track 1. From the right pane, it plays the
// playlist starting at the highlighted track. Empty playlists are a no-op.
func handleBrowserEnter(m Model) (Model, tea.Cmd) {
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	if m.browser.pane == leftPane {
		return m, playPlaylist(m.client, current, 0)
	}
	if len(m.browser.tracks) == 0 {
		return m, nil
	}
	return m, playPlaylist(m.client, current, m.browser.trackCursor)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestBrowserEnter -v`
Expected: PASS.

Run: `go test ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/app/browser.go internal/app/update_test.go
git commit -m "app: enter plays (left=whole playlist; right=from highlighted track)"
```

---

### Task 21: TUI — `r` refetches the focused pane's data

**Files:**
- Modify: `internal/app/browser.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append:

```go
func TestBrowserRRefetchesPlaylistsOnLeftPane(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      leftPane,
		playlists: []domain.Playlist{{Name: "A"}},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	mm := updated.(Model)
	if !mm.browser.loadingLists {
		t.Errorf("loadingLists = false; want true")
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylists Cmd; got nil")
	}
	if _, ok := cmd().(playlistsMsg); !ok {
		t.Errorf("Cmd did not produce playlistsMsg")
	}
}

func TestBrowserRRefetchesTracksOnRightPane(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{
		pane:      rightPane,
		playlists: []domain.Playlist{{Name: "Liked Songs"}},
		tracksFor: "Liked Songs",
		tracks:    []domain.Track{{Title: "T"}},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	mm := updated.(Model)
	if !mm.browser.loadingTracks {
		t.Errorf("loadingTracks = false; want true")
	}
	if cmd == nil {
		t.Fatal("expected fetchPlaylistTracks Cmd; got nil")
	}
	msg := cmd()
	pmsg, ok := msg.(playlistTracksMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want playlistTracksMsg", msg)
	}
	if pmsg.name != "Liked Songs" {
		t.Errorf("refetch name = %q; want Liked Songs", pmsg.name)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app/ -run TestBrowserR -v`
Expected: FAIL.

- [ ] **Step 3: Implement `r`**

In `internal/app/browser.go`, extend `handleBrowserKey`:

```go
case "r":
	return handleBrowserRefetch(m)
```

Add the helper:

```go
// handleBrowserRefetch refetches the focused pane's data: playlists for the
// left pane, tracks for the right pane. Resets the relevant tracksFor sentinel
// so the result is not treated as stale.
func handleBrowserRefetch(m Model) (Model, tea.Cmd) {
	if m.browser.pane == leftPane {
		m.browser.loadingLists = true
		return m, fetchPlaylists(m.client)
	}
	if len(m.browser.playlists) == 0 {
		return m, nil
	}
	current := m.browser.playlists[m.browser.playlistCursor].Name
	m.browser.loadingTracks = true
	m.browser.tracksFor = "" // force the playlistTracksMsg handler to accept the result
	return m, fetchPlaylistTracks(m.client, current)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestBrowserR -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/browser.go internal/app/update_test.go
git commit -m "app: 'r' refetches focused pane's data"
```

---

### Task 22: TUI — `esc` returns to now-playing

**Files:**
- Modify: `internal/app/browser.go`
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestBrowserEscReturnsToNowPlaying(t *testing.T) {
	c := fake.New()
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{playlists: []domain.Playlist{{Name: "X"}}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(Model)
	if mm.mode != modeNowPlaying {
		t.Errorf("mode = %v; want modeNowPlaying", mm.mode)
	}
	if mm.browser != nil {
		t.Errorf("browser state should be cleared on esc; got %+v", mm.browser)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/app/ -run TestBrowserEsc -v`
Expected: FAIL.

- [ ] **Step 3: Implement `esc`**

In `internal/app/browser.go`, extend `handleBrowserKey`:

```go
case "esc":
	m.mode = modeNowPlaying
	m.browser = nil
	return m, nil
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/app/ -run TestBrowserEsc -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/browser.go internal/app/update_test.go
git commit -m "app: esc returns from browser to now-playing"
```

---

### Task 23: TUI — transport keys remain live in browser mode

**Why:** Spec says `space`, `n`, `p`, `+`, `-`, `q` should work in browser mode. The current `handleBrowserKey` returns `(m, nil)` for unhandled keys, which would silently swallow them. We need to fall through to the now-playing handler instead.

**Files:**
- Modify: `internal/app/update.go` (rework the diversion)
- Modify: `internal/app/browser.go` (return a "not handled" signal)
- Modify: `internal/app/update_test.go`

- [ ] **Step 1: Write the failing tests**

Append:

```go
func TestBrowserModeTransportKeysStillFire(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	m := New(c, nil)
	m.mode = modeBrowser
	m.browser = &browserState{playlists: []domain.Playlist{{Name: "X"}}}

	tests := []struct {
		name string
		key  tea.KeyMsg
		want func(*fake.Client) bool
	}{
		{"space → playpause", tea.KeyMsg{Type: tea.KeySpace}, func(c *fake.Client) bool { return c.PlayPauseCalls == 1 }},
		{"n → next", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, func(c *fake.Client) bool { return c.NextCalls == 1 }},
		{"p → prev", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, func(c *fake.Client) bool { return c.PrevCalls == 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.New()
			c.Launch(context.Background())
			m := New(c, nil)
			m.mode = modeBrowser
			m.browser = &browserState{playlists: []domain.Playlist{{Name: "X"}}}

			updated, cmd := m.Update(tt.key)
			if cmd != nil {
				cmd() // execute the Cmd so the fake's counter is incremented
			}
			_ = updated
			if !tt.want(c) {
				t.Errorf("transport call did not fire for %s", tt.name)
			}
		})
	}
}
```

(Volume +/- is left out of the test — it's handled the same way and adding it doesn't add coverage.)

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/app/ -run TestBrowserModeTransport -v`
Expected: FAIL — `handleBrowserKey` swallows `space`/`n`/`p` and returns `(m, nil)`.

- [ ] **Step 3: Make `handleBrowserKey` signal "not handled"**

Change the signature of `handleBrowserKey` in `browser.go` to return a third value:

```go
func handleBrowserKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.browser == nil {
		return m, nil, false
	}
	switch msg.String() {
	case "up", "k":
		mm := browserCursorUp(m)
		return mm, nil, true
	case "down", "j":
		mm := browserCursorDown(m)
		return mm, nil, true
	case "tab", "right":
		mm, cmd := browserFocusRight(m)
		return mm, cmd, true
	case "shift+tab", "left":
		m.browser.pane = leftPane
		return m, nil, true
	case "enter":
		mm, cmd := handleBrowserEnter(m)
		return mm, cmd, true
	case "r":
		mm, cmd := handleBrowserRefetch(m)
		return mm, cmd, true
	case "esc":
		m.mode = modeNowPlaying
		m.browser = nil
		return m, nil, true
	case "l":
		// Already in browser; spec says no-op (don't toggle).
		return m, nil, true
	}
	// Not a browser-specific key — let the now-playing handler take it.
	return m, nil, false
}
```

In `update.go`, change the diversion:

```go
if m.mode == modeBrowser {
	if mm, cmd, handled := handleBrowserKey(m, msg); handled {
		return mm, cmd
	}
	// Fall through to the now-playing key handler for transport keys etc.
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestBrowserModeTransport -v`
Expected: PASS.

Run: `go test ./internal/app/ -v`
Expected: ALL existing browser tests still pass (the helper signature changed but the routing is the same for handled keys).

- [ ] **Step 5: Commit**

```bash
git add internal/app/browser.go internal/app/update.go internal/app/update_test.go
git commit -m "app: fall through to transport keys when browser doesn't handle key"
```

---

### Task 24: TUI — browser view rendering

**Why:** Render the two-column browser. View is hand-tested (the existing `view.go` has no unit tests).

**Files:**
- Modify: `internal/app/view.go`
- Modify: `internal/app/browser.go`

- [ ] **Step 1: Read existing `view.go` to understand the rendering style**

Run: `cat internal/app/view.go`

(Note the helper functions, lipgloss usage if any, layout idioms.)

- [ ] **Step 2: Add `renderBrowser` to `browser.go`**

```go
import (
	"fmt"
	"strings"
	// keep tea + context imports
)

// renderBrowser returns the full-screen string for modeBrowser. Layout is two
// columns separated by a vertical bar. Long lists scroll via window-clamping
// around the cursor. Width comes from the Model.
func renderBrowser(m Model) string {
	if m.browser == nil {
		return "" // shouldn't happen, but guard anyway
	}
	// Reserve some terminal-edge padding; everything else is split.
	totalWidth := m.width
	if totalWidth < 40 {
		totalWidth = 80 // safe default
	}
	leftWidth := totalWidth/2 - 1
	rightWidth := totalWidth - leftWidth - 3 // 3 for " │ "
	height := m.height - 4
	if height < 5 {
		height = 20
	}

	leftLines := renderLeftPane(m.browser, leftWidth, height)
	rightLines := renderRightPane(m.browser, rightWidth, height)

	var out strings.Builder
	out.WriteString("┌─ goove · browser ")
	out.WriteString(strings.Repeat("─", maxInt(0, totalWidth-20)))
	out.WriteString("┐\n")
	for i := 0; i < height; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		fmt.Fprintf(&out, "│ %-*s │ %-*s │\n", leftWidth, left, rightWidth, right)
	}
	out.WriteString("└")
	out.WriteString(strings.Repeat("─", totalWidth-2))
	out.WriteString("┘\n")
	out.WriteString(" ↑↓: nav   tab: pane   ⏎: play   r: refetch   esc: back   space: ⏯\n")
	return out.String()
}

func renderLeftPane(b *browserState, width, height int) []string {
	header := "Playlists"
	if b.pane == leftPane {
		header = "▸ Playlists"
	}
	out := []string{header, ""}
	if b.loadingLists {
		out = append(out, "Loading…")
		return out
	}
	if b.err != nil && b.pane == leftPane {
		out = append(out, "error: "+b.err.Error())
		return out
	}
	if len(b.playlists) == 0 {
		out = append(out, "(no playlists)")
		return out
	}
	visibleRows := height - 2
	start := scrollWindow(b.playlistCursor, visibleRows, len(b.playlists))
	for i := start; i < len(b.playlists) && i-start < visibleRows; i++ {
		marker := "  "
		if i == b.playlistCursor && b.pane == leftPane {
			marker = "▸ "
		}
		row := fmt.Sprintf("%s%s", marker, b.playlists[i].Name)
		if b.playlists[i].Kind == "subscription" {
			row += " (sub)"
		}
		out = append(out, truncate(row, width))
	}
	return out
}

func renderRightPane(b *browserState, width, height int) []string {
	title := "Tracks"
	if len(b.playlists) > 0 {
		title = "Tracks — " + b.playlists[b.playlistCursor].Name
	}
	if b.pane == rightPane {
		title = "▸ " + title
	}
	out := []string{title, ""}
	if b.pane == rightPane && b.loadingTracks {
		out = append(out, "Loading…")
		return out
	}
	if b.pane == rightPane && b.err != nil {
		out = append(out, "error: "+b.err.Error())
		return out
	}
	current := ""
	if len(b.playlists) > 0 {
		current = b.playlists[b.playlistCursor].Name
	}
	if b.tracksFor != current {
		out = append(out, "(press tab to load)")
		return out
	}
	if len(b.tracks) == 0 {
		out = append(out, "(no tracks)")
		return out
	}
	visibleRows := height - 2
	start := scrollWindow(b.trackCursor, visibleRows, len(b.tracks))
	for i := start; i < len(b.tracks) && i-start < visibleRows; i++ {
		marker := "  "
		if i == b.trackCursor && b.pane == rightPane {
			marker = "▸ "
		}
		t := b.tracks[i]
		row := fmt.Sprintf("%s%d. %s — %s", marker, i+1, t.Title, t.Artist)
		out = append(out, truncate(row, width))
	}
	return out
}

// scrollWindow returns the top-of-window index such that cursor is visible
// within a viewport of size visible across total items. Cursor stays roughly
// centred when possible.
func scrollWindow(cursor, visible, total int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := cursor - half
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 3: Wire it into `View()`**

In `internal/app/view.go`, find the `View()` method and add a guard at the top:

```go
if m.mode == modeBrowser {
	return renderBrowser(m)
}
```

(Place this before any other view-rendering logic — the picker overlay still renders over the now-playing card; the browser replaces the now-playing card and does NOT render the picker on top of itself in v1.)

- [ ] **Step 4: Verify compile + manual smoke**

Run: `go build ./...`
Expected: clean.

Run: `go test ./...`
Expected: clean.

**Manual smoke** (per spec — view is not unit-tested):

```bash
go run ./cmd/goove
```

In the running TUI:
1. Press `l` — should switch to a two-column browser; left column shows your playlists, right shows "(press tab to load)".
2. Press `j`/`k` — cursor moves in left pane; right pane updates its title to the highlighted playlist's name.
3. Press `tab` — focus shifts; brief "Loading…" then tracks appear.
4. Press `j`/`k` — cursor moves in right pane.
5. Press `enter` (in right pane) — that track starts playing; status footer outside the browser should reflect the change in the next tick (manual: press `esc` to verify).
6. Press `esc` — back to now-playing.
7. Press `q` — quit.

If anything is off (text wraps oddly at narrow widths, etc.) note it but don't block — the spec accepts manual hand-testing for v1 layout.

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/browser.go
git commit -m "app: render two-column browser view"
```

---

### Task 25: TUI — now-playing keybind footer mentions `l: browse`

**Files:**
- Modify: `internal/app/view.go`

- [ ] **Step 1: Find the existing keybind footer line**

Run: `grep -n "space" internal/app/view.go`

You're looking for the line that lists the now-playing keybinds (matches the README's `space: play/pause   n: next   p: prev   +/-: vol   q: quit`).

- [ ] **Step 2: Add `l: browse` to the footer**

Change the footer string to include `l: browse`. Position is judgement — between `+/-: vol` and `q: quit` reads naturally. Example:

```go
" space: play/pause   n: next   p: prev   +/-: vol   l: browse   q: quit"
```

- [ ] **Step 3: Verify**

Run: `go build ./...`
Expected: clean.

Smoke check the running app — footer now shows `l: browse`.

- [ ] **Step 4: Commit**

```bash
git add internal/app/view.go
git commit -m "app: add 'l: browse' to now-playing keybind footer"
```

---

### Task 26: README — document playlists CLI + `l` keybind

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the keys table**

In `README.md`, find the `## Keys` table and add a row for `l`:

```markdown
| `l` | open playlist browser (browser keys: ↑↓ nav, tab pane, ⏎ play, esc back) |
```

- [ ] **Step 2: Add a CLI subcommand block**

Below the existing CLI examples (or wherever subcommands are listed), add:

```markdown
## Playlists from the CLI

```bash
goove playlists list                      # user + subscription playlists
goove playlists tracks "Liked Songs"      # tracks of a playlist
goove playlists play "Liked Songs"        # play a playlist from the start
goove playlists play "Liked Songs" --track 5   # start from track 5 (1-based)

# 'goove playlist <subcommand>' (singular) is an alias.
# Names match exactly first, then case-insensitive substring; multiple
# matches are listed and the command exits 1.
```
```

(If the README doesn't yet have a CLI section, add one above `## Logs` — the user can decide later if they want a fuller restructure.)

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "readme: document playlists CLI + 'l' browser keybind"
```

---

## Verification before opening the PR

After Task 26, run the full pipeline:

- [ ] `go build ./...` — clean
- [ ] `go test ./...` — all unit tests pass
- [ ] `go test -tags=integration ./internal/music/applescript/` — integration test passes against real Music.app
- [ ] `go run ./cmd/goove` — manual smoke per Task 24's checklist
- [ ] `go install ./cmd/goove` then `goove playlists list`, `goove playlists tracks "<one of yours>"`, `goove playlists play "<one of yours>"` — CLI works against real Music.app

Then open the PR from `feature/playlists` to `main`.
