//go:build darwin

package applescript

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/themoderngeek/goove/internal/music"
)

// fakeRunner records the script it was called with and returns scripted output.
type fakeRunner struct {
	script string
	out    []byte
	stderr []byte
	err    error
}

func (f *fakeRunner) Run(ctx context.Context, script string) ([]byte, error) {
	f.script = script
	if f.err != nil {
		return f.out, &runnerErr{err: f.err, stderr: f.stderr}
	}
	return f.out, nil
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
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want wrapping music.ErrUnavailable", err)
	}
}

func TestRunnerPermissionStderrMapsToErrPermission(t *testing.T) {
	r := &fakeRunner{
		err:    errors.New("exit status 1"),
		stderr: []byte("execution error: Not authorized to send Apple events to Music. (-1743)\n"),
	}
	c := New(r)

	_, err := c.IsRunning(context.Background())
	if !errors.Is(err, music.ErrPermission) {
		t.Fatalf("err = %v; want ErrPermission", err)
	}
}

func TestStatusParsesRunnerOutput(t *testing.T) {
	r := &fakeRunner{out: []byte("T\nA\nAlb\n10.0\n200.0\nplaying\n80\n")}
	c := New(r)

	np, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status err = %v", err)
	}
	if r.script != scriptStatus {
		t.Errorf("ran %q; want scriptStatus", r.script)
	}
	if np.Track.Title != "T" {
		t.Errorf("Title = %q", np.Track.Title)
	}
	if !np.IsPlaying {
		t.Error("IsPlaying = false; want true")
	}
	if np.Volume != 80 {
		t.Errorf("Volume = %d; want 80", np.Volume)
	}
	if np.LastSyncedAt.IsZero() {
		t.Error("LastSyncedAt should be stamped by client")
	}
}

func TestStatusReturnsErrNotRunningFromSentinel(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.Status(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestPlayPauseRunsPlayPauseScript(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	if err := c.PlayPause(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.script != scriptPlayPause {
		t.Errorf("ran %q; want scriptPlayPause", r.script)
	}
}

func TestNextAndPrevRunRespectiveScripts(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	c.Next(context.Background())
	if r.script != scriptNext {
		t.Errorf("after Next: ran %q; want scriptNext", r.script)
	}
	c.Prev(context.Background())
	if r.script != scriptPrev {
		t.Errorf("after Prev: ran %q; want scriptPrev", r.script)
	}
}

func TestSetVolumeFormatsScriptWithPercent(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	c.SetVolume(context.Background(), 73)
	want := `tell application "Music" to set sound volume to 73`
	if r.script != want {
		t.Errorf("ran %q; want %q", r.script, want)
	}
}

func TestSetVolumeClampsToZeroAndHundred(t *testing.T) {
	r := &fakeRunner{}
	c := New(r)
	c.SetVolume(context.Background(), 150)
	if !contains(r.script, "to 100") {
		t.Errorf("over-100 should clamp; ran %q", r.script)
	}
	c.SetVolume(context.Background(), -5)
	if !contains(r.script, "to 0") {
		t.Errorf("under-0 should clamp; ran %q", r.script)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestRunnerErrorWithUnrelatedStderrMapsToErrUnavailable(t *testing.T) {
	r := &fakeRunner{
		err:    errors.New("exit status 1"),
		stderr: []byte("execution error: some other apple-events problem.\n"),
	}
	c := New(r)

	_, err := c.IsRunning(context.Background())
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want wrapping music.ErrUnavailable", err)
	}
	if errors.Is(err, music.ErrPermission) {
		t.Fatalf("err = %v; should NOT match ErrPermission", err)
	}
}
