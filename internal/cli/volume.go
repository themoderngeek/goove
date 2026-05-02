package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/themoderngeek/goove/internal/music"
)

func cmdVolume(args []string, client music.Client, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: volume requires a value (0-100)")
		return 1
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "goove: invalid volume: %s\n", args[0])
		return 1
	}
	// applescript.Client.SetVolume already clamps; passing the raw value is fine.
	// fake.Client.SetVolume also clamps. The behaviour is identical either way.
	if err := client.SetVolume(context.Background(), n); err != nil {
		return errorExit(err, stderr, true)
	}
	return 0
}
