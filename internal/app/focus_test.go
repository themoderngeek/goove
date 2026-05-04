package app

import "testing"

func TestNextFocusCyclesForward(t *testing.T) {
	tests := []struct {
		from, want focus
	}{
		{focusPlaylists, focusSearch},
		{focusSearch, focusOutput},
		{focusOutput, focusMain},
		{focusMain, focusPlaylists},
	}
	for _, tt := range tests {
		got := nextFocus(tt.from)
		if got != tt.want {
			t.Errorf("nextFocus(%v) = %v; want %v", tt.from, got, tt.want)
		}
	}
}

func TestPrevFocusCyclesBackward(t *testing.T) {
	tests := []struct {
		from, want focus
	}{
		{focusPlaylists, focusMain},
		{focusSearch, focusPlaylists},
		{focusOutput, focusSearch},
		{focusMain, focusOutput},
	}
	for _, tt := range tests {
		got := prevFocus(tt.from)
		if got != tt.want {
			t.Errorf("prevFocus(%v) = %v; want %v", tt.from, got, tt.want)
		}
	}
}
