package domain

import "testing"

func TestPlaylistZeroValue(t *testing.T) {
	var p Playlist
	if p.Name != "" || p.Kind != "" || p.TrackCount != 0 {
		t.Errorf("zero-value Playlist = %+v; want empty", p)
	}
}

func TestPlaylistFieldsAssignable(t *testing.T) {
	p := Playlist{Name: "Liked Songs", Kind: "user", TrackCount: 42}
	if p.Name != "Liked Songs" || p.Kind != "user" || p.TrackCount != 42 {
		t.Errorf("got %+v", p)
	}
}
