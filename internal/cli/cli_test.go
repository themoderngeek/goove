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
