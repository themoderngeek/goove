//go:build darwin

package applescript

import (
	"bytes"
	"context"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, script string) ([]byte, error)
}

// runnerErr carries the captured stderr alongside the underlying exec error
// so the Client can recognise specific failure modes (e.g. -1743 permission).
type runnerErr struct {
	err    error
	stderr []byte
}

func (r *runnerErr) Error() string {
	if len(r.stderr) > 0 {
		return r.err.Error() + ": " + string(bytes.TrimSpace(r.stderr))
	}
	return r.err.Error()
}

func (r *runnerErr) Unwrap() error { return r.err }

type OsascriptRunner struct{}

func (OsascriptRunner) Run(ctx context.Context, script string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, &runnerErr{err: err, stderr: stderr.Bytes()}
	}
	return out, nil
}
