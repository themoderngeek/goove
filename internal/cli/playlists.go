package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

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

// Forward decl — body in Task 13.

func cmdPlaylistsPlay(args []string, client music.Client, stderr io.Writer) int {
	fmt.Fprintln(stderr, "goove: playlists play not yet implemented")
	return 1
}
