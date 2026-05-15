package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/themoderngeek/goove/internal/domain"
)

// ResumeContext records the playlist and 1-based next-track index that
// handleQueueHandoff should hand control back to when the queue drains.
// Zero value = no resume target (drain ends in silence).
type ResumeContext struct {
	PlaylistName string
	NextIndex    int
}

// handleQueueHandoff runs once per status tick (after the existing
// artwork / playlist-prefetch logic in handleStatus). It compares the
// current tick's now-playing PID against lastTrackPID to detect track
// changes, then routes to one of four branches:
//
//   - No track change: return immediately, no mutation.
//   - newPID == pendingJumpPID: clear the flag, capture resume context
//     if valid and unset, return (don't pop the queue, don't dispatch).
//   - newPID == queue.Items[0].PersistentID: a previous tick's
//     intercept has landed; pop the head and return (no dispatch).
//   - Otherwise with non-empty queue: capture resume context if valid
//     and unset, dispatch PlayTrack(head) and pop. (Intercept.)
//   - Otherwise with empty queue and resume set: dispatch
//     PlayPlaylist(resume) and clear resume. (Drain.)
//
// prevPID / prevPlaylist / prevIdx are the *previous* tick's cached
// values, captured by the caller before refreshing m.lastTrackPID /
// m.lastPlaylist / m.lastTrackIdx for the next round.
func (m Model) handleQueueHandoff(now domain.NowPlaying, prevPID, prevPlaylist string, prevIdx int) (Model, tea.Cmd) {
	newPID := now.Track.PersistentID
	if newPID == prevPID {
		return m, nil
	}

	// Pending-jump match: overlay Enter dispatched a PlayTrack; recognise
	// our own transition without popping or re-intercepting.
	if newPID != "" && newPID == m.pendingJumpPID {
		m.pendingJumpPID = ""
		if m.resume.PlaylistName == "" && prevPlaylist != "" && prevIdx > 0 {
			m.resume = ResumeContext{PlaylistName: prevPlaylist, NextIndex: prevIdx + 1}
		}
		return m, nil
	}

	if m.queue.Len() == 0 {
		// Drain: hand back to interrupted playlist if we have one.
		if m.resume.PlaylistName != "" {
			cmd := playPlaylist(m.client, m.resume.PlaylistName, m.resume.NextIndex)
			m.resume = ResumeContext{}
			return m, cmd
		}
		return m, nil
	}

	head := m.queue.Items[0]
	if newPID != "" && newPID == head.PersistentID {
		// Our previous intercept has landed; pop and move on.
		m.queue.PopHead()
		return m, nil
	}

	// Intercept: capture resume (if empty and previous context valid)
	// and dispatch PlayTrack on the head.
	if m.resume.PlaylistName == "" && prevPlaylist != "" && prevIdx > 0 {
		m.resume = ResumeContext{PlaylistName: prevPlaylist, NextIndex: prevIdx + 1}
	}
	m.queue.PopHead()
	return m, playTrack(m.client, head.PersistentID)
}

// indexOfPID returns the 1-based index of pid inside the cached track
// list for playlistName, or 0 if pid is empty, the playlist isn't
// cached, or the PID isn't in the list. The 1-based convention matches
// PlayPlaylist's fromTrackIndex argument.
func (m Model) indexOfPID(pid, playlistName string) int {
	if pid == "" || playlistName == "" {
		return 0
	}
	tracks, ok := m.playlists.tracksByName[playlistName]
	if !ok {
		return 0
	}
	for i, t := range tracks {
		if t.PersistentID == pid {
			return i + 1
		}
	}
	return 0
}
