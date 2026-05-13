package app

import (
	"context"
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
	"github.com/themoderngeek/goove/internal/music/fake"
)

func handoffModel(t *testing.T) Model {
	t.Helper()
	c := fake.New()
	_ = c.Launch(context.Background())
	c.SetPlaylists([]domain.Playlist{{Name: "LZ"}})
	c.SetPlaylistTracks("LZ", []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
		{Title: "Misty", PersistentID: "MM"},
	})
	c.SetLibraryTracks([]domain.Track{
		{Title: "Hotel California", PersistentID: "HC"},
		{Title: "Wonderwall", PersistentID: "WW"},
	})
	m := New(c, nil)
	// Pre-cache the playlist so indexOfPID works in handler tests.
	m.playlists.tracksByName["LZ"] = []domain.Track{
		{Title: "Black Dog", PersistentID: "BD"},
		{Title: "Stairway", PersistentID: "ST"},
		{Title: "Misty", PersistentID: "MM"},
	}
	return m
}

func TestHandoffNoTrackChange(t *testing.T) {
	m := handoffModel(t)
	m.lastTrackPID = "ST"
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "ST"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil on no-change")
	}
	if got.queue.Len() != 0 || got.resume.PlaylistName != "" {
		t.Errorf("state mutated on no-change: %+v", got)
	}
}

func TestHandoffEmptyQueueNoResumeIsNoDispatch(t *testing.T) {
	m := handoffModel(t)
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	_, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil")
	}
}

func TestHandoffEmptyQueueWithResumeDispatchesPlayPlaylist(t *testing.T) {
	m := handoffModel(t)
	m.resume = ResumeContext{PlaylistName: "LZ", NextIndex: 3}
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "HC"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayPlaylist Cmd")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume not cleared: %+v", got.resume)
	}
	// Invoke the cmd to confirm it produces a playPlaylistMsg and that the
	// fake client recorded the call with the right args.
	out := cmd()
	if _, ok := out.(playPlaylistMsg); !ok {
		t.Errorf("cmd result = %T; want playPlaylistMsg", out)
	}
	rec := m.client.(*fake.Client).PlayPlaylistRecord()
	if len(rec) != 1 || rec[0].Name != "LZ" || rec[0].FromIdx != 3 {
		t.Errorf("PlayPlaylist record = %v; want [{LZ 3}]", rec)
	}
}

func TestHandoffInterceptCapturesResumeAndPopsHead(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	// Previous tick: track ST (index 2) playing in LZ.
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3}", got.resume)
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0 (head popped)", got.queue.Len())
	}
	out := cmd()
	if _, ok := out.(playTrackResultMsg); !ok {
		t.Errorf("cmd result = %T; want playTrackResultMsg", out)
	}
	rec := m.client.(*fake.Client).PlayTrackRecord()
	if len(rec) != 1 || rec[0].PersistentID != "HC" {
		t.Errorf("PlayTrack record = %v; want [{HC}]", rec)
	}
}

func TestHandoffInterceptDoesNotOverwriteExistingResume(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	m.resume = ResumeContext{PlaylistName: "Other", NextIndex: 5}
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "Other" || got.resume.NextIndex != 5 {
		t.Errorf("resume overwritten: %+v; want {Other 5}", got.resume)
	}
}

func TestHandoffInterceptWithEmptyPrevPlaylistLeavesResumeEmpty(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "MM"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "ST", "", 0)
	if cmd == nil {
		t.Fatal("cmd == nil; want PlayTrack Cmd")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured with no valid prev context: %+v", got.resume)
	}
}

func TestHandoffNewPIDMatchesQueueHeadPopsOnly(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "Hotel California", PersistentID: "HC"})
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "HC"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil (pop-on-match, no dispatch)")
	}
	if got.queue.Len() != 0 {
		t.Errorf("queue.Len = %d; want 0", got.queue.Len())
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured on pop-on-match: %+v", got.resume)
	}
}

func TestHandoffNewPIDMatchesPendingJumpClearsFlag(t *testing.T) {
	m := handoffModel(t)
	m.queue.Add(domain.Track{Title: "A", PersistentID: "A1"})
	m.pendingJumpPID = "C1"
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "C1"}, CurrentPlaylistName: ""}
	got, cmd := m.handleQueueHandoff(now, "ST", "LZ", 2)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil (pending jump match)")
	}
	if got.pendingJumpPID != "" {
		t.Errorf("pendingJumpPID = %q; want cleared", got.pendingJumpPID)
	}
	if got.queue.Len() != 1 {
		t.Errorf("queue.Len = %d; want 1 (not popped on jump)", got.queue.Len())
	}
	if got.resume.PlaylistName != "LZ" || got.resume.NextIndex != 3 {
		t.Errorf("resume = %+v; want {LZ 3} (captured on jump)", got.resume)
	}
}

func TestHandoffFirstTickIsNoOpWithEmptyPrev(t *testing.T) {
	m := handoffModel(t)
	// Empty queue, empty prev — typical launch state.
	now := domain.NowPlaying{Track: domain.Track{PersistentID: "ST"}, CurrentPlaylistName: "LZ"}
	got, cmd := m.handleQueueHandoff(now, "", "", 0)
	if cmd != nil {
		t.Errorf("cmd != nil; want nil on first tick with empty queue")
	}
	if got.resume.PlaylistName != "" {
		t.Errorf("resume captured on first tick: %+v", got.resume)
	}
}

func TestIndexOfPIDFindsTrack(t *testing.T) {
	m := handoffModel(t)
	if got := m.indexOfPID("ST", "LZ"); got != 2 {
		t.Errorf("indexOfPID(ST, LZ) = %d; want 2", got)
	}
	if got := m.indexOfPID("BD", "LZ"); got != 1 {
		t.Errorf("indexOfPID(BD, LZ) = %d; want 1", got)
	}
}

func TestIndexOfPIDReturnsZeroOnMiss(t *testing.T) {
	m := handoffModel(t)
	if got := m.indexOfPID("XX", "LZ"); got != 0 {
		t.Errorf("indexOfPID(miss) = %d; want 0", got)
	}
	if got := m.indexOfPID("ST", "Unknown"); got != 0 {
		t.Errorf("indexOfPID(unknown playlist) = %d; want 0", got)
	}
	if got := m.indexOfPID("", "LZ"); got != 0 {
		t.Errorf("indexOfPID(empty pid) = %d; want 0", got)
	}
}
