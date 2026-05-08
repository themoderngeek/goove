# goove

A small TUI for controlling Apple Music on macOS, written in Go.

```
┌─ Now Playing ──────────────────────────────────────────────┐
│  ▓ART▓  ▶  Stairway to Heaven                              │
│         Led Zeppelin · Led Zeppelin IV                     │
│         ▮▮▮▮▮▮▮▮▯▯▯▯▯▯▯▯▯  3:42 / 8:02   vol 50%           │
└────────────────────────────────────────────────────────────┘
┌─ Playlists ───┐┌─ Liked Songs (now playing) ──────────────┐
│ ▶ Liked Songs ││   1. Black Dog          Led Zeppelin     │
│   Recent      ││   2. Rock and Roll      Led Zeppelin     │
│   Top 25      ││ ▶ 3. Stairway to Heaven Led Zeppelin     │
└───────────────┘│   4. Misty Mountain Hop Led Zeppelin     │
┌─ Search ──────┐│                                          │
│ /led ze       ││                                          │
│  3 results    ││                                          │
└───────────────┘│                                          │
┌─ Output ──────┐│                                          │
│ ● MacBook     ││                                          │
│   Sonos       ││                                          │
└───────────────┘└──────────────────────────────────────────┘
 space:play/pause  n:next  p:prev  +/-:vol  q:quit · j/k:nav  ⏎:play
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

### Globals (work everywhere)

| key | action |
|---|---|
| `space` | play / pause (or launch Music if Disconnected) |
| `n` | next track |
| `p` | previous track |
| `+` / `=` | volume +5% |
| `-` | volume −5% |
| `q` | quit |
| `Tab` / `Shift-Tab` | cycle focus through Playlists → Search → Output → Main |
| `1` / `2` / `3` / `4` | jump focus to Playlists / Search / Output / Main |
| `/` | focus the Search panel and start typing |
| `o` | focus the Output panel |

### Panel-scoped

| panel | key | action |
|---|---|---|
| Playlists | `j` / `k` / `↑` / `↓` | move cursor (live-previews tracks in main pane) |
| Playlists | `⏎` | play the highlighted playlist |
| Search (idle) | any printable | enter input mode and start the query |
| Search (input) | `Backspace` | remove last rune |
| Search (input) | `⏎` | run the search; results show in main pane |
| Search (input) | `Esc` | clear and exit input mode |
| Output | `j` / `k` / `↑` / `↓` | move cursor |
| Output | `⏎` | switch audio to the highlighted device |
| Main | `j` / `k` / `↑` / `↓` | move cursor |
| Main | `⏎` | play the highlighted track |
| Main | `Esc` | (search-results mode only) return to selected playlist |

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
make tools          # install pinned dev tools (one-time)
make help           # list all targets
make test           # unit tests
make ci             # everything CI runs (fmt, vet, lint, vuln, race tests, build)
make run            # run from source
make build          # produce a binary
```

Integration tests (hit real Music.app):

```bash
make test-integration
```

The design lives in [`docs/superpowers/specs/2026-04-30-goove-mvp-design.md`](docs/superpowers/specs/2026-04-30-goove-mvp-design.md).
The plan it was built against lives in [`docs/superpowers/plans/2026-04-30-goove-mvp.md`](docs/superpowers/plans/2026-04-30-goove-mvp.md).
The TUI overhaul (LazyGit-inspired multi-panel layout) is specced in
[`docs/superpowers/specs/2026-05-04-tui-overhaul-design.md`](docs/superpowers/specs/2026-05-04-tui-overhaul-design.md)
and planned in
[`docs/superpowers/plans/2026-05-04-tui-overhaul.md`](docs/superpowers/plans/2026-05-04-tui-overhaul.md).

## License

See `LICENSE`.
