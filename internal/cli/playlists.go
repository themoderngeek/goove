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
