# goove

A small TUI for controlling Apple Music on macOS, written in Go.

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
 space: play/pause   n: next   p: prev   +/-: vol   /: search   o: output   l: browse   q: quit
```

## Install

```bash
go install github.com/themoderngeek/goove/cmd/goove@latest
```

This drops a `goove` binary into `$GOBIN` (or `$HOME/go/bin`).

## Run

```bash
goove
```

On first run macOS will ask for permission to control Music — say yes once.
If you say no, you can re-enable it under
**System Settings → Privacy & Security → Automation**.

## Keys

| key | action |
|---|---|
| `space` | play / pause (or launch Music if not running) |
| `n` | next track |
| `p` | previous track |
| `+` / `=` | volume +5% |
| `-` | volume −5% |
| `/` | open search modal (modal keys: type to query, ↑↓ nav, ⏎ play, `^R` refresh, esc cancel) |
| `o` | open output (AirPlay) picker (picker keys: ↑↓ nav, ⏎ select, esc cancel) |
| `l` | open playlist browser (browser keys: ↑↓ nav, tab pane, ⏎ play, `r` refresh, esc back) |
| `q` | quit |

## CLI commands

Every action is also available as a one-shot subcommand, so goove works from
scripts and keyboard shortcuts without launching the TUI.

```bash
goove status [--json]                 # current track (one line)
goove play                            # start playback
goove pause                           # pause playback
goove toggle                          # play/pause toggle
goove next                            # skip forward
goove prev                            # skip backward
goove volume <0..100>                 # set volume (silently clamps)
goove launch                          # launch Apple Music if not running

goove targets list [--json]           # AirPlay devices
goove targets get  [--json]           # currently selected device
goove targets set  <name>             # route audio to <name>

goove playlists list                          # user + subscription playlists
goove playlists tracks "Liked Songs"          # tracks of a playlist
goove playlists play   "Liked Songs"          # play a playlist from the start
goove playlists play   "Liked Songs" --track 5   # start from track 5 (1-based)

goove help
```

`playlist` (singular) is an alias for `playlists`. Playlist and target names
match exactly first, then by case-insensitive substring; multiple matches are
listed and the command exits 1.

## Logs

Structured logs write to `~/Library/Logs/goove/goove.log`.
Set `GOOVE_LOG=debug` for verbose logging.

## Development

```bash
go test ./...                                            # unit tests
go test -tags=integration ./internal/music/applescript/  # hits real Music.app
go run ./cmd/goove                                       # run from source
go build -o goove ./cmd/goove                            # produce a binary
```

The design lives in [`docs/superpowers/specs/2026-04-30-goove-mvp-design.md`](docs/superpowers/specs/2026-04-30-goove-mvp-design.md).
The plan it was built against lives in [`docs/superpowers/plans/2026-04-30-goove-mvp.md`](docs/superpowers/plans/2026-04-30-goove-mvp.md).

## License

See `LICENSE`.
