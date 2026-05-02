//go:build darwin

package applescript

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestArtworkOnNotRunningReturnsErrNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.Artwork(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestArtworkOnNoArtSentinelReturnsErrNoArtwork(t *testing.T) {
	r := &fakeRunner{out: []byte("NO_ART\n")}
	c := New(r)
	_, err := c.Artwork(context.Background())
	if !errors.Is(err, music.ErrNoArtwork) {
		t.Fatalf("err = %v; want ErrNoArtwork", err)
	}
}

func TestArtworkOnOKReturnsCacheFileBytes(t *testing.T) {
	r := &fakeRunner{out: []byte("OK\n")}
	c := New(r)

	// Resolve the cache path the same way Client.Artwork does so we can
	// pre-populate the file the runner will then "claim" to have written.
	cachePath, err := artworkCachePath()
	if err != nil {
		t.Fatalf("artworkCachePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0xde, 0xad}
	if err := os.WriteFile(cachePath, want, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	defer os.Remove(cachePath)

	got, err := c.Artwork(context.Background())
	if err != nil {
		t.Fatalf("Artwork err = %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got = %x; want %x", got, want)
	}
}

func TestArtworkRunsScriptWithCachePath(t *testing.T) {
	r := &fakeRunner{out: []byte("NO_ART\n")}
	c := New(r)
	c.Artwork(context.Background())

	cachePath, _ := artworkCachePath()
	if !strings.Contains(r.script, cachePath) {
		t.Errorf("script did not contain cache path %q; script = %q", cachePath, r.script)
	}
	if !strings.Contains(r.script, "raw data of") {
		t.Errorf("script did not use 'raw data of'; script = %q", r.script)
	}
}

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

func TestAirPlayDevicesRunsScript(t *testing.T) {
	r := &fakeRunner{out: []byte("")}
	c := New(r)
	c.AirPlayDevices(context.Background())
	if r.script != scriptAirPlayDevices {
		t.Errorf("ran %q; want scriptAirPlayDevices", r.script)
	}
}

func TestAirPlayDevicesParsesOutput(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\ttrue\n")}
	c := New(r)

	devices, err := c.AirPlayDevices(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "Computer" {
		t.Errorf("got = %+v", devices)
	}
}

func TestAirPlayDevicesNotRunning(t *testing.T) {
	r := &fakeRunner{out: []byte("NOT_RUNNING\n")}
	c := New(r)
	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestCurrentAirPlayDeviceReturnsSelected(t *testing.T) {
	r := &fakeRunner{out: []byte(
		"Computer\tcomputer\ttrue\tfalse\tfalse\n" +
			"Kitchen Sonos\tAirPlay\ttrue\tfalse\ttrue\n",
	)}
	c := New(r)

	got, err := c.CurrentAirPlayDevice(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestCurrentAirPlayDeviceNoneSelectedReturnsErrDeviceNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\tfalse\n")}
	c := New(r)
	_, err := c.CurrentAirPlayDevice(context.Background())
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

// twoCallFakeRunner records every script invocation, so we can verify
// SetAirPlayDevice does the list-then-set sequence.
type twoCallFakeRunner struct {
	scripts []string
	outs    [][]byte
	errs    []error
	idx     int
}

func (f *twoCallFakeRunner) Run(ctx context.Context, script string) ([]byte, error) {
	f.scripts = append(f.scripts, script)
	if f.idx >= len(f.outs) {
		return nil, errors.New("no more outputs scripted")
	}
	out, err := f.outs[f.idx], f.errs[f.idx]
	f.idx++
	return out, err
}

func TestSetAirPlayDeviceCallsListThenSet(t *testing.T) {
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Computer\tcomputer\ttrue\tfalse\ttrue\n" +
				"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n"),
			[]byte("OK\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)

	err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.scripts) != 2 {
		t.Fatalf("script call count = %d; want 2", len(r.scripts))
	}
	if r.scripts[0] != scriptAirPlayDevices {
		t.Errorf("first script = %q; want scriptAirPlayDevices", r.scripts[0])
	}
	if !strings.Contains(r.scripts[1], "Kitchen Sonos") {
		t.Errorf("second script did not contain device name: %q", r.scripts[1])
	}
}

func TestSetAirPlayDeviceUsesExactNameForSetCall(t *testing.T) {
	// User passes substring "kitchen"; matcher resolves to "Kitchen Sonos";
	// the set script should be called with the exact name "Kitchen Sonos".
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Computer\tcomputer\ttrue\tfalse\ttrue\n" +
				"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n"),
			[]byte("OK\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)

	c.SetAirPlayDevice(context.Background(), "kitchen")
	if !strings.Contains(r.scripts[1], "Kitchen Sonos") {
		t.Errorf("set script did not contain exact name 'Kitchen Sonos': %q", r.scripts[1])
	}
}

func TestSetAirPlayDeviceNotFoundReturnsErrDeviceNotFound(t *testing.T) {
	r := &fakeRunner{out: []byte("Computer\tcomputer\ttrue\tfalse\ttrue\n")}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestSetAirPlayDeviceAmbiguousReturnsErrAmbiguousDevice(t *testing.T) {
	r := &fakeRunner{out: []byte(
		"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n" +
			"Office Sonos\tAirPlay\ttrue\tfalse\tfalse\n",
	)}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "sonos")
	if !errors.Is(err, music.ErrAmbiguousDevice) {
		t.Fatalf("err = %v; want ErrAmbiguousDevice", err)
	}
}

func TestSetAirPlayDeviceRaceReturnsErrDeviceNotFound(t *testing.T) {
	// List succeeds; set returns NOT_FOUND (the device disappeared between calls).
	r := &twoCallFakeRunner{
		outs: [][]byte{
			[]byte("Kitchen Sonos\tAirPlay\ttrue\tfalse\ttrue\n"),
			[]byte("NOT_FOUND\n"),
		},
		errs: []error{nil, nil},
	}
	c := New(r)
	err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}
