package scheduler

import (
	"testing"
	"time"
)

func jstTime(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, jst)
}

func TestNextSyncAt_Weekly(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{
			name: "from Sunday → next Monday 09:00 JST",
			from: jstTime(2026, time.May, 10, 12, 0), // Sunday
			want: jstTime(2026, time.May, 11, 9, 0),  // Monday
		},
		{
			name: "from Monday before 09:00 → same Monday 09:00",
			from: jstTime(2026, time.May, 11, 8, 0),
			want: jstTime(2026, time.May, 11, 9, 0),
		},
		{
			name: "from Monday exactly 09:00 → next Monday 09:00",
			from: jstTime(2026, time.May, 11, 9, 0),
			want: jstTime(2026, time.May, 18, 9, 0),
		},
		{
			name: "from Monday after 09:00 → next Monday 09:00",
			from: jstTime(2026, time.May, 11, 10, 0),
			want: jstTime(2026, time.May, 18, 9, 0),
		},
		{
			name: "from Friday → next Monday 09:00",
			from: jstTime(2026, time.May, 8, 14, 30), // Friday
			want: jstTime(2026, time.May, 11, 9, 0),  // Monday
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NextSyncAt("weekly", tc.from)
			if !got.Equal(tc.want.UTC()) {
				t.Errorf("NextSyncAt(weekly, %v) = %v, want %v", tc.from, got.In(jst), tc.want.In(jst))
			}
			if !got.After(tc.from) {
				t.Errorf("result %v is not strictly after from %v", got, tc.from)
			}
		})
	}
}

func TestNextSyncAt_Monthly(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{
			name: "from mid-month → 1st of next month 09:00",
			from: jstTime(2026, time.May, 15, 12, 0),
			want: jstTime(2026, time.June, 1, 9, 0),
		},
		{
			name: "from 1st before 09:00 → same day 09:00",
			from: jstTime(2026, time.May, 1, 8, 0),
			want: jstTime(2026, time.May, 1, 9, 0),
		},
		{
			name: "from 1st exactly 09:00 → 1st of next month",
			from: jstTime(2026, time.May, 1, 9, 0),
			want: jstTime(2026, time.June, 1, 9, 0),
		},
		{
			name: "from 1st after 09:00 → 1st of next month",
			from: jstTime(2026, time.May, 1, 10, 0),
			want: jstTime(2026, time.June, 1, 9, 0),
		},
		{
			name: "from December → January 1st of next year",
			from: jstTime(2026, time.December, 20, 12, 0),
			want: jstTime(2027, time.January, 1, 9, 0),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NextSyncAt("monthly", tc.from)
			if !got.Equal(tc.want.UTC()) {
				t.Errorf("NextSyncAt(monthly, %v) = %v, want %v", tc.from, got.In(jst), tc.want.In(jst))
			}
			if !got.After(tc.from) {
				t.Errorf("result %v is not strictly after from %v", got, tc.from)
			}
		})
	}
}

func TestNextSyncAt_None(t *testing.T) {
	got := NextSyncAt("none", time.Now())
	if !got.IsZero() {
		t.Errorf("expected zero time for 'none' schedule, got %v", got)
	}
}
