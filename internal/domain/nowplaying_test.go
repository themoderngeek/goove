package domain

import (
	"testing"
	"time"
)

func TestDisplayedPosition(t *testing.T) {
	t0 := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		np   NowPlaying
		now  time.Time
		want time.Duration
	}{
		{
			name: "paused does not advance",
			np: NowPlaying{
				Position:     30 * time.Second,
				Duration:     180 * time.Second,
				IsPlaying:    false,
				LastSyncedAt: t0,
			},
			now:  t0.Add(5 * time.Second),
			want: 30 * time.Second,
		},
		{
			name: "playing advances by elapsed wall clock",
			np: NowPlaying{
				Position:     30 * time.Second,
				Duration:     180 * time.Second,
				IsPlaying:    true,
				LastSyncedAt: t0,
			},
			now:  t0.Add(7 * time.Second),
			want: 37 * time.Second,
		},
		{
			name: "playing clamps to duration",
			np: NowPlaying{
				Position:     170 * time.Second,
				Duration:     180 * time.Second,
				IsPlaying:    true,
				LastSyncedAt: t0,
			},
			now:  t0.Add(30 * time.Second),
			want: 180 * time.Second,
		},
		{
			name: "now before LastSyncedAt does not regress position",
			np: NowPlaying{
				Position:     30 * time.Second,
				Duration:     180 * time.Second,
				IsPlaying:    true,
				LastSyncedAt: t0,
			},
			now:  t0.Add(-5 * time.Second),
			want: 30 * time.Second,
		},
		{
			name: "zero duration returns position unchanged",
			np: NowPlaying{
				Position:     0,
				Duration:     0,
				IsPlaying:    true,
				LastSyncedAt: t0,
			},
			now:  t0.Add(3 * time.Second),
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.np.DisplayedPosition(tc.now)
			if got != tc.want {
				t.Fatalf("DisplayedPosition(%v) = %v; want %v", tc.now, got, tc.want)
			}
		})
	}
}
