package cli

import (
	"bytes"
	"context"
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
