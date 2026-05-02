// Package cli implements the goove subcommand mode (goove status, goove toggle, etc.).
//
// Run is the entry point: cmd/goove/main.go dispatches into this package when the
// first os.Arg looks like a subcommand or help flag. The package has zero
// dependency on internal/app — it's a separate frontend consuming the same
// music.Client interface.
package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/themoderngeek/goove/internal/music"
)

const usageText = `goove — Apple Music TUI controller

Usage:
  goove                       Launch the TUI
  goove status [--json]       Print the current track (one line)
  goove play                  Start playback (no-op if already playing)
  goove pause                 Pause playback (no-op if already paused)
  goove toggle                Play/pause toggle
  goove next                  Skip to the next track
  goove prev                  Skip to the previous track
  goove volume <0..100>       Set the volume (silently clamps out-of-range)
  goove launch                Launch Apple Music if not running
  goove targets list|get|set [name]   Inspect or change the AirPlay device
  goove help, --help, -h      Show this message

Logs: ~/Library/Logs/goove/goove.log (TUI mode only)
Project: github.com/themoderngeek/goove
`

// Run is the CLI entry point. Returns the exit code.
func Run(args []string, client music.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 1
	}
	switch args[0] {
	case "status":
		return cmdStatus(args[1:], client, stdout, stderr)
	case "play":
		return cmdPlay(client, stderr)
	case "pause":
		return cmdPause(client, stderr)
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
}

// errorExit maps a music.Client error to a stderr message and an exit code.
// Used by every command handler that calls a music.Client method.
//
// notRunningHint controls whether the ErrNotRunning message includes the
// "(run 'goove launch' first)" suffix — true for transport commands,
// false for `status` (which is a state-query, not a state-mutator).
func errorExit(err error, stderr io.Writer, notRunningHint bool) int {
	switch {
	case errors.Is(err, music.ErrPermission):
		fmt.Fprintln(stderr, "goove: not authorised to control Music — System Settings → Privacy & Security → Automation")
		return 2
	case errors.Is(err, music.ErrNotRunning):
		if notRunningHint {
			fmt.Fprintln(stderr, "goove: Apple Music isn't running (run 'goove launch' first)")
		} else {
			fmt.Fprintln(stderr, "goove: Apple Music isn't running")
		}
		return 1
	default:
		fmt.Fprintf(stderr, "goove: %v\n", err)
		return 1
	}
}
