package app

import (
	"strings"
	"testing"
)

func TestHintBarAlwaysContainsGlobals(t *testing.T) {
	for _, f := range []focusKind{focusPlaylists, focusSearch, focusOutput, focusMain} {
		got := renderHintBar(Model{focus: f})
		for _, want := range []string{"space", "n", "p", "q"} {
			if !strings.Contains(got, want) {
				t.Errorf("focus=%v: hint bar %q missing global %q", f, got, want)
			}
		}
	}
}

func TestHintBarContainsPanelKeysForPlaylists(t *testing.T) {
	got := renderHintBar(Model{focus: focusPlaylists})
	if !strings.Contains(got, "j/k") {
		t.Errorf("hint bar for Playlists missing j/k: %q", got)
	}
	if !strings.Contains(got, "play") {
		t.Errorf("hint bar for Playlists missing play hint: %q", got)
	}
}

func TestHintBarContainsPanelKeysForSearchInIdle(t *testing.T) {
	got := renderHintBar(Model{focus: focusSearch})
	if !strings.Contains(got, "type to search") {
		t.Errorf("hint bar for Search idle missing 'type to search': %q", got)
	}
}

func TestHintBarContainsPanelKeysForOutput(t *testing.T) {
	got := renderHintBar(Model{focus: focusOutput})
	if !strings.Contains(got, "switch") {
		t.Errorf("hint bar for Output missing 'switch': %q", got)
	}
}
