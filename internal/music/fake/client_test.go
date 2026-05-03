package fake

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestArtworkAfterSetReturnsBytes(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	want := []byte{0x89, 0x50, 0x4e, 0x47} // PNG header bytes
	c.SetArtwork(want)

	got, err := c.Artwork(context.Background())
	if err != nil {
		t.Fatalf("Artwork err = %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got = %v; want %v", got, want)
	}
}

func TestArtworkWithoutSetReturnsErrNoArtwork(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)

	_, err := c.Artwork(context.Background())
	if !errors.Is(err, music.ErrNoArtwork) {
		t.Fatalf("err = %v; want ErrNoArtwork", err)
	}
}

func TestArtworkErrOverridesArtwork(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetTrack(domain.Track{Title: "T"}, 100, 0, true)
	c.SetArtwork([]byte{0x89, 0x50})
	c.SetArtworkErr(music.ErrPermission)

	_, err := c.Artwork(context.Background())
	if !errors.Is(err, music.ErrPermission) {
		t.Fatalf("err = %v; want ErrPermission", err)
	}
}

func TestArtworkRespectsForcedErr(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetArtwork([]byte{0x89})
	c.SimulateError(music.ErrUnavailable)

	_, err := c.Artwork(context.Background())
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want ErrUnavailable", err)
	}
}

func TestSetDevicesPopulatesList(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	devices := []domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: true},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true},
	}
	c.SetDevices(devices)

	got, err := c.AirPlayDevices(context.Background())
	if err != nil {
		t.Fatalf("AirPlayDevices err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[0].Name != "Computer" || got[1].Name != "Kitchen Sonos" {
		t.Errorf("got names = %q, %q", got[0].Name, got[1].Name)
	}
}

func TestAirPlayDevicesNotRunning(t *testing.T) {
	c := New()
	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestCurrentAirPlayDeviceReturnsSelected(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Kind: "computer", Available: true, Selected: false},
		{Name: "Kitchen Sonos", Kind: "AirPlay", Available: true, Selected: true},
	})

	got, err := c.CurrentAirPlayDevice(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestCurrentAirPlayDeviceNoneSelectedReturnsErrDeviceNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: false},
	})

	_, err := c.CurrentAirPlayDevice(context.Background())
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestSetAirPlayDeviceUpdatesSelectedFlag(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{
		{Name: "Computer", Selected: true},
		{Name: "Kitchen Sonos", Selected: false},
	})

	if err := c.SetAirPlayDevice(context.Background(), "Kitchen Sonos"); err != nil {
		t.Fatalf("err = %v", err)
	}
	got, _ := c.AirPlayDevices(context.Background())
	if got[0].Selected {
		t.Errorf("Computer.Selected = true; want false")
	}
	if !got[1].Selected {
		t.Errorf("Kitchen Sonos.Selected = false; want true")
	}
}

func TestSetAirPlayDeviceUnknownReturnsErrDeviceNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})

	err := c.SetAirPlayDevice(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestAirPlayDevicesHonoursForcedErr(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetDevices([]domain.AudioDevice{{Name: "Computer", Selected: true}})
	c.SimulateError(music.ErrPermission)

	_, err := c.AirPlayDevices(context.Background())
	if !errors.Is(err, music.ErrPermission) {
		t.Fatalf("err = %v; want ErrPermission", err)
	}
}

func TestPlayIncrementsCounter(t *testing.T) {
	c := New()
	c.Launch(context.Background())

	if err := c.Play(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PlayCalls != 1 {
		t.Errorf("PlayCalls = %d; want 1", c.PlayCalls)
	}
}

func TestPauseIncrementsCounter(t *testing.T) {
	c := New()
	c.Launch(context.Background())

	if err := c.Pause(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PauseCalls != 1 {
		t.Errorf("PauseCalls = %d; want 1", c.PauseCalls)
	}
}

func TestPlayNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New() // not launched
	err := c.Play(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestPauseNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New() // not launched
	err := c.Pause(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestPlaylistsReturnsSeededList(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{
		{Name: "Liked Songs", Kind: "user", TrackCount: 3},
		{Name: "Workout", Kind: "subscription", TrackCount: 5},
	})

	got, err := c.Playlists(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "Liked Songs" || got[1].Name != "Workout" {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistsNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New()
	_, err := c.Playlists(context.Background())
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestPlaylistTracksReturnsSeededTracks(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylistTracks("Liked Songs", []domain.Track{
		{Title: "A", Artist: "X", Duration: 90 * time.Second},
		{Title: "B", Artist: "Y", Duration: 120 * time.Second},
	})

	got, err := c.PlaylistTracks(context.Background(), "Liked Songs")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 || got[0].Title != "A" || got[1].Duration != 120*time.Second {
		t.Errorf("got = %+v", got)
	}
}

func TestPlaylistTracksUnknownNameReturnsErrPlaylistNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	_, err := c.PlaylistTracks(context.Background(), "Atlantis")
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestPlayPlaylistRecordsInvocation(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs", Kind: "user", TrackCount: 3}})

	if err := c.PlayPlaylist(context.Background(), "Liked Songs", 0); err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.PlayPlaylistCalls != 1 {
		t.Errorf("PlayPlaylistCalls = %d; want 1", c.PlayPlaylistCalls)
	}
	rec := c.PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].Name != "Liked Songs" || rec[0].FromIdx != 0 {
		t.Errorf("record = %+v", rec)
	}
}

func TestPlayPlaylistFromIndexRecorded(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})

	c.PlayPlaylist(context.Background(), "Liked Songs", 4)

	rec := c.PlayPlaylistRecord()
	if rec[0].FromIdx != 4 {
		t.Errorf("FromIdx = %d; want 4", rec[0].FromIdx)
	}
}

func TestPlayPlaylistUnknownNameReturnsErrPlaylistNotFound(t *testing.T) {
	c := New()
	c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "Liked Songs"}})

	err := c.PlayPlaylist(context.Background(), "Atlantis", 0)
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestPlayPlaylistNotRunningReturnsErrNotRunning(t *testing.T) {
	c := New()
	err := c.PlayPlaylist(context.Background(), "Liked Songs", 0)
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}
