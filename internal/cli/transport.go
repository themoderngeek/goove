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
