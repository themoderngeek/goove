package domain

import (
	"sort"
	"strings"
)

// RankSearchResults orders OR-matched tracks by which field the query
// matched: title-matches first, then artist, then album. Within each group
// it sorts alphabetically by lowercased title (stable). Tracks that don't
// match anywhere — defensive; shouldn't happen for an OR-matched input — sort
// last.
//
// Match is case-insensitive substring, mirroring AppleScript's `whose name
// contains` clause.
func RankSearchResults(tracks []Track, query string) []Track {
	if len(tracks) == 0 {
		return tracks
	}
	q := strings.ToLower(query)
	var groups [4][]Track
	for _, t := range tracks {
		switch {
		case strings.Contains(strings.ToLower(t.Title), q):
			groups[0] = append(groups[0], t)
		case strings.Contains(strings.ToLower(t.Artist), q):
			groups[1] = append(groups[1], t)
		case strings.Contains(strings.ToLower(t.Album), q):
			groups[2] = append(groups[2], t)
		default:
			groups[3] = append(groups[3], t)
		}
	}
	for i := range groups {
		g := groups[i]
		sort.SliceStable(g, func(a, b int) bool {
			return strings.ToLower(g[a].Title) < strings.ToLower(g[b].Title)
		})
	}
	out := make([]Track, 0, len(tracks))
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
