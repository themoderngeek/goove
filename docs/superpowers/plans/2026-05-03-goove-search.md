# goove — local-library search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/`-triggered search modal to the goove TUI that lets the user find any track in their local Apple Music library, debounced live, and play one with enter.

**Architecture:** Modal overlay (peer to the existing output picker). New `searchState` on `Model`; new `SearchTracks` / `PlayTrack` methods on `music.Client` with AppleScript and fake implementations; pure `domain.RankSearchResults` for ordering. Spec: `docs/superpowers/specs/2026-05-03-goove-search-design.md`.

**Tech Stack:** Go, Bubble Tea (Elm Architecture), AppleScript via `osascript`, lipgloss for rendering, table-driven tests.

---

## File map

**New files**
- `internal/domain/search.go` — `RankSearchResults` pure helper
- `internal/domain/search_test.go` — table-driven tests for ranking
- `internal/app/search.go` — `searchState`, `renderSearch`, `handleSearchKey`, `fetchSearch`
- `internal/app/search_test.go` — modal lifecycle, debounce, seq invalidation, key handling

**Modified files**
- `internal/domain/nowplaying.go` — add `PersistentID string` to `Track`
- `internal/music/client.go` — add `SearchTracks`, `PlayTrack`, `SearchResult`, `ErrTrackNotFound`
- `internal/music/applescript/scripts.go` — add `scriptSearchTracks`, `scriptPlayTrack`
- `internal/music/applescript/parse.go` — add `parseSearchTracks`
- `internal/music/applescript/parse_test.go` — search-result parser tests
- `internal/music/applescript/client.go` — implement `SearchTracks`, `PlayTrack`
- `internal/music/applescript/client_test.go` — script invocation + error mapping tests
- `internal/music/applescript/client_integration_test.go` — real-Music-app integration test (build-tagged)
- `internal/music/fake/client.go` — implement `SearchTracks`, `PlayTrack` + helpers
- `internal/music/fake/client_test.go` — fake search/play tests
- `internal/app/model.go` — add `search *searchState` to `Model`
- `internal/app/messages.go` — add `searchDebounceMsg`, `searchResultsMsg`, `searchPlayedMsg`
- `internal/app/view.go` — short-circuit on `m.search != nil`; update `connectedKeybindsText`
- `internal/app/update.go` — `/` keybind, message handlers, modal short-circuit
- `README.md` — add `/` to the Keys table

---

## Task 1: Feature branch + commit spec

The brainstorming spec is already on disk in the working tree but uncommitted. The feature branch holds spec + plan + implementation together.

**Files:** none modified directly.

- [ ] **Step 1: Create the feature branch**

```bash
git checkout -b feature/search
```

Expected output: `Switched to a new branch 'feature/search'`.

- [ ] **Step 2: Commit the spec and the plan together**

```bash
git add docs/superpowers/specs/2026-05-03-goove-search-design.md docs/superpowers/plans/2026-05-03-goove-search.md
git commit -m "$(cat <<'EOF'
spec: search feature design + implementation plan

Locks in: local-library-only scope, modal-overlay UI ('/' from now-playing),
debounced live results, OR-match across title/artist/album, play-and-close
on enter. Out of scope (deferred): catalog search, multi-token queries,
queueing search results as play context.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Expected output: a commit on `feature/search` with both files added.

- [ ] **Step 3: Verify**

```bash
git log --oneline -1
```

Expected output: one line of the form `<hash> spec: search feature design + implementation plan`.

---

## Task 2: Domain — `Track.PersistentID` + `RankSearchResults`

Adds the persistent-ID field every search result needs to be playable, plus the pure ranker the app layer will call after the OR-matched query returns.

**Files:**
- Modify: `internal/domain/nowplaying.go`
- Create: `internal/domain/search.go`
- Create: `internal/domain/search_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/domain/search_test.go`:

```go
package domain

import (
	"reflect"
	"testing"
)

func TestRankSearchResults_GroupsByMatchSource(t *testing.T) {
	tracks := []Track{
		{Title: "Album Match Only", Artist: "Other", Album: "Stairway"},
		{Title: "Bumble", Artist: "Stairway Band", Album: "X"},
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV"},
		{Title: "Another Stairway", Artist: "Y", Album: "Z"},
	}
	got := RankSearchResults(tracks, "stair")
	want := []Track{
		// Title-match group, alphabetical by title.
		{Title: "Another Stairway", Artist: "Y", Album: "Z"},
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV"},
		// Artist-match group.
		{Title: "Bumble", Artist: "Stairway Band", Album: "X"},
		// Album-match group.
		{Title: "Album Match Only", Artist: "Other", Album: "Stairway"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordering wrong\n got: %v\nwant: %v", got, want)
	}
}

func TestRankSearchResults_CaseInsensitive(t *testing.T) {
	tracks := []Track{
		{Title: "STAIRWAY", Artist: "X", Album: "Y"},
		{Title: "lower", Artist: "STAIR-something", Album: "Y"},
	}
	got := RankSearchResults(tracks, "stair")
	want := []Track{
		{Title: "STAIRWAY", Artist: "X", Album: "Y"},
		{Title: "lower", Artist: "STAIR-something", Album: "Y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("case-insensitive failed\n got: %v\nwant: %v", got, want)
	}
}

func TestRankSearchResults_StableWithinGroup(t *testing.T) {
	// Two tracks with the same lowercase title sort by their input order
	// (stable sort) — neither should be reordered.
	tracks := []Track{
		{Title: "Stair", Artist: "Aaa"},
		{Title: "stair", Artist: "Bbb"},
	}
	got := RankSearchResults(tracks, "stair")
	if got[0].Artist != "Aaa" || got[1].Artist != "Bbb" {
		t.Errorf("stable order broken: got %v", got)
	}
}

func TestRankSearchResults_EmptyInputs(t *testing.T) {
	if got := RankSearchResults(nil, "x"); got != nil {
		t.Errorf("expected nil for nil tracks, got %v", got)
	}
	if got := RankSearchResults([]Track{}, "x"); len(got) != 0 {
		t.Errorf("expected empty for empty tracks, got %v", got)
	}
}
```

- [ ] **Step 2: Run the tests — confirm they fail**

```bash
go test ./internal/domain/ -run RankSearchResults -v
```

Expected: compile error `RankSearchResults is undefined`.

- [ ] **Step 3: Add `PersistentID` to `Track`**

Edit `internal/domain/nowplaying.go`:

```go
type Track struct {
	Title        string
	Artist       string
	Album        string
	Duration     time.Duration // populated by playlist tracks; left zero for NowPlaying.Track
	PersistentID string        // populated by search results; left empty elsewhere — Apple Music's stable per-library track handle, used by PlayTrack
}
```

- [ ] **Step 4: Implement `RankSearchResults`**

Create `internal/domain/search.go`:

```go
package domain

import (
	"sort"
	"strings"
)

// RankSearchResults orders OR-matched tracks by which field the query
// matched: title-matches first, then artist, then album. Within each group
// it sorts alphabetically by lowercased title (stable). Tracks that don't
// match anywhere — defensive; shouldn't happen for an OR-matched input — sort
// last.
//
// Match is case-insensitive substring, mirroring AppleScript's `whose name
// contains` clause.
func RankSearchResults(tracks []Track, query string) []Track {
	if len(tracks) == 0 {
		return tracks
	}
	q := strings.ToLower(query)
	var groups [4][]Track
	for _, t := range tracks {
		switch {
		case strings.Contains(strings.ToLower(t.Title), q):
			groups[0] = append(groups[0], t)
		case strings.Contains(strings.ToLower(t.Artist), q):
			groups[1] = append(groups[1], t)
		case strings.Contains(strings.ToLower(t.Album), q):
			groups[2] = append(groups[2], t)
		default:
			groups[3] = append(groups[3], t)
		}
	}
	for i := range groups {
		g := groups[i]
		sort.SliceStable(g, func(a, b int) bool {
			return strings.ToLower(g[a].Title) < strings.ToLower(g[b].Title)
		})
	}
	out := make([]Track, 0, len(tracks))
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
```

- [ ] **Step 5: Run the tests — confirm they pass**

```bash
go test ./internal/domain/ -v
```

Expected: all four `RankSearchResults` tests PASS, plus existing tests stay green.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/nowplaying.go internal/domain/search.go internal/domain/search_test.go
git commit -m "$(cat <<'EOF'
domain: add Track.PersistentID and RankSearchResults

PersistentID is populated by search results and used by PlayTrack as the
stable handle. RankSearchResults orders OR-matched tracks by where the query
matched (title > artist > album), alphabetical within each group.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Music interface — methods + `SearchResult` + `ErrTrackNotFound` + stub impls

The interface change forces both `applescript.Client` and `fake.Client` to grow the methods. We add minimal stubs returning `ErrUnavailable` so the package compiles; real impls land in tasks 4–6.

**Files:**
- Modify: `internal/music/client.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/fake/client.go`

- [ ] **Step 1: Add the new types and interface methods**

Edit `internal/music/client.go`:

```go
package music

import (
	"context"
	"errors"

	"github.com/themoderngeek/goove/internal/domain"
)

// SearchResult is the wire shape for SearchTracks. Tracks holds at most 100
// rows (the cap is enforced inside the AppleScript). Total carries the full
// underlying match count so callers can render a "100 of N" truncation hint.
type SearchResult struct {
	Tracks []domain.Track
	Total  int
}

type Client interface {
	IsRunning(ctx context.Context) (bool, error)
	Launch(ctx context.Context) error
	Status(ctx context.Context) (domain.NowPlaying, error)
	PlayPause(ctx context.Context) error
	Next(ctx context.Context) error
	Prev(ctx context.Context) error
	SetVolume(ctx context.Context, percent int) error
	Artwork(ctx context.Context) ([]byte, error)
	AirPlayDevices(ctx context.Context) ([]domain.AudioDevice, error)
	CurrentAirPlayDevice(ctx context.Context) (domain.AudioDevice, error)
	SetAirPlayDevice(ctx context.Context, name string) error
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Playlists(ctx context.Context) ([]domain.Playlist, error)
	PlaylistTracks(ctx context.Context, playlistName string) ([]domain.Track, error)
	PlayPlaylist(ctx context.Context, playlistName string, fromTrackIndex int) error

	// SearchTracks returns up to 100 library tracks whose title, artist, or
	// album contains query (case-insensitive). Total in the result is the
	// full underlying match count.
	SearchTracks(ctx context.Context, query string) (SearchResult, error)

	// PlayTrack starts playback of the track with the given persistent ID.
	// Replaces the current play context. Returns ErrTrackNotFound if no
	// track in the library has that ID (e.g. it was deleted).
	PlayTrack(ctx context.Context, persistentID string) error
}

var (
	ErrNotRunning       = errors.New("music: app not running")
	ErrNoTrack          = errors.New("music: no track loaded")
	ErrUnavailable      = errors.New("music: backend call failed")
	ErrPermission       = errors.New("music: automation permission denied")
	ErrNoArtwork        = errors.New("music: track has no artwork")
	ErrDeviceNotFound   = errors.New("music: airplay device not found")
	ErrAmbiguousDevice  = errors.New("music: airplay device name matches multiple devices")
	ErrPlaylistNotFound = errors.New("music: playlist not found")
	ErrTrackNotFound    = errors.New("music: track not found")
)
```

- [ ] **Step 2: Run the build — confirm both impls fail to compile**

```bash
go build ./...
```

Expected: build errors of the form `*Client does not implement music.Client (missing SearchTracks method)` for both `applescript.Client` and `fake.Client`.

- [ ] **Step 3: Add stubs to the AppleScript client**

Edit `internal/music/applescript/client.go`. Append these methods just before the compile-time check at the bottom:

```go
// SearchTracks is implemented in Task 5 — stub returns ErrUnavailable so the
// interface contract is satisfied while this is being built up.
func (c *Client) SearchTracks(ctx context.Context, query string) (music.SearchResult, error) {
	return music.SearchResult{}, music.ErrUnavailable
}

// PlayTrack is implemented in Task 6 — stub returns ErrUnavailable.
func (c *Client) PlayTrack(ctx context.Context, persistentID string) error {
	return music.ErrUnavailable
}
```

- [ ] **Step 4: Add stubs and helpers to the fake client**

Edit `internal/music/fake/client.go`. Add these fields to `Client`:

```go
	// Set by SetTracks; queried by SearchTracks. Distinct from playlistTracks
	// because library search is a property of the whole library, not of any
	// one playlist.
	libraryTracks []domain.Track

	// Records of PlayTrack invocations.
	playTrackRecord []playTrackCall

	PlayTrackCalls int
```

Add the helper type just below `playPlaylistCall`:

```go
type playTrackCall struct {
	PersistentID string
}
```

Append these methods (they will be filled in during Task 4):

```go
// SetLibraryTracks supplies the in-memory library searched by SearchTracks.
func (c *Client) SetLibraryTracks(tracks []domain.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.libraryTracks = tracks
}

// PlayTrackRecord returns a copy of the recorded PlayTrack invocations.
func (c *Client) PlayTrackRecord() []playTrackCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]playTrackCall, len(c.playTrackRecord))
	copy(out, c.playTrackRecord)
	return out
}

// SearchTracks is implemented in Task 4 — stub.
func (c *Client) SearchTracks(ctx context.Context, query string) (music.SearchResult, error) {
	return music.SearchResult{}, music.ErrUnavailable
}

// PlayTrack is implemented in Task 4 — stub.
func (c *Client) PlayTrack(ctx context.Context, persistentID string) error {
	return music.ErrUnavailable
}
```

- [ ] **Step 5: Run the build and tests — both should be green**

```bash
go build ./... && go test ./...
```

Expected: build clean, all existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/music/client.go internal/music/applescript/client.go internal/music/fake/client.go
git commit -m "$(cat <<'EOF'
music: add SearchTracks/PlayTrack to Client interface (stubs)

Adds SearchResult, ErrTrackNotFound, and stubs for both implementations so
the package compiles. Real impls land in the next tasks.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Fake client — `SearchTracks` and `PlayTrack`

The fake's job is to be the deterministic test double the app-layer tests will lean on.

**Files:**
- Modify: `internal/music/fake/client.go`
- Modify: `internal/music/fake/client_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/music/fake/client_test.go`:

```go
func TestSearchTracks_NotRunning(t *testing.T) {
	c := New()
	if _, err := c.SearchTracks(context.Background(), "x"); !errors.Is(err, music.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestSearchTracks_OrMatchesAcrossFields(t *testing.T) {
	c := New()
	_ = c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV", PersistentID: "A"},
		{Title: "Black Dog", Artist: "Led Zeppelin", Album: "IV", PersistentID: "B"},
		{Title: "Wonderwall", Artist: "Oasis", Album: "Morning Glory", PersistentID: "C"},
	})
	got, err := c.SearchTracks(context.Background(), "led")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Total != 2 || len(got.Tracks) != 2 {
		t.Errorf("expected 2 hits, got total=%d len=%d", got.Total, len(got.Tracks))
	}
}

func TestSearchTracks_Cap100(t *testing.T) {
	c := New()
	_ = c.Launch(context.Background())
	tracks := make([]domain.Track, 150)
	for i := range tracks {
		tracks[i] = domain.Track{Title: "match", PersistentID: fmt.Sprintf("p%d", i)}
	}
	c.SetLibraryTracks(tracks)
	got, err := c.SearchTracks(context.Background(), "match")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Total != 150 {
		t.Errorf("expected total=150, got %d", got.Total)
	}
	if len(got.Tracks) != 100 {
		t.Errorf("expected len=100, got %d", len(got.Tracks))
	}
}

func TestPlayTrack_NotRunning(t *testing.T) {
	c := New()
	if err := c.PlayTrack(context.Background(), "A"); !errors.Is(err, music.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestPlayTrack_RecordsCallAndUpdatesNowPlaying(t *testing.T) {
	c := New()
	_ = c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{
		{Title: "Stairway", Artist: "LZ", Album: "IV", PersistentID: "A", Duration: 8 * time.Minute},
	})
	if err := c.PlayTrack(context.Background(), "A"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if c.PlayTrackCalls != 1 || len(c.PlayTrackRecord()) != 1 {
		t.Errorf("expected one recorded call, got %d / %d", c.PlayTrackCalls, len(c.PlayTrackRecord()))
	}
	np, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if np.Track.Title != "Stairway" || !np.IsPlaying {
		t.Errorf("expected Stairway playing, got %+v", np)
	}
}

func TestPlayTrack_TrackNotFound(t *testing.T) {
	c := New()
	_ = c.Launch(context.Background())
	c.SetLibraryTracks([]domain.Track{{PersistentID: "A"}})
	if err := c.PlayTrack(context.Background(), "Z"); !errors.Is(err, music.ErrTrackNotFound) {
		t.Errorf("expected ErrTrackNotFound, got %v", err)
	}
}
```

If `fmt` isn't already imported in this test file, add it to the import list.

- [ ] **Step 2: Run the tests — confirm failures**

```bash
go test ./internal/music/fake/ -run "Search|PlayTrack" -v
```

Expected: assertion failures (the stubs return `ErrUnavailable`).

- [ ] **Step 3: Replace the stubs with real implementations**

Edit `internal/music/fake/client.go`. Replace the `SearchTracks` stub with:

```go
// SearchTracks implements music.Client. OR-matches case-insensitive substring
// across title/artist/album, caps at 100, returns Total = pre-cap match count.
func (c *Client) SearchTracks(ctx context.Context, query string) (music.SearchResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return music.SearchResult{}, c.forcedErr
	}
	if !c.running {
		return music.SearchResult{}, music.ErrNotRunning
	}
	q := strings.ToLower(query)
	var hits []domain.Track
	for _, t := range c.libraryTracks {
		if strings.Contains(strings.ToLower(t.Title), q) ||
			strings.Contains(strings.ToLower(t.Artist), q) ||
			strings.Contains(strings.ToLower(t.Album), q) {
			hits = append(hits, t)
		}
	}
	total := len(hits)
	if len(hits) > 100 {
		hits = hits[:100]
	}
	return music.SearchResult{Tracks: hits, Total: total}, nil
}
```

Replace the `PlayTrack` stub with:

```go
// PlayTrack implements music.Client. Sets now-playing to the matching track
// and flips IsPlaying on. ErrTrackNotFound if no library track has that ID.
func (c *Client) PlayTrack(ctx context.Context, persistentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forcedErr != nil {
		return c.forcedErr
	}
	if !c.running {
		return music.ErrNotRunning
	}
	for _, t := range c.libraryTracks {
		if t.PersistentID == persistentID {
			c.PlayTrackCalls++
			c.playTrackRecord = append(c.playTrackRecord, playTrackCall{PersistentID: persistentID})
			c.hasTrack = true
			c.track = t
			c.duration = t.Duration
			c.position = 0
			c.playing = true
			return nil
		}
	}
	return music.ErrTrackNotFound
}
```

Add `"strings"` to the imports if it's not already there.

- [ ] **Step 4: Run the tests — confirm pass**

```bash
go test ./internal/music/fake/ -v
```

Expected: all tests including the five new ones pass.

- [ ] **Step 5: Commit**

```bash
git add internal/music/fake/client.go internal/music/fake/client_test.go
git commit -m "$(cat <<'EOF'
fake/music: implement SearchTracks and PlayTrack

OR-match across title/artist/album, capped at 100 with Total preserved.
PlayTrack flips IsPlaying and records the persistent ID for assertions.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: AppleScript — search script + parser + `client.SearchTracks`

The hard tasks: get the AppleScript right, parse the tab-separated output, wire it into the client.

**Files:**
- Modify: `internal/music/applescript/scripts.go`
- Modify: `internal/music/applescript/parse.go`
- Modify: `internal/music/applescript/parse_test.go`
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write the failing parser tests**

Append to `internal/music/applescript/parse_test.go`:

```go
func TestParseSearchTracks_NotRunning(t *testing.T) {
	if _, _, err := parseSearchTracks("NOT_RUNNING\n"); !errors.Is(err, music.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestParseSearchTracks_Empty(t *testing.T) {
	tracks, total, err := parseSearchTracks("0\n")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if total != 0 || len(tracks) != 0 {
		t.Errorf("expected empty, got total=%d tracks=%v", total, tracks)
	}
}

func TestParseSearchTracks_HappyPath(t *testing.T) {
	raw := "2\n" +
		"PID-A\tStairway to Heaven\tLed Zeppelin\tIV\t482\n" +
		"PID-B\tBlack Dog\tLed Zeppelin\tIV\t295\n"
	tracks, total, err := parseSearchTracks(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
	if tracks[0].PersistentID != "PID-A" || tracks[0].Title != "Stairway to Heaven" {
		t.Errorf("track[0] wrong: %+v", tracks[0])
	}
	if tracks[1].Duration != 295*time.Second {
		t.Errorf("track[1] duration wrong: %v", tracks[1].Duration)
	}
}

func TestParseSearchTracks_TruncationTotalGreaterThanRows(t *testing.T) {
	raw := "412\n" +
		"PID-A\tA\tArtist\tAlbum\t100\n"
	tracks, total, err := parseSearchTracks(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if total != 412 {
		t.Errorf("expected total=412, got %d", total)
	}
	if len(tracks) != 1 {
		t.Errorf("expected 1 row, got %d", len(tracks))
	}
}

func TestParseSearchTracks_MalformedRowsSkipped(t *testing.T) {
	// Wrong field count and unparseable duration are skipped defensively.
	raw := "3\n" +
		"PID-A\tTitle\tArtist\tAlbum\t100\n" +
		"PID-B\tTitle\tArtistOnly\n" + // 3 fields — bad
		"PID-C\tTitle\tArtist\tAlbum\tNaN\n" + // duration unparseable
		"PID-D\tTitle\tArtist\tAlbum\t200\n"
	tracks, _, err := parseSearchTracks(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("expected 2 valid rows, got %d", len(tracks))
	}
}
```

`time` and `errors` should already be imported by the existing test file; if `errors` is missing, add it.

- [ ] **Step 2: Run the parser tests — confirm failures**

```bash
go test ./internal/music/applescript/ -run ParseSearchTracks -v
```

Expected: compile error `parseSearchTracks is undefined`.

- [ ] **Step 3: Add the AppleScript and parser**

Append to `internal/music/applescript/scripts.go`:

```go
// scriptSearchTracks runs an OR-matched substring search across the library
// and returns:
//
//	first line: total match count (digits only)
//	then up to 100 tab-separated track lines:
//	  persistent_id\ttitle\tartist\talbum\tduration_seconds
//
// %s is the EXACT search query. Returns "NOT_RUNNING" if Music isn't running.
//
// SECURITY: the query is interpolated unescaped — same trust model as
// scriptSetAirPlay and scriptPlaylistTracks. Callers must escape embedded
// `"` and `\` before calling (handled in client.SearchTracks via
// applescriptEscape).
//
// NOTE: track names containing tabs or linefeeds would corrupt parsing —
// accepted MVP limitation matching the rest of the codebase.
const scriptSearchTracks = `tell application "Music"
	if not running then return "NOT_RUNNING"
	set q to "%s"
	set hits to (every track of library playlist 1 whose ¬
		(name contains q) or (artist contains q) or (album contains q))
	set total to count of hits
	if total is 0 then return "0"
	if total > 100 then
		set hits to items 1 thru 100 of hits
	end if
	set out to (total as text)
	repeat with t in hits
		set ln to (persistent ID of t) & tab & (name of t) & tab & ¬
				  (artist of t) & tab & (album of t) & tab & ((duration of t) as text)
		set out to out & linefeed & ln
	end repeat
	return out
end tell`

// scriptPlayTrack starts playback of the track with the given persistent ID.
// %s is the EXACT persistent ID. Returns "OK" | "NOT_RUNNING" | "NOT_FOUND".
const scriptPlayTrack = `tell application "Music"
	if not running then return "NOT_RUNNING"
	try
		play (some track of library playlist 1 whose persistent ID is "%s")
	on error
		return "NOT_FOUND"
	end try
	return "OK"
end tell`
```

- [ ] **Step 4: Add the parser**

Append to `internal/music/applescript/parse.go`:

```go
// parseSearchTracks parses scriptSearchTracks output. The first line is the
// total underlying match count; following lines (up to 100) are tab-separated
// track records. NOT_RUNNING maps to ErrNotRunning. Malformed rows (wrong
// field count or non-numeric duration) are skipped defensively.
func parseSearchTracks(raw string) ([]domain.Track, int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "NOT_RUNNING" {
		return nil, 0, music.ErrNotRunning
	}
	lines := strings.Split(trimmed, "\n")
	total, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: search total parse: %v", music.ErrUnavailable, err)
	}
	var tracks []domain.Track
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			continue
		}
		secs, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		if err != nil {
			continue
		}
		tracks = append(tracks, domain.Track{
			PersistentID: fields[0],
			Title:        fields[1],
			Artist:       fields[2],
			Album:        fields[3],
			Duration:     time.Duration(secs * float64(time.Second)),
		})
	}
	return tracks, total, nil
}
```

- [ ] **Step 5: Run the parser tests — confirm pass**

```bash
go test ./internal/music/applescript/ -run ParseSearchTracks -v
```

Expected: all five new tests PASS.

- [ ] **Step 6: Write the failing client test**

Append to `internal/music/applescript/client_test.go`. The existing tests in this file already use the project's `Runner` test fake — match its style. If the existing test pattern uses a different fixture name, follow it; below uses the convention you'll see in the existing tests.

```go
func TestSearchTracks_HappyPath(t *testing.T) {
	runner := &fakeRunner{
		out: []byte("2\n" +
			"PID-A\tStairway\tLed Zeppelin\tIV\t482\n" +
			"PID-B\tBlack Dog\tLed Zeppelin\tIV\t295\n"),
	}
	c := New(runner)
	got, err := c.SearchTracks(context.Background(), "led")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Total != 2 || len(got.Tracks) != 2 {
		t.Errorf("got Total=%d Tracks=%d", got.Total, len(got.Tracks))
	}
	if !strings.Contains(runner.script, `set q to "led"`) {
		t.Errorf("script did not contain interpolated query: %s", runner.script)
	}
}

func TestSearchTracks_EscapesQuotesAndBackslashes(t *testing.T) {
	runner := &fakeRunner{out: []byte("0\n")}
	c := New(runner)
	if _, err := c.SearchTracks(context.Background(), `quote " and back \`); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(runner.script, `set q to "quote \" and back \\"`) {
		t.Errorf("query not escaped correctly:\n%s", runner.script)
	}
}

func TestSearchTracks_NotRunningMaps(t *testing.T) {
	runner := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(runner)
	if _, err := c.SearchTracks(context.Background(), "x"); !errors.Is(err, music.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}
```

The existing `fakeRunner` (in `client_test.go`) exposes the most recent script as `r.script`. Use that field name; do not introduce a new one.

- [ ] **Step 7: Run the client tests — confirm failures**

```bash
go test ./internal/music/applescript/ -run TestSearchTracks -v
```

Expected: failures because `SearchTracks` is still the stub.

- [ ] **Step 8: Implement `SearchTracks` and the escape helper on the client**

Edit `internal/music/applescript/client.go`. Add the helper near the top of the file (after the `callTimeout` const):

```go
// applescriptEscape escapes embedded double-quote and backslash characters so
// the value can be safely interpolated inside an AppleScript string literal.
// It also strips control characters (tab, linefeed, carriage return) which
// would otherwise corrupt the tab/linefeed-delimited output formats.
func applescriptEscape(s string) string {
	stripped := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(stripped)
}
```

Replace the `SearchTracks` stub with:

```go
// SearchTracks implements music.Client. OR-matches the query against title,
// artist, and album. Returns up to 100 tracks; Total carries the full match
// count for truncation hints.
func (c *Client) SearchTracks(ctx context.Context, query string) (music.SearchResult, error) {
	out, err := c.run(ctx, fmt.Sprintf(scriptSearchTracks, applescriptEscape(query)))
	if err != nil {
		return music.SearchResult{}, err
	}
	tracks, total, err := parseSearchTracks(string(out))
	if err != nil {
		return music.SearchResult{}, err
	}
	return music.SearchResult{Tracks: tracks, Total: total}, nil
}
```

- [ ] **Step 9: Run the full applescript suite — confirm all green**

```bash
go test ./internal/music/applescript/ -v
```

Expected: all tests pass, including the three new client tests and the five new parser tests.

- [ ] **Step 10: Commit**

```bash
git add internal/music/applescript/scripts.go internal/music/applescript/parse.go internal/music/applescript/parse_test.go internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "$(cat <<'EOF'
applescript: implement SearchTracks (OR-match, cap 100, total hint)

Adds scriptSearchTracks, parseSearchTracks, and the SearchTracks client
method. Query is interpolated via a new applescriptEscape helper that
escapes "/\\ and strips control chars — the existing scripts that
interpolate user input are not retro-fitted here; doing so is a separate
concern.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: AppleScript — `client.PlayTrack`

**Files:**
- Modify: `internal/music/applescript/client.go`
- Modify: `internal/music/applescript/client_test.go`

- [ ] **Step 1: Write the failing client tests**

Append to `internal/music/applescript/client_test.go`:

```go
func TestPlayTrack_OK(t *testing.T) {
	runner := &fakeRunner{out: []byte("OK\n")}
	c := New(runner)
	if err := c.PlayTrack(context.Background(), "PID-A"); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
	if !strings.Contains(runner.script, `persistent ID is "PID-A"`) {
		t.Errorf("script missing persistent ID: %s", runner.script)
	}
}

func TestPlayTrack_NotFoundMaps(t *testing.T) {
	runner := &fakeRunner{out: []byte("NOT_FOUND\n")}
	c := New(runner)
	if err := c.PlayTrack(context.Background(), "PID-A"); !errors.Is(err, music.ErrTrackNotFound) {
		t.Errorf("expected ErrTrackNotFound, got %v", err)
	}
}

func TestPlayTrack_NotRunningMaps(t *testing.T) {
	runner := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(runner)
	if err := c.PlayTrack(context.Background(), "PID-A"); !errors.Is(err, music.ErrNotRunning) {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestPlayTrack_EscapesPersistentID(t *testing.T) {
	runner := &fakeRunner{out: []byte("OK\n")}
	c := New(runner)
	_ = c.PlayTrack(context.Background(), `weird " id`)
	if !strings.Contains(runner.script, `persistent ID is "weird \" id"`) {
		t.Errorf("persistent ID not escaped:\n%s", runner.script)
	}
}
```

- [ ] **Step 2: Run — confirm failures**

```bash
go test ./internal/music/applescript/ -run TestPlayTrack -v
```

Expected: failures (stub returns ErrUnavailable).

- [ ] **Step 3: Replace the `PlayTrack` stub**

Replace the stub in `internal/music/applescript/client.go` with:

```go
// PlayTrack implements music.Client. Plays the track with the given persistent
// ID; replaces the current play context. ErrTrackNotFound if no library track
// has that ID.
func (c *Client) PlayTrack(ctx context.Context, persistentID string) error {
	out, err := c.run(ctx, fmt.Sprintf(scriptPlayTrack, applescriptEscape(persistentID)))
	if err != nil {
		return err
	}
	switch strings.TrimSpace(string(out)) {
	case "OK":
		return nil
	case "NOT_RUNNING":
		return music.ErrNotRunning
	case "NOT_FOUND":
		return music.ErrTrackNotFound
	default:
		return fmt.Errorf("%w: unexpected scriptPlayTrack output: %q", music.ErrUnavailable, out)
	}
}
```

- [ ] **Step 4: Run — confirm pass**

```bash
go test ./internal/music/applescript/ -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/music/applescript/client.go internal/music/applescript/client_test.go
git commit -m "$(cat <<'EOF'
applescript: implement PlayTrack (persistent ID, OK/NOT_RUNNING/NOT_FOUND mapping)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: AppleScript integration test (build-tagged)

Exercises `SearchTracks` and `PlayTrack` against the real Music.app. Skipped by default; only runs with `-tags=integration`.

**Files:**
- Modify: `internal/music/applescript/client_integration_test.go`

- [ ] **Step 1: Append the integration tests**

Open `internal/music/applescript/client_integration_test.go` and append:

```go
// TestIntegrationSearchTracks_Smoke runs against the real Music.app library.
// It does not assert on which tracks come back — only that the call succeeds,
// returns a non-negative total, and that any returned track has a non-empty
// persistent ID.
func TestIntegrationSearchTracks_Smoke(t *testing.T) {
	c := NewDefault()
	if running, err := c.IsRunning(context.Background()); err != nil || !running {
		t.Skip("Music.app not running; integration test skipped")
	}
	got, err := c.SearchTracks(context.Background(), "a")
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
	}
	if got.Total < 0 {
		t.Errorf("Total negative: %d", got.Total)
	}
	for _, tr := range got.Tracks {
		if tr.PersistentID == "" {
			t.Errorf("track without PersistentID: %+v", tr)
		}
	}
}
```

(The `PlayTrack` integration path is intentionally not exercised — it would interrupt whatever the user is listening to. The unit path is fully covered.)

- [ ] **Step 2: Run the integration tests (manually, when Music is in a known state)**

```bash
go test -tags=integration ./internal/music/applescript/ -run TestIntegrationSearchTracks -v
```

Expected: PASS, or SKIP if Music.app is not running.

- [ ] **Step 3: Commit**

```bash
git add internal/music/applescript/client_integration_test.go
git commit -m "$(cat <<'EOF'
applescript: integration smoke test for SearchTracks

PlayTrack is unit-tested only — exercising it in an integration test would
interrupt the user's playback.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: App layer — `searchState` + `renderSearch` view

View-only first: define the modal state and how it renders, with no interactivity.

**Files:**
- Modify: `internal/app/model.go`
- Create: `internal/app/search.go`
- Create: `internal/app/search_test.go`
- Modify: `internal/app/view.go`

- [ ] **Step 1: Add `searchState` to the model**

Edit `internal/app/model.go`. Add the type next to `pickerState`:

```go
// searchState is the modal search overlay state.
// nil on Model means "search not open"; non-nil means "search modal showing."
//
// seq is bumped on every keystroke; in-flight debounce ticks and result
// messages carry the seq they were issued under, so stale ones are dropped
// when seq advances. Same pattern as the artwork fetch's track-key guard.
type searchState struct {
	query   string
	seq     uint64
	loading bool
	results []domain.Track
	total   int
	cursor  int
	err     error
}
```

Add a field to `Model`:

```go
	search   *searchState // nil ⇒ search modal not open
```

- [ ] **Step 2: Write the failing render test**

Create `internal/app/search_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func TestRenderSearch_EmptyInput(t *testing.T) {
	got := renderSearch(&searchState{})
	if !strings.Contains(got, "type to search your library") {
		t.Errorf("missing empty-state hint:\n%s", got)
	}
}

func TestRenderSearch_Loading(t *testing.T) {
	got := renderSearch(&searchState{query: "stair", loading: true})
	if !strings.Contains(got, "searching") {
		t.Errorf("missing searching hint:\n%s", got)
	}
}

func TestRenderSearch_NoMatches(t *testing.T) {
	got := renderSearch(&searchState{query: "zzqq"})
	if !strings.Contains(got, "no matches in your library") {
		t.Errorf("missing no-matches text:\n%s", got)
	}
}

func TestRenderSearch_Results(t *testing.T) {
	s := &searchState{
		query: "stair",
		total: 3,
		results: []domain.Track{
			{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV", PersistentID: "A"},
			{Title: "Take the Stairs", Artist: "Phantogram", Album: "Three", PersistentID: "B"},
		},
		cursor: 0,
	}
	got := renderSearch(s)
	if !strings.Contains(got, "Stairway to Heaven") {
		t.Errorf("missing first track:\n%s", got)
	}
	if !strings.Contains(got, "Phantogram") {
		t.Errorf("missing second track:\n%s", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("missing cursor marker:\n%s", got)
	}
}

func TestRenderSearch_TruncationHint(t *testing.T) {
	s := &searchState{query: "the", total: 412}
	for i := 0; i < 100; i++ {
		s.results = append(s.results, domain.Track{Title: "x", PersistentID: "p"})
	}
	got := renderSearch(s)
	if !strings.Contains(got, "100 of 412") {
		t.Errorf("missing truncation hint:\n%s", got)
	}
}

func TestRenderSearch_ErrorFooter(t *testing.T) {
	s := &searchState{query: "stair", err: errSentinel("boom")}
	got := renderSearch(s)
	if !strings.Contains(got, "error: boom") {
		t.Errorf("missing error footer:\n%s", got)
	}
	if !strings.Contains(got, "r retry") {
		t.Errorf("error state should label r as retry:\n%s", got)
	}
}

// errSentinel is a tiny test-only error wrapper.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
```

- [ ] **Step 3: Run — confirm failures**

```bash
go test ./internal/app/ -run RenderSearch -v
```

Expected: compile error `renderSearch is undefined`.

- [ ] **Step 4: Implement `search.go`**

Create `internal/app/search.go`:

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSearch is the modal overlay shown when m.search != nil.
// Replaces the player view entirely (no side-by-side composition), matching
// the picker pattern.
func renderSearch(s *searchState) string {
	var body strings.Builder
	body.WriteString("> ")
	body.WriteString(s.query)
	body.WriteString("_")
	body.WriteString("\n")
	body.WriteString(strings.Repeat("─", 46))
	body.WriteString("\n\n")

	switch {
	case s.query == "":
		body.WriteString(subtitleStyle.Render("type to search your library"))
	case s.loading:
		body.WriteString(subtitleStyle.Render("searching…"))
	case len(s.results) == 0:
		body.WriteString(subtitleStyle.Render("no matches in your library"))
	default:
		for i, t := range s.results {
			cursor := " "
			if i == s.cursor {
				cursor = "▶"
			}
			body.WriteString(fmt.Sprintf("  %s %s\n", cursor, titleStyle.Render(t.Title)))
			body.WriteString("    ")
			body.WriteString(subtitleStyle.Render(t.Artist + " · " + t.Album))
			if i < len(s.results)-1 {
				body.WriteString("\n\n")
			}
		}
		body.WriteString("\n\n")
		if s.total > len(s.results) {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("…  %d of %d — refine the query", len(s.results), s.total)))
		} else {
			body.WriteString(subtitleStyle.Render(fmt.Sprintf("%d results", s.total)))
		}
	}

	header := titleStyle.Render("search")
	card := cardStyle.Render(header + "\n\n" + body.String())

	footerText := " ⏎ play   esc cancel"
	if len(s.results) > 0 {
		footerText = " ↑/↓ navigate   ⏎ play   r refresh   esc cancel"
	}
	footer := footerStyle.Render(footerText)

	out := card + "\n" + footer
	if s.err != nil {
		// Override the footer label to "retry" while an error is showing.
		errFooter := errorStyle.Render("error: " + s.err.Error())
		footerText = " ⏎ play   r retry   esc cancel"
		out = card + "\n" + footerStyle.Render(footerText) + "\n" + errFooter
	}
	return lipgloss.NewStyle().Margin(0, 2).Render(out)
}
```

- [ ] **Step 5: Wire the view short-circuit**

Edit `internal/app/view.go`. Just below the `m.permissionDenied` check (the very first one in `View`), add:

```go
	if m.search != nil {
		return renderSearch(m.search)
	}
```

The order matters: the permission-denied screen still wins, but search beats picker, browser, and now-playing.

- [ ] **Step 6: Run — confirm tests pass**

```bash
go test ./internal/app/ -run RenderSearch -v
```

Expected: all six render tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/model.go internal/app/search.go internal/app/search_test.go internal/app/view.go
git commit -m "$(cat <<'EOF'
app: searchState + renderSearch view (no interactivity yet)

Modal overlay matching the picker pattern. Empty / loading / no-results /
results / truncation / error footer states all rendered.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: App — open `/`, close esc, suppression rules

**Files:**
- Modify: `internal/app/update.go`
- Modify: `internal/app/search_test.go`

- [ ] **Step 1: Write the failing tests**

The existing test helper is `newTestModel()` in `update_test.go` — it returns a `Model` with a fake client and `state = Disconnected{}`. Tests that need other states transition the model explicitly (either by dispatching a `statusMsg` or by direct field assignment in the test, which is acceptable inside the package).

Add a small helper at the top of `internal/app/search_test.go` for the Connected case, then the tests:

```go
// connectedTestModel returns a Model whose fake client is running and whose
// state is Connected. Tests that need a populated library can call
// SetLibraryTracks on the returned client via type assertion.
func connectedTestModel(t *testing.T) (Model, *fake.Client) {
	t.Helper()
	c := fake.New()
	if err := c.Launch(context.Background()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	m := New(c, nil)
	m.state = Connected{}
	return m, c
}

func TestSlash_OpensSearchFromConnected(t *testing.T) {
	m, _ := connectedTestModel(t)
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := out.(Model)
	if mm.search == nil {
		t.Fatalf("expected search state, got nil")
	}
	if mm.search.query != "" {
		t.Errorf("expected empty query, got %q", mm.search.query)
	}
}

func TestSlash_NoOpInDisconnected(t *testing.T) {
	// Default newTestModel is Disconnected.
	m := newTestModel()
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open when Disconnected")
	}
}

func TestSlash_NoOpWhenPickerOpen(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.picker = &pickerState{}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open while picker is open")
	}
}

func TestSlash_NoOpWhenBrowserOpen(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.mode = modeBrowser
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out.(Model).search != nil {
		t.Errorf("search should not open while browser is open")
	}
}

func TestEsc_ClosesSearch(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair"}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if out.(Model).search != nil {
		t.Errorf("esc should close search")
	}
}
```

Make sure the imports at the top of `internal/app/search_test.go` include `context`, `tea "github.com/charmbracelet/bubbletea"`, and `"github.com/themoderngeek/goove/internal/music/fake"`.

- [ ] **Step 2: Run — confirm failures**

```bash
go test ./internal/app/ -run "Slash|Esc_ClosesSearch" -v
```

Expected: failures.

- [ ] **Step 3: Wire the keybind and esc handler in `update.go`**

In `internal/app/update.go`, modify `Update` to short-circuit when search is open. Find the existing handler structure and:

(a) After the existing `m.picker != nil` short-circuit and before the `m.mode == modeBrowser` block, add:

```go
	if m.search != nil {
		return m.handleSearchKey(msg)
	}
```

(Where `msg` is the same `tea.KeyMsg` already in scope. If the picker check captures both the key dispatch and updates differently from the browser/normal flow, mirror whichever pattern the picker already uses — do not invent a new one.)

(b) In the now-playing key switch (where `case "o":` and `case "l":` live), add:

```go
	case "/":
		// Suppress search in Disconnected, when picker is open, or when in browser.
		// permissionDenied is already handled at the top of Update.
		if _, ok := m.state.(Disconnected); ok {
			return m, nil
		}
		if m.picker != nil || m.mode == modeBrowser {
			return m, nil
		}
		m.search = &searchState{}
		return m, nil
```

Add the `handleSearchKey` method to `internal/app/search.go`:

```go
// handleSearchKey routes keystrokes when the search modal is open. Transport
// keys do NOT fall through (unlike the browser); the modal is fully captive
// the way the picker is.
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search = nil
		return m, nil
	}
	return m, nil
}
```

Add the import:

```go
import tea "github.com/charmbracelet/bubbletea"
```

- [ ] **Step 4: Run — confirm pass**

```bash
go test ./internal/app/ -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/app/update.go internal/app/search.go internal/app/search_test.go
git commit -m "$(cat <<'EOF'
app: '/' opens search modal, esc closes; suppress in Disconnected/picker/browser

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: App — typing + debounce + seq invalidation

**Files:**
- Modify: `internal/app/messages.go`
- Modify: `internal/app/search.go`
- Modify: `internal/app/search_test.go`

- [ ] **Step 1: Add the message types**

Append to `internal/app/messages.go`:

```go
// searchDebounceMsg fires 250ms after the last keystroke in the search modal.
// seq is the searchState.seq the tick was scheduled under — handlers drop
// the message if it doesn't match the current seq (stale).
type searchDebounceMsg struct {
	seq uint64
}

// searchResultsMsg carries the result of a SearchTracks call. seq + query
// guard against a result arriving for a query the user has already moved
// on from.
type searchResultsMsg struct {
	seq    uint64
	query  string
	result music.SearchResult
	err    error
}
```

You'll need `"github.com/themoderngeek/goove/internal/music"` in the imports of `messages.go` (it isn't there yet).

- [ ] **Step 2: Write the failing tests**

Append to `internal/app/search_test.go`:

```go
func TestTyping_StartsDebounce_BumpsSeq(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{}
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	mm := out.(Model)
	if mm.search.query != "s" {
		t.Errorf("query: got %q want %q", mm.search.query, "s")
	}
	if mm.search.seq != 1 {
		t.Errorf("seq: got %d want 1", mm.search.seq)
	}
	if cmd == nil {
		t.Errorf("expected debounce Cmd, got nil")
	}
}

func TestBackspace_RemovesLastRune(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 5}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	mm := out.(Model)
	if mm.search.query != "stai" {
		t.Errorf("query: got %q want %q", mm.search.query, "stai")
	}
	if mm.search.seq != 6 {
		t.Errorf("seq: got %d want 6", mm.search.seq)
	}
}

func TestBackspace_OnEmptyQuery_NoOp(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if out.(Model).search.query != "" {
		t.Errorf("expected query still empty")
	}
}

func TestDebounceMsg_StaleSeqDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 10}
	out, cmd := m.Update(searchDebounceMsg{seq: 7})
	if cmd != nil {
		t.Errorf("stale debounce should not fire query")
	}
	if out.(Model).search.loading {
		t.Errorf("stale debounce should not set loading")
	}
}

func TestDebounceMsg_EmptyQueryDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{seq: 1}
	_, cmd := m.Update(searchDebounceMsg{seq: 1})
	if cmd != nil {
		t.Errorf("empty-query debounce should not fire query")
	}
}

func TestDebounceMsg_FreshFiresQuery(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 2}
	out, cmd := m.Update(searchDebounceMsg{seq: 2})
	if cmd == nil {
		t.Errorf("expected SearchTracks Cmd")
	}
	if !out.(Model).search.loading {
		t.Errorf("expected loading=true")
	}
}

func TestResultsMsg_StaleSeqDropped(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 10, loading: true}
	out, _ := m.Update(searchResultsMsg{seq: 5, query: "stair"})
	mm := out.(Model)
	if !mm.search.loading {
		t.Errorf("stale result should not clear loading")
	}
}

func TestResultsMsg_FreshPopulatesAndRanks(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 3, loading: true}
	result := music.SearchResult{
		Tracks: []domain.Track{
			{Title: "Album-only", Album: "Stair Master", PersistentID: "C"},
			{Title: "Stairway", Artist: "X", Album: "Y", PersistentID: "A"},
		},
		Total: 2,
	}
	out, _ := m.Update(searchResultsMsg{seq: 3, query: "stair", result: result})
	mm := out.(Model)
	if mm.search.loading {
		t.Errorf("loading should clear on fresh result")
	}
	if mm.search.total != 2 || len(mm.search.results) != 2 {
		t.Errorf("results not populated: %+v", mm.search)
	}
	// Title-match ranks first.
	if mm.search.results[0].PersistentID != "A" {
		t.Errorf("expected title-match first, got %+v", mm.search.results[0])
	}
	if mm.search.cursor != 0 {
		t.Errorf("cursor should reset to 0, got %d", mm.search.cursor)
	}
}
```

You'll need to import the `music` package in this test file (top of the file).

- [ ] **Step 3: Run — confirm failures**

```bash
go test ./internal/app/ -run "Typing|Backspace|DebounceMsg|ResultsMsg" -v
```

- [ ] **Step 4: Add the `fetchSearch` Cmd and the debounce helper**

Append to `internal/app/search.go`:

```go
import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

const searchDebounceDuration = 250 * time.Millisecond

// scheduleSearchDebounce returns a tea.Tick Cmd that emits a searchDebounceMsg
// stamped with the given seq.
func scheduleSearchDebounce(seq uint64) tea.Cmd {
	return tea.Tick(searchDebounceDuration, func(time.Time) tea.Msg {
		return searchDebounceMsg{seq: seq}
	})
}

// fetchSearch invokes SearchTracks in a goroutine and emits a searchResultsMsg.
func fetchSearch(client music.Client, seq uint64, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := client.SearchTracks(context.Background(), query)
		return searchResultsMsg{seq: seq, query: query, result: res, err: err}
	}
}
```

If `internal/app/search.go` already has its own `import (` block from earlier tasks, merge these into it — don't create a second block.

- [ ] **Step 5: Extend `handleSearchKey` to handle typing and backspace**

Replace the `handleSearchKey` body:

```go
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.search = nil
		return m, nil

	case tea.KeyBackspace:
		if len(m.search.query) == 0 {
			return m, nil
		}
		runes := []rune(m.search.query)
		m.search.query = string(runes[:len(runes)-1])
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)

	case tea.KeyRunes:
		m.search.query += string(msg.Runes)
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	}
	return m, nil
}
```

- [ ] **Step 6: Wire the message handlers in `update.go`**

In `internal/app/update.go` `Update`, add cases (alongside the existing `playlistsMsg`, `playlistTracksMsg` etc. blocks):

```go
	case searchDebounceMsg:
		if m.search == nil || msg.seq != m.search.seq {
			return m, nil
		}
		if m.search.query == "" {
			return m, nil
		}
		m.search.loading = true
		return m, fetchSearch(m.client, m.search.seq, m.search.query)

	case searchResultsMsg:
		if m.search == nil || msg.seq != m.search.seq {
			return m, nil
		}
		m.search.loading = false
		m.search.err = msg.err
		m.search.results = domain.RankSearchResults(msg.result.Tracks, msg.query)
		m.search.total = msg.result.Total
		m.search.cursor = 0
		return m, nil
```

- [ ] **Step 7: Run — confirm pass**

```bash
go test ./internal/app/ -v
```

- [ ] **Step 8: Commit**

```bash
git add internal/app/messages.go internal/app/search.go internal/app/update.go internal/app/search_test.go
git commit -m "$(cat <<'EOF'
app: search modal — typing, debounce (250ms), seq invalidation

Each keystroke bumps seq and schedules a tea.Tick debounce. Stale debounce
ticks and stale result messages are dropped. Empty queries don't fire
SearchTracks. Result ranking goes through domain.RankSearchResults.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: App — navigation, enter (play), `r` (refresh)

**Files:**
- Modify: `internal/app/messages.go`
- Modify: `internal/app/search.go`
- Modify: `internal/app/update.go`
- Modify: `internal/app/search_test.go`

- [ ] **Step 1: Add `searchPlayedMsg`**

Append to `internal/app/messages.go`:

```go
// searchPlayedMsg carries the result of a PlayTrack call from inside search.
// On error, the modal stays open and shows the error footer.
type searchPlayedMsg struct {
	err error
}
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/app/search_test.go`:

```go
func TestArrowDown_MovesCursor(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{
		results: []domain.Track{{Title: "A", PersistentID: "1"}, {Title: "B", PersistentID: "2"}},
	}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if out.(Model).search.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", out.(Model).search.cursor)
	}
}

func TestArrowUp_DecrementsCursor(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{
		results: []domain.Track{{}, {}},
		cursor:  1,
	}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if out.(Model).search.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", out.(Model).search.cursor)
	}
}

func TestArrowDown_AtEnd_NoOp(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{
		results: []domain.Track{{}, {}},
		cursor:  1,
	}
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if out.(Model).search.cursor != 1 {
		t.Errorf("cursor should not advance past end")
	}
}

func TestEnter_PlaysHighlightedAndClosesModal(t *testing.T) {
	m, client := connectedTestModel(t)
	client.SetLibraryTracks([]domain.Track{
		{Title: "A", PersistentID: "PID-A"},
		{Title: "B", PersistentID: "PID-B"},
	})
	m.search = &searchState{
		query: "x",
		results: []domain.Track{
			{Title: "A", PersistentID: "PID-A"},
			{Title: "B", PersistentID: "PID-B"},
		},
		cursor: 1,
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// The cmd should call PlayTrack on PID-B and emit searchPlayedMsg.
	msg := cmd()
	if _, ok := msg.(searchPlayedMsg); !ok {
		t.Errorf("expected searchPlayedMsg, got %T", msg)
	}
	if len(client.PlayTrackRecord()) != 1 || client.PlayTrackRecord()[0].PersistentID != "PID-B" {
		t.Errorf("PlayTrack not called with PID-B: %+v", client.PlayTrackRecord())
	}
}

func TestEnter_NoResults_NoOp(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "x"}
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no Cmd when results empty")
	}
	if out.(Model).search == nil {
		t.Errorf("modal should stay open when there's nothing to play")
	}
}

func TestSearchPlayedMsg_Success_ClosesModal(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "x"}
	out, _ := m.Update(searchPlayedMsg{err: nil})
	if out.(Model).search != nil {
		t.Errorf("modal should close on successful play")
	}
}

func TestSearchPlayedMsg_Error_KeepsModalAndShowsErr(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "x"}
	out, _ := m.Update(searchPlayedMsg{err: errSentinel("boom")})
	mm := out.(Model)
	if mm.search == nil {
		t.Fatalf("modal should stay open on play error")
	}
	if mm.search.err == nil || mm.search.err.Error() != "boom" {
		t.Errorf("expected err 'boom' on modal, got %v", mm.search.err)
	}
}

func TestR_FiresQueryImmediately(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{query: "stair", seq: 3}
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Errorf("expected fetchSearch Cmd from r")
	}
	if !out.(Model).search.loading {
		t.Errorf("expected loading=true")
	}
	if out.(Model).search.seq != 4 {
		t.Errorf("expected seq=4 (bumped), got %d", out.(Model).search.seq)
	}
}

func TestR_EmptyQuery_NoOp(t *testing.T) {
	m, _ := connectedTestModel(t)
	m.search = &searchState{}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Errorf("expected no Cmd from r with empty query")
	}
}
```

All tests in this task use the `connectedTestModel(t)` helper introduced in Task 9. The fake client is already running; tests that need a populated library call `client.SetLibraryTracks(...)` on the returned `*fake.Client`.

- [ ] **Step 3: Run — confirm failures**

```bash
go test ./internal/app/ -run "ArrowDown|ArrowUp|Enter_|SearchPlayedMsg|TestR_" -v
```

- [ ] **Step 4: Implement the new handlers**

Add a `playSelected` Cmd helper to `internal/app/search.go`:

```go
// playSearchSelection invokes PlayTrack for the highlighted result.
func playSearchSelection(client music.Client, persistentID string) tea.Cmd {
	return func() tea.Msg {
		return searchPlayedMsg{err: client.PlayTrack(context.Background(), persistentID)}
	}
}
```

Extend `handleSearchKey` (replacing the previous version):

```go
func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.search = nil
		return m, nil
	case tea.KeyBackspace:
		if len(m.search.query) == 0 {
			return m, nil
		}
		runes := []rune(m.search.query)
		m.search.query = string(runes[:len(runes)-1])
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	case tea.KeyUp:
		if m.search.cursor > 0 {
			m.search.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.search.cursor < len(m.search.results)-1 {
			m.search.cursor++
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.search.results) == 0 {
			return m, nil
		}
		pid := m.search.results[m.search.cursor].PersistentID
		return m, playSearchSelection(m.client, pid)
	case tea.KeyRunes:
		// Single-rune special-cases: 'r' is refresh; everything else appends.
		if len(msg.Runes) == 1 && msg.Runes[0] == 'r' {
			if m.search.query == "" {
				return m, nil
			}
			m.search.seq++
			m.search.loading = true
			m.search.err = nil
			return m, fetchSearch(m.client, m.search.seq, m.search.query)
		}
		m.search.query += string(msg.Runes)
		m.search.seq++
		m.search.results = nil
		m.search.total = 0
		m.search.err = nil
		return m, scheduleSearchDebounce(m.search.seq)
	}
	return m, nil
}
```

Note: `'r'` as refresh inside the modal is in tension with `'r'` as a query character. We accept this: typing `r` while searching will refresh instead of appending. If the user wants `r` literally in their query, they'll get inconsistent behavior — call this out in the spec follow-up if needed.

Also handle `searchPlayedMsg` in `update.go`:

```go
	case searchPlayedMsg:
		if m.search == nil {
			return m, nil
		}
		if msg.err != nil {
			m.search.err = msg.err
			return m, nil
		}
		m.search = nil
		return m, nil
```

- [ ] **Step 5: Run — confirm pass**

```bash
go test ./internal/app/ -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/app/messages.go internal/app/search.go internal/app/update.go internal/app/search_test.go
git commit -m "$(cat <<'EOF'
app: search modal — up/down navigation, enter plays, r refreshes

Enter calls PlayTrack on the highlighted result; on success the modal closes,
on error it stays open and shows the error footer. r re-runs the current
query immediately (skipping debounce). Down beyond end and up beyond start
are no-ops.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Polish — connectedKeybindsText footer + README + manual smoke

**Files:**
- Modify: `internal/app/view.go`
- Modify: `README.md`

- [ ] **Step 1: Add `/` to the now-playing footer**

Edit `internal/app/view.go`:

```go
const connectedKeybindsText = " space: play/pause   n: next   p: prev   +/-: vol   /: search   o: output   l: browse   q: quit"
```

- [ ] **Step 2: Update the README**

Edit `README.md`. In the mock footer at the top:

```
 space: play/pause   n: next   p: prev   +/-: vol   /: search   o: output   l: browse   q: quit
```

In the Keys table, add a row above `o`:

```
| `/` | open search modal (modal keys: type to query, ↑↓ nav, ⏎ play, `r` refresh, esc cancel) |
```

- [ ] **Step 3: Run the full suite plus a build**

```bash
go test ./... && go build -o /tmp/goove ./cmd/goove && /tmp/goove --help
```

Expected: tests pass, binary builds, help output is unchanged (search has no CLI surface).

- [ ] **Step 4: Manual smoke (with Music.app running)**

```bash
/tmp/goove
```

Verify:
1. Press `/` from now-playing → modal opens with empty input and the hint.
2. Type a few characters → after ~250ms a search fires, results render.
3. ↑/↓ moves the cursor.
4. enter plays the highlighted track and the modal closes.
5. Press `/` again, type a no-match query → "no matches in your library".
6. Press esc → modal closes back to now-playing.
7. While in browser (`l`), press `/` → no-op.
8. While in picker (`o`), press `/` → no-op.

If anything is off, treat that as a bug to fix before moving on (do not commit broken behavior).

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go README.md
git commit -m "$(cat <<'EOF'
app+readme: '/: search' in now-playing keybind footer and Keys table

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 6: Final review**

```bash
git log feature/search ^main --oneline
```

Expected: one commit per task, in the order written. Each commit is independently testable / shippable.

---

## Open at completion

When this plan is done, `feature/search` is ready for PR review. Things still to consider before merge:

- Does the modal feel responsive on the user's actual library? If 250ms is too short / too long, tune `searchDebounceDuration`.
- Is the `r`-as-refresh / `r`-as-query-character collision noticeable? If users type words containing "r" without seeing them in the query, we may need to move refresh to a different key (e.g. ctrl-r) or only treat `r` as refresh when results are present.
- Does `goove help` need a mention? It's a TUI-only feature; current convention is "TUI keys are shown by the TUI itself, CLI subcommands are shown by `help`." We're not adding to `help` — confirm that's the right call during review.
