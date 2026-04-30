//go:build darwin

package applescript

import (
	"context"
	"os/exec"
)

// Runner executes an AppleScript and returns the raw stdout, or an error
// that wraps the underlying *exec.ExitError when osascript exits non-zero.
// stderr is discarded by the real implementation; tests can inspect via fakes.
type Runner interface {
	Run(ctx context.Context, script string) ([]byte, error)
}

// OsascriptRunner runs scripts via the osascript binary.
type OsascriptRunner struct{}

func (OsascriptRunner) Run(ctx context.Context, script string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.Output()
	return out, err
}
