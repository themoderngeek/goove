//go:build darwin

package applescript

import (
	"context"
	"errors"
	"testing"
)

// fakeRunner records the script it was called with and returns scripted output.
type fakeRunner struct {
	script string
	out    []byte
	err    error
}

func (f *fakeRunner) Run(ctx context.Context, script string) ([]byte, error) {
	f.script = script
	return f.out, f.err
}

func TestIsRunningReturnsTrue(t *testing.T) {
	r := &fakeRunner{out: []byte("true\n")}
	c := New(r)

	running, err := c.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if !running {
		t.Fatal("expected running = true")
	}
	if r.script != scriptIsRunning {
		t.Errorf("ran %q; want %q", r.script, scriptIsRunning)
	}
}

func TestIsRunningReturnsFalse(t *testing.T) {
	r := &fakeRunner{out: []byte("false\n")}
	c := New(r)

	running, err := c.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if running {
		t.Fatal("expected running = false")
	}
}

func TestLaunchRunsLaunchScript(t *testing.T) {
	r := &fakeRunner{out: []byte("")}
	c := New(r)

	if err := c.Launch(context.Background()); err != nil {
		t.Fatalf("Launch err = %v", err)
	}
	if r.script != scriptLaunch {
		t.Errorf("ran %q; want %q", r.script, scriptLaunch)
	}
}

func TestRunnerErrorBecomesErrUnavailable(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	c := New(r)

	_, err := c.IsRunning(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
