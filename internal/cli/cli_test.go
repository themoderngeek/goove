package cli

import (
	"bytes"
	"strings"
	"testing"

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
