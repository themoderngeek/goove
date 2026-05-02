package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

// Note: `errors` is added to imports in Task 17 when cmdTargetsSet uses errors.Is.

// deviceJSON is the wire format for `goove targets list --json` and
// `goove targets get --json`. snake_case to match other CLI JSON shapes.
type deviceJSON struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Selected  bool   `json:"selected"`
}

func toDeviceJSON(d domain.AudioDevice) deviceJSON {
	return deviceJSON{
		Name:      d.Name,
		Kind:      d.Kind,
		Available: d.Available,
		Active:    d.Active,
		Selected:  d.Selected,
	}
}

// cmdTargets is the two-level dispatcher for `goove targets <subcommand>`.
func cmdTargets(args []string, client music.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "goove: targets requires a subcommand: list, get, set")
		return 1
	}
	switch args[0] {
	case "list":
		return cmdTargetsList(args[1:], client, stdout, stderr)
	case "get":
		return cmdTargetsGet(args[1:], client, stdout, stderr)
	case "set":
		return cmdTargetsSet(args[1:], client, stderr)
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, "goove targets — manage Music's AirPlay output device")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Usage:")
		fmt.Fprintln(stdout, "  goove targets list [--json]   List all AirPlay devices")
		fmt.Fprintln(stdout, "  goove targets get [--json]    Print the currently-selected device")
		fmt.Fprintln(stdout, "  goove targets set <name>      Set the AirPlay device by name")
		return 0
	default:
		fmt.Fprintf(stderr, "goove: unknown targets subcommand: %s\n", args[0])
		fmt.Fprintln(stderr, "       valid subcommands: list, get, set")
		return 1
	}
}

func cmdTargetsList(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	devices, err := client.AirPlayDevices(context.Background())
	if err != nil {
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		out := make([]deviceJSON, 0, len(devices))
		for _, d := range devices {
			out = append(out, toDeviceJSON(d))
		}
		if err := json.NewEncoder(stdout).Encode(out); err != nil {
			return 1
		}
		return 0
	}

	if len(devices) == 0 {
		fmt.Fprintln(stdout, "(no AirPlay devices visible)")
		return 0
	}

	// Compute the longest name for left alignment.
	maxName := 0
	for _, d := range devices {
		if len(d.Name) > maxName {
			maxName = len(d.Name)
		}
	}
	for _, d := range devices {
		sel := " "
		if d.Selected {
			sel = "*"
		}
		act := " "
		if d.Active {
			act = "▶"
		}
		line := fmt.Sprintf("%s%s %-*s (%s)", sel, act, maxName, d.Name, d.Kind)
		if !d.Available {
			line += "  [unavailable]"
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// Forward decls so the file compiles before T16/T17 add the impls.
// Bodies are intentionally empty — neither is dispatched-to by any T15 test.
// (Go does not require unused parameters to be discarded.)
func cmdTargetsGet(args []string, client music.Client, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonOutput = true
		}
	}

	device, err := client.CurrentAirPlayDevice(context.Background())
	if err != nil {
		// ErrDeviceNotFound is a meaningful state report ("nothing selected"),
		// but for `get` we treat it as a 1-exit since there's no name to print.
		return errorExit(err, stderr, true)
	}

	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(toDeviceJSON(device)); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, device.Name)
	return 0
}

func cmdTargetsSet(args []string, client music.Client, stderr io.Writer) int {
	return 1 // Implemented in Task 17.
}
