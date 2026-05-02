package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func TestHelpFlagPrintsUsageToStdoutExits0(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			c := fake.New()

			code := Run([]string{arg}, c, &stdout, &stderr)

			if code != 0 {
				t.Errorf("exit = %d; want 0", code)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Errorf("stdout did not contain 'Usage:': %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}

func TestUnknownCommandPrintsErrorAndUsageToStderrExits1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := fake.New()

	code := Run([]string{"frobnicate"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "unknown command: frobnicate") {
		t.Errorf("stderr missing 'unknown command' message: %q", got)
	}
	if !strings.Contains(got, "Usage:") {
		t.Errorf("stderr missing usage block: %q", got)
	}
}

func TestNoArgsReturnsUsageToStderrExits1(t *testing.T) {
	// Defensive: main shouldn't call Run with no args, but if it does we
	// fall back to printing usage to stderr and returning 1.
	var stdout, stderr bytes.Buffer
	c := fake.New()

	code := Run([]string{}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr missing usage block: %q", stderr.String())
	}
}

func setupRunningClient(t *testing.T) *fake.Client {
	t.Helper()
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	return c
}

func TestToggleSuccessSilentExit0(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"toggle"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if c.PlayPauseCalls != 1 {
		t.Errorf("PlayPauseCalls = %d; want 1", c.PlayPauseCalls)
	}
}

func TestToggleNotRunningExit1WithHint(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"toggle"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "goove launch") {
		t.Errorf("stderr missing 'goove launch' hint: %q", stderr.String())
	}
}

func TestTogglePermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"toggle"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "not authorised") {
		t.Errorf("stderr missing permission message: %q", stderr.String())
	}
}

func TestNextSuccessIncrementsCounter(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"next"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if c.NextCalls != 1 {
		t.Errorf("NextCalls = %d; want 1", c.NextCalls)
	}
}

func TestNextNotRunningExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"next"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
}

func TestPrevSuccessIncrementsCounter(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"prev"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if c.PrevCalls != 1 {
		t.Errorf("PrevCalls = %d; want 1", c.PrevCalls)
	}
}

func TestPrevNotRunningExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"prev"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
}

func TestLaunchSuccessFromNotRunning(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"launch"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if c.LaunchCalls != 1 {
		t.Errorf("LaunchCalls = %d; want 1", c.LaunchCalls)
	}
	running, _ := c.IsRunning(context.Background())
	if !running {
		t.Errorf("expected fake to be running after Launch")
	}
}

func TestLaunchSuccessWhenAlreadyRunning(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) // already running before our call
	var stdout, stderr bytes.Buffer

	code := Run([]string{"launch"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0 (launch is idempotent)", code)
	}
}

func TestLaunchPermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"launch"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "not authorised") {
		t.Errorf("stderr missing permission message: %q", stderr.String())
	}
}

func TestVolumeSuccessSetsValue(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume", "73"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if c.SetVolumeCalls != 1 {
		t.Errorf("SetVolumeCalls = %d; want 1", c.SetVolumeCalls)
	}
	// fake.Client.SetVolume clamps too, so reading back the volume confirms 73.
	np, _ := c.Status(context.Background())
	if np.Volume != 73 {
		t.Errorf("Volume = %d; want 73", np.Volume)
	}
}

func TestVolumeMissingArgExit1(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "volume requires a value") {
		t.Errorf("stderr missing 'requires a value': %q", stderr.String())
	}
	if c.SetVolumeCalls != 0 {
		t.Errorf("SetVolumeCalls = %d; want 0 (no client call on bad args)", c.SetVolumeCalls)
	}
}

func TestVolumeInvalidArgExit1(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume", "loud"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid volume") {
		t.Errorf("stderr missing 'invalid volume': %q", stderr.String())
	}
	if c.SetVolumeCalls != 0 {
		t.Errorf("SetVolumeCalls = %d; want 0", c.SetVolumeCalls)
	}
}

func TestVolumeClampHigh(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume", "200"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0 (out-of-range silently clamps)", code)
	}
	np, _ := c.Status(context.Background())
	if np.Volume != 100 {
		t.Errorf("Volume = %d; want 100 (clamped)", np.Volume)
	}
}

func TestVolumeClampLow(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume", "-10"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	np, _ := c.Status(context.Background())
	if np.Volume != 0 {
		t.Errorf("Volume = %d; want 0 (clamped)", np.Volume)
	}
}

func TestVolumeNotRunningExit1WithHint(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"volume", "50"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestStatusPlainConnectedPlaying(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "Hippie Sunshine", Artist: "Kasabian", Album: "ACT III"}, 186, 61, true)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "▶") {
		t.Errorf("stdout missing playing symbol ▶: %q", got)
	}
	if !strings.Contains(got, "Hippie Sunshine") {
		t.Errorf("stdout missing title: %q", got)
	}
	if !strings.Contains(got, "Kasabian") {
		t.Errorf("stdout missing artist: %q", got)
	}
	if !strings.Contains(got, "1:01") {
		t.Errorf("stdout missing position 1:01: %q", got)
	}
	if !strings.Contains(got, "3:06") {
		t.Errorf("stdout missing duration 3:06: %q", got)
	}
	// Volume should be present — fake's default is 50.
	if !strings.Contains(got, "%") {
		t.Errorf("stdout missing volume percentage: %q", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestStatusPlainConnectedPaused(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, false)
	var stdout, stderr bytes.Buffer

	Run([]string{"status"}, c, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "⏸") {
		t.Errorf("stdout missing paused symbol ⏸: %q", stdout.String())
	}
}

func TestStatusPlainConnectedNoArtist(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T", Artist: "", Album: "A"}, 100, 0, true)
	var stdout, stderr bytes.Buffer

	Run([]string{"status"}, c, &stdout, &stderr)

	got := stdout.String()
	// Should NOT contain " — " (the artist separator) when artist is empty.
	if strings.Contains(got, " — ") {
		t.Errorf("stdout should not have ' — ' separator when artist is empty: %q", got)
	}
}

func TestStatusPlainIdleExit0(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) // running but no track set
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0 (Idle is a successful state report)", code)
	}
	if !strings.Contains(stdout.String(), "(no track loaded)") {
		t.Errorf("stdout missing '(no track loaded)': %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestStatusNotRunningExit1NoHint(t *testing.T) {
	c := fake.New() // not running
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", got)
	}
	// status should NOT include the launch hint.
	if strings.Contains(got, "goove launch") {
		t.Errorf("stderr should NOT include 'goove launch' hint for status: %q", got)
	}
}

func TestStatusPermissionExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
}

func TestStatusUnavailableExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrUnavailable)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
}

func TestStatusJSONConnected(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T", Artist: "A", Album: "B"}, 186, 61, true)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status", "--json"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	// Parse the JSON to verify shape.
	var got struct {
		IsPlaying bool `json:"is_playing"`
		Track     *struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Album  string `json:"album"`
		} `json:"track"`
		PositionSec *int `json:"position_sec"`
		DurationSec *int `json:"duration_sec"`
		Volume      *int `json:"volume"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if !got.IsPlaying {
		t.Errorf("IsPlaying = false; want true")
	}
	if got.Track == nil || got.Track.Title != "T" || got.Track.Artist != "A" || got.Track.Album != "B" {
		t.Errorf("Track = %+v", got.Track)
	}
	if got.PositionSec == nil || *got.PositionSec != 61 {
		t.Errorf("PositionSec = %v; want 61", got.PositionSec)
	}
	if got.DurationSec == nil || *got.DurationSec != 186 {
		t.Errorf("DurationSec = %v; want 186", got.DurationSec)
	}
	if got.Volume == nil || *got.Volume != 50 {
		t.Errorf("Volume = %v; want 50 (fake default)", got.Volume)
	}
}

func TestStatusJSONShortFlagJEquivalent(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status", "-j"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	// Output must be valid JSON (not the plain-text format).
	var any map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &any); err != nil {
		t.Errorf("output is not valid JSON with -j: %v\n%q", err, stdout.String())
	}
}

func TestStatusJSONIdleOmitsOptionalFields(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background()) // running but no track
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status", "--json"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	// Validate JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, got)
	}
	if v, ok := parsed["is_playing"]; !ok || v != false {
		t.Errorf("is_playing = %v; want false", v)
	}
	if v, ok := parsed["track"]; !ok || v != nil {
		t.Errorf("track = %v; want nil", v)
	}
	if _, ok := parsed["volume"]; ok {
		t.Errorf("volume should be omitted in Idle JSON output: %q", got)
	}
	if _, ok := parsed["position_sec"]; ok {
		t.Errorf("position_sec should be omitted in Idle JSON output: %q", got)
	}
}

func TestStatusJSONErrorPathStillPrintsToStderr(t *testing.T) {
	c := fake.New()
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"status", "--json"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
	// stdout MUST be empty (don't pollute the JSON pipe with anything).
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on permission error in --json mode: %q", stdout.String())
	}
	// stderr gets the plain-text error.
	if !strings.Contains(stderr.String(), "not authorised") {
		t.Errorf("stderr missing permission message: %q", stderr.String())
	}
}

func TestTargetsListPlainConnected(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Active: true},
		{Name: "Office", Kind: "AirPlay", Available: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "Computer") {
		t.Errorf("stdout missing Computer: %q", got)
	}
	if !strings.Contains(got, "*") {
		t.Errorf("stdout missing selected marker '*': %q", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("stdout missing active marker '▶': %q", got)
	}
	if !strings.Contains(got, "unavailable") {
		t.Errorf("stdout missing 'unavailable' annotation for Office: %q", got)
	}
}

func TestTargetsListJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list", "--json"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0]["name"] != "Computer" {
		t.Errorf("name = %v; want Computer", got[0]["name"])
	}
	if got[0]["selected"] != true {
		t.Errorf("selected = %v; want true", got[0]["selected"])
	}
}

func TestTargetsListEmptyPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if !strings.Contains(stdout.String(), "(no AirPlay devices visible)") {
		t.Errorf("stdout missing empty marker: %q", stdout.String())
	}
}

func TestTargetsListEmptyJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{})
	var stdout, stderr bytes.Buffer

	Run([]string{"targets", "list", "--json"}, c, &stdout, &stderr)
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Errorf("stdout = %q; want '[]'", stdout.String())
	}
}

func TestTargetsListNotRunningExit1(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "list"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestTargetsNoSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a subcommand") {
		t.Errorf("stderr missing 'requires a subcommand': %q", stderr.String())
	}
}

func TestTargetsUnknownSubcommandExit1(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "frobnicate"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Errorf("stderr missing unknown subcommand name: %q", stderr.String())
	}
}

func TestTargetsHelpFlag(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		t.Run(arg, func(t *testing.T) {
			c := fake.New()
			var stdout, stderr bytes.Buffer

			code := Run([]string{"targets", arg}, c, &stdout, &stderr)
			if code != 0 {
				t.Errorf("exit = %d; want 0", code)
			}
			if !strings.Contains(stdout.String(), "manage Music's AirPlay") {
				t.Errorf("stdout missing targets-specific help: %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}

func TestTargetsGetPlain(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
		{Name: "Kitchen Sonos", Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if strings.TrimSpace(stdout.String()) != "Kitchen Sonos" {
		t.Errorf("stdout = %q; want 'Kitchen Sonos'", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestTargetsGetJSON(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Selected: true},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get", "--json"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%q", err, stdout.String())
	}
	if got["name"] != "Kitchen Sonos" {
		t.Errorf("name = %v; want Kitchen Sonos", got["name"])
	}
	if got["selected"] != true {
		t.Errorf("selected = %v; want true", got["selected"])
	}
}

func TestTargetsGetNoneSelectedExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "get"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1 (no device selected)", code)
	}
}

func TestTargetsSetSuccess(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Kitchen Sonos"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Kitchen Sonos" {
		t.Errorf("current = %q; want Kitchen Sonos", cur.Name)
	}
}

func TestTargetsSetMissingNameExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a device name") {
		t.Errorf("stderr missing 'requires a device name': %q", stderr.String())
	}
}

func TestTargetsSetNotFoundExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Atlantis"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "airplay device not found: Atlantis") {
		t.Errorf("stderr missing 'not found: Atlantis': %q", stderr.String())
	}
}

func TestTargetsSetAmbiguousExit1(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Kitchen Sonos"},
		{Name: "Office Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "sonos"}, c, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "matches multiple") {
		t.Errorf("stderr missing 'matches multiple': %q", got)
	}
	if !strings.Contains(got, "Kitchen Sonos") || !strings.Contains(got, "Office Sonos") {
		t.Errorf("stderr should list both matches: %q", got)
	}
}

func TestTargetsSetExactMatchPriority(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Living Room", Selected: false},
		{Name: "Living Room Speakers", Selected: false},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "Living Room"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0 (exact match should win, no ambiguity)", code)
	}
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Living Room" {
		t.Errorf("current = %q; want exact 'Living Room'", cur.Name)
	}
}

func TestTargetsSetSubstringMatch(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos"},
	})
	var stdout, stderr bytes.Buffer

	code := Run([]string{"targets", "set", "kitchen"}, c, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	cur, _ := c.CurrentAirPlayDevice(context.Background())
	if cur.Name != "Kitchen Sonos" {
		t.Errorf("current = %q; want Kitchen Sonos (resolved from 'kitchen')", cur.Name)
	}
}

func TestPlaySuccessSilentExit0(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if c.PlayCalls != 1 {
		t.Errorf("PlayCalls = %d; want 1", c.PlayCalls)
	}
}

func TestPlayNotRunningExit1WithHint(t *testing.T) {
	c := fake.New() // not launched
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "goove launch") {
		t.Errorf("stderr missing 'goove launch' hint: %q", stderr.String())
	}
}

func TestPlayPermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"play"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
	if !strings.Contains(stderr.String(), "not authorised") {
		t.Errorf("stderr missing permission message: %q", stderr.String())
	}
}

func TestPauseSuccessSilentExit0(t *testing.T) {
	c := setupRunningClient(t)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if c.PauseCalls != 1 {
		t.Errorf("PauseCalls = %d; want 1", c.PauseCalls)
	}
}

func TestPauseNotRunningExit1WithHint(t *testing.T) {
	c := fake.New()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), "isn't running") {
		t.Errorf("stderr missing 'isn't running': %q", stderr.String())
	}
}

func TestPausePermissionDeniedExit2(t *testing.T) {
	c := fake.New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	var stdout, stderr bytes.Buffer

	code := Run([]string{"pause"}, c, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d; want 2", code)
	}
}
