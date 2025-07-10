package helpers

import (
	"testing"
	"time"
)

// test if DaysInMonth is returning the correct number of days in a month across years
func Test_DaysInMonth(t *testing.T) {
	tests := []struct {
		dateAnchor time.Time
		expected   int
	}{
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 29},
		{time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), 30},
		{time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), 30},
		{time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), 30},
		{time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC), 31},
		{time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC), 30},
		{time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC), 31},
		// non leap year
		{time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), 28},
		// leap year in far future
		{time.Date(2100, 2, 1, 0, 0, 0, 0, time.UTC), 28},
		{time.Date(2200, 2, 1, 0, 0, 0, 0, time.UTC), 28},
		{time.Date(2400, 2, 1, 0, 0, 0, 0, time.UTC), 29},
	}

	for _, test := range tests {
		actual := DaysInMonth(test.dateAnchor)
		if actual != test.expected {
			t.Errorf("daysInMonth(%v) = %d, expected %d", test.dateAnchor, actual, test.expected)
		}
	}
}

// test if DateKey is returning the correct key for a date
func Test_DateKey(t *testing.T) {
	tests := []struct {
		date     time.Time
		expected string
	}{
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "2024-01-01"},
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "2024-01-15"},
		{time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), "2024-12-31"},
	}

	for _, test := range tests {
		actual := DateKey(test.date)
		if actual != test.expected {
			t.Errorf("DateKey(%v) = %s, expected %s", test.date, actual, test.expected)
		}
	}
}
