package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music"
)

func TestNewIsNotRunningByDefault(t *testing.T) {
	c := New()
	running, err := c.IsRunning(context.Background())
	if err != nil {
		t.Fatalf("IsRunning err = %v", err)
	}
	if running {
		t.Fatal("expected not running by default")
	}
}

func TestLaunchMakesItRunningAndIdle(t *testing.T) {
	c := New()
	if err := c.Launch(context.Background()); err != nil {
		t.Fatalf("Launch err = %v", err)
	}
	running, _ := c.IsRunning(context.Background())
	if !running {
		t.Fatal("expected running after Launch")
	}
	_, err := c.Status(context.Background())
	if !errors.Is(err, music.ErrNoTrack) {
		t.Fatalf("expected ErrNoTrack, got %v", err)
	}
}

func TestSetTrackThenStatusReturnsIt(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T", Artist: "A", Album: "Alb"}, 240, 30, true)
	np, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status err = %v", err)
	}
	if np.Track.Title != "T" {
		t.Errorf("Title = %q; want %q", np.Track.Title, "T")
	}
	if !np.IsPlaying {
		t.Errorf("IsPlaying = false; want true")
	}
	if np.Duration.Seconds() != 240 {
		t.Errorf("Duration = %v; want 240s", np.Duration)
	}
	if np.Position.Seconds() != 30 {
		t.Errorf("Position = %v; want 30s", np.Position)
	}
}

func TestPlayPauseToggles(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 10, false)

	if err := c.PlayPause(context.Background()); err != nil {
		t.Fatalf("PlayPause err = %v", err)
	}
	np, _ := c.Status(context.Background())
	if !np.IsPlaying {
		t.Fatal("expected playing after first PlayPause")
	}
	c.PlayPause(context.Background())
	np, _ = c.Status(context.Background())
	if np.IsPlaying {
		t.Fatal("expected paused after second PlayPause")
	}
}

func TestSetVolumeClampsAndReports(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, false)

	c.SetVolume(context.Background(), 150)
	np, _ := c.Status(context.Background())
	if np.Volume != 100 {
		t.Errorf("Volume = %d; want 100 (clamped)", np.Volume)
	}
	c.SetVolume(context.Background(), -10)
	np, _ = c.Status(context.Background())
	if np.Volume != 0 {
		t.Errorf("Volume = %d; want 0 (clamped)", np.Volume)
	}
}

func TestStatusWhenNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New()
	_, err := c.Status(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("expected ErrNotRunning, got %v", err)
	}
}

func TestSimulateError(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SimulateError(music.ErrPermission)
	_, err := c.Status(context.Background())
	if !errors.Is(err, music.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}
