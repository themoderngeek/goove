package cli

import (
	"context"
	"io"

	"github.com/themoderngeek/goove/internal/music"
)

func cmdToggle(client music.Client, stderr io.Writer) int {
	if err := client.PlayPause(context.Background()); err != nil {
		return errorExit(err, stderr, true)
	}
	return 0
}

func cmdNext(client music.Client, stderr io.Writer) int {
	if err := client.Next(context.Background()); err != nil {
		return errorExit(err, stderr, true)
	}
	return 0
}

func cmdPrev(client music.Client, stderr io.Writer) int {
	if err := client.Prev(context.Background()); err != nil {
		return errorExit(err, stderr, true)
	}
	return 0
}

func cmdLaunch(client music.Client, stderr io.Writer) int {
	if err := client.Launch(context.Background()); err != nil {
		// Launch is idempotent — but transient/permission errors still surface.
		// `notRunningHint = false` because launch IS the "run launch first" answer.
		return errorExit(err, stderr, false)
	}
	return 0
}

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
