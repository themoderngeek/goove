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
 space: play/pause   n: next   p: prev   +/-: vol   q: quit
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
| `q` | quit |

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
