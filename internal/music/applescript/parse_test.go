//go:build darwin

package applescript

import (
	"errors"
	"testing"

	"github.com/themoderngeek/goove/internal/music"
)

func TestParseStatusReturnsErrNotRunning(t *testing.T) {
	_, err := parseStatus("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParseStatusReturnsErrNoTrack(t *testing.T) {
	_, err := parseStatus("NO_TRACK\n")
	if !errors.Is(err, music.ErrNoTrack) {
		t.Fatalf("err = %v; want ErrNoTrack", err)
	}
}

func TestParseStatusParsesPlayingTrack(t *testing.T) {
	raw := "Stairway to Heaven\nLed Zeppelin\nLed Zeppelin IV\n221.5\n482.0\nplaying\n75\n"
	np, err := parseStatus(raw)
	if err != nil {
		t.Fatalf("parseStatus err = %v", err)
	}
	if np.Track.Title != "Stairway to Heaven" {
		t.Errorf("Title = %q", np.Track.Title)
	}
	if np.Track.Artist != "Led Zeppelin" {
		t.Errorf("Artist = %q", np.Track.Artist)
	}
	if np.Track.Album != "Led Zeppelin IV" {
		t.Errorf("Album = %q", np.Track.Album)
	}
	if np.Position.Seconds() != 221.5 {
		t.Errorf("Position = %v; want 221.5s", np.Position)
	}
	if np.Duration.Seconds() != 482.0 {
		t.Errorf("Duration = %v; want 482s", np.Duration)
	}
	if !np.IsPlaying {
		t.Errorf("IsPlaying = false; want true")
	}
	if np.Volume != 75 {
		t.Errorf("Volume = %d; want 75", np.Volume)
	}
}

func TestParseStatusReadsPausedState(t *testing.T) {
	raw := "T\nA\nB\n0.0\n100.0\npaused\n50\n"
	np, err := parseStatus(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if np.IsPlaying {
		t.Errorf("IsPlaying = true; want false (state was paused)")
	}
}

func TestParseStatusOnMalformedReturnsErrUnavailable(t *testing.T) {
	_, err := parseStatus("only one line")
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want ErrUnavailable", err)
	}
}

func TestParseStatusOnNonNumericPositionReturnsErrUnavailable(t *testing.T) {
	raw := "T\nA\nB\nNOT_A_NUMBER\n100\nplaying\n50\n"
	_, err := parseStatus(raw)
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want ErrUnavailable", err)
	}
}
