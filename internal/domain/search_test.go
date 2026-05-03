package domain

import (
	"reflect"
	"testing"
)

func TestRankSearchResults_GroupsByMatchSource(t *testing.T) {
	tracks := []Track{
		{Title: "Album Match Only", Artist: "Other", Album: "Stairway"},
		{Title: "Bumble", Artist: "Stairway Band", Album: "X"},
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV"},
		{Title: "Another Stairway", Artist: "Y", Album: "Z"},
	}
	got := RankSearchResults(tracks, "stair")
	want := []Track{
		// Title-match group, alphabetical by title.
		{Title: "Another Stairway", Artist: "Y", Album: "Z"},
		{Title: "Stairway to Heaven", Artist: "Led Zeppelin", Album: "IV"},
		// Artist-match group.
		{Title: "Bumble", Artist: "Stairway Band", Album: "X"},
		// Album-match group.
		{Title: "Album Match Only", Artist: "Other", Album: "Stairway"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordering wrong\n got: %v\nwant: %v", got, want)
	}
}

func TestRankSearchResults_CaseInsensitive(t *testing.T) {
	tracks := []Track{
		{Title: "STAIRWAY", Artist: "X", Album: "Y"},
		{Title: "lower", Artist: "STAIR-something", Album: "Y"},
	}
	got := RankSearchResults(tracks, "stair")
	want := []Track{
		{Title: "STAIRWAY", Artist: "X", Album: "Y"},
		{Title: "lower", Artist: "STAIR-something", Album: "Y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("case-insensitive failed\n got: %v\nwant: %v", got, want)
	}
}

func TestRankSearchResults_StableWithinGroup(t *testing.T) {
	// Two tracks with the same lowercase title sort by their input order
	// (stable sort) — neither should be reordered.
	tracks := []Track{
		{Title: "Stair", Artist: "Aaa"},
		{Title: "stair", Artist: "Bbb"},
	}
	got := RankSearchResults(tracks, "stair")
	if got[0].Artist != "Aaa" || got[1].Artist != "Bbb" {
		t.Errorf("stable order broken: got %v", got)
	}
}

func TestRankSearchResults_EmptyInputs(t *testing.T) {
	if got := RankSearchResults(nil, "x"); got != nil {
		t.Errorf("expected nil for nil tracks, got %v", got)
	}
	if got := RankSearchResults([]Track{}, "x"); len(got) != 0 {
		t.Errorf("expected empty for empty tracks, got %v", got)
	}
}
