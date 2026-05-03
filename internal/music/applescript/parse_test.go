//go:build darwin

package applescript

import (
	"errors"
	"testing"
	"time"

	"github.com/themoderngeek/goove/internal/domain"
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

func TestParseStatusHandlesCRLFLineEndings(t *testing.T) {
	raw := "T\r\nA\r\nAlb\r\n10.0\r\n200.0\r\nplaying\r\n80\r\n"
	np, err := parseStatus(raw)
	if err != nil {
		t.Fatalf("parseStatus err = %v", err)
	}
	if np.Track.Title != "T" {
		t.Errorf("Title = %q; want %q", np.Track.Title, "T")
	}
	if np.Volume != 80 {
		t.Errorf("Volume = %d; want 80", np.Volume)
	}
	if !np.IsPlaying {
		t.Errorf("IsPlaying = false; want true")
	}
}

func TestParseStatusHandlesCRLFOnSentinel(t *testing.T) {
	if _, err := parseStatus("NOT_RUNNING\r\n"); !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
	if _, err := parseStatus("NO_TRACK\r\n"); !errors.Is(err, music.ErrNoTrack) {
		t.Fatalf("err = %v; want ErrNoTrack", err)
	}
}

func TestParseAirPlayDevicesEmpty(t *testing.T) {
	got, err := parseAirPlayDevices("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParseAirPlayDevicesNotRunning(t *testing.T) {
	_, err := parseAirPlayDevices("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParseAirPlayDevicesSingle(t *testing.T) {
	raw := "Computer\tcomputer\ttrue\tfalse\ttrue\n"
	got, err := parseAirPlayDevices(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	want := domain.AudioDevice{Name: "Computer", Kind: "computer", Available: true, Active: false, Selected: true}
	if got[0] != want {
		t.Errorf("got = %+v; want %+v", got[0], want)
	}
}

func TestParseAirPlayDevicesMultiple(t *testing.T) {
	raw := "Computer\tcomputer\ttrue\tfalse\ttrue\n" +
		"Kitchen Sonos\tAirPlay\ttrue\tfalse\tfalse\n" +
		"Office\tAirPlay\tfalse\tfalse\tfalse"
	got, err := parseAirPlayDevices(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[0].Name != "Computer" || got[1].Name != "Kitchen Sonos" || got[2].Name != "Office" {
		t.Errorf("names = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	if got[2].Available {
		t.Errorf("Office.Available = true; want false")
	}
}

func TestParseAirPlayDevicesParsesBoolFields(t *testing.T) {
	raw := "X\tspeaker\tfalse\ttrue\tfalse\n"
	got, _ := parseAirPlayDevices(raw)
	if got[0].Available || !got[0].Active || got[0].Selected {
		t.Errorf("got = %+v", got[0])
	}
}

func TestParseAirPlayDevicesMalformedReturnsErrUnavailable(t *testing.T) {
	raw := "X\tspeaker\ttrue\n"
	_, err := parseAirPlayDevices(raw)
	if !errors.Is(err, music.ErrUnavailable) {
		t.Fatalf("err = %v; want ErrUnavailable", err)
	}
}

func TestMatchAirPlayDeviceExactWins(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Living Room"},
		{Name: "Living Room Speakers"},
	}
	got, err := matchAirPlayDevice(devices, "Living Room")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Living Room" {
		t.Errorf("got = %q; want exact 'Living Room'", got.Name)
	}
}

func TestMatchAirPlayDeviceCaseInsensitiveSubstring(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Mark's Mac mini"},
		{Name: "Kitchen Sonos"},
	}
	got, err := matchAirPlayDevice(devices, "kitchen")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Name != "Kitchen Sonos" {
		t.Errorf("got = %q; want Kitchen Sonos", got.Name)
	}
}

func TestMatchAirPlayDeviceNotFoundReturnsErrDeviceNotFound(t *testing.T) {
	devices := []domain.AudioDevice{{Name: "Computer"}}
	_, err := matchAirPlayDevice(devices, "Atlantis")
	if !errors.Is(err, music.ErrDeviceNotFound) {
		t.Fatalf("err = %v; want ErrDeviceNotFound", err)
	}
}

func TestMatchAirPlayDeviceAmbiguousReturnsErrAmbiguousDevice(t *testing.T) {
	devices := []domain.AudioDevice{
		{Name: "Kitchen Sonos"},
		{Name: "Office Sonos"},
	}
	_, err := matchAirPlayDevice(devices, "sonos")
	if !errors.Is(err, music.ErrAmbiguousDevice) {
		t.Fatalf("err = %v; want ErrAmbiguousDevice", err)
	}
}

func TestParsePlaylistsEmpty(t *testing.T) {
	got, err := parsePlaylists("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParsePlaylistsNotRunning(t *testing.T) {
	_, err := parsePlaylists("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParsePlaylistsSingle(t *testing.T) {
	raw := "Liked Songs\tuser\t42\n"
	got, err := parsePlaylists(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := domain.Playlist{Name: "Liked Songs", Kind: "user", TrackCount: 42}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got = %+v; want [%+v]", got, want)
	}
}

func TestParsePlaylistsMultiple(t *testing.T) {
	raw := "Liked Songs\tuser\t42\n" +
		"Workout\tsubscription\t12\n" +
		"90s Mix\tuser\t30"
	got, err := parsePlaylists(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[1].Kind != "subscription" || got[1].TrackCount != 12 {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestParsePlaylistsSkipsEmptyName(t *testing.T) {
	raw := "\tuser\t0\n" + "Liked Songs\tuser\t42\n"
	got, _ := parsePlaylists(raw)
	if len(got) != 1 || got[0].Name != "Liked Songs" {
		t.Errorf("got = %+v; expected empty-name row to be skipped", got)
	}
}

func TestParsePlaylistsSkipsMalformedRow(t *testing.T) {
	raw := "Bad Row\tuser\n" + "Liked Songs\tuser\t42\n"
	got, _ := parsePlaylists(raw)
	if len(got) != 1 || got[0].Name != "Liked Songs" {
		t.Errorf("got = %+v; expected malformed row to be skipped", got)
	}
}

func TestParsePlaylistTracksEmpty(t *testing.T) {
	got, err := parsePlaylistTracks("")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d; want 0", len(got))
	}
}

func TestParsePlaylistTracksNotRunning(t *testing.T) {
	_, err := parsePlaylistTracks("NOT_RUNNING\n")
	if !errors.Is(err, music.ErrNotRunning) {
		t.Fatalf("err = %v; want ErrNotRunning", err)
	}
}

func TestParsePlaylistTracksNotFound(t *testing.T) {
	_, err := parsePlaylistTracks("NOT_FOUND\n")
	if !errors.Is(err, music.ErrPlaylistNotFound) {
		t.Fatalf("err = %v; want ErrPlaylistNotFound", err)
	}
}

func TestParsePlaylistTracksSingle(t *testing.T) {
	raw := "Stairway to Heaven\tLed Zeppelin\tLed Zeppelin IV\t482\n"
	got, err := parsePlaylistTracks(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	want := domain.Track{
		Title:    "Stairway to Heaven",
		Artist:   "Led Zeppelin",
		Album:    "Led Zeppelin IV",
		Duration: 482 * time.Second,
	}
	if got[0] != want {
		t.Errorf("got[0] = %+v; want %+v", got[0], want)
	}
}

func TestParsePlaylistTracksMultiple(t *testing.T) {
	raw := "A\tArtist\tAlbum\t100\n" +
		"B\tArtist\tAlbum\t200\n" +
		"C\tArtist\tAlbum\t300"
	got, err := parsePlaylistTracks(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	if got[2].Duration != 300*time.Second {
		t.Errorf("got[2].Duration = %v; want 300s", got[2].Duration)
	}
}

func TestParsePlaylistTracksSkipsMalformedRow(t *testing.T) {
	raw := "BadRow\tArtist\tAlbum\n" + "Good\tArtist\tAlbum\t100\n"
	got, _ := parsePlaylistTracks(raw)
	if len(got) != 1 || got[0].Title != "Good" {
		t.Errorf("got = %+v; expected malformed row to be skipped", got)
	}
}
