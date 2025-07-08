package domain

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
		actual := daysInMonth(test.dateAnchor)
		if actual != test.expected {
			t.Errorf("daysInMonth(%v) = %d, expected %d", test.dateAnchor, actual, test.expected)
		}
	}
}

// mock data for testing
var (
	mockDateAnchor    = time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	mockMonthDays     = []MonthDay{}
	mockMonthCalendar = MonthCalendar{}
)

func init() {
	mockMonthDays = make([]MonthDay, 0, daysInMonth(mockDateAnchor)) // start with 0 length, capacity for all days
	for i := 1; i <= daysInMonth(mockDateAnchor); i++ {
		mockMonthDays = append(
			mockMonthDays,
			*NewMonthDay(
				time.Date(mockDateAnchor.Year(), mockDateAnchor.Month(), i, 0, 0, 0, 0, mockDateAnchor.Location()),
				false)) // isHoliday only available if Calendar API is called, for now we'll just set it to false
	}
	mockMonthCalendar = *NewMonthCalendar(mockDateAnchor, mockMonthDays)
}

// test Validate: fail case - not enough days
func Test_MonthCalendar_Validate_Fail_NotEnoughDays(t *testing.T) {
	tmpMonthCalendar := MonthCalendar{
		Month: mockMonthCalendar.Month,
		Days:  make([]MonthDay, daysInMonth(mockDateAnchor)-1),
	}
	for i := 0; i < len(tmpMonthCalendar.Days)-1; i++ {
		tmpMonthCalendar.Days[i] = mockMonthCalendar.Days[i]
	}

	valid, err := tmpMonthCalendar.Validate()
	if valid {
		t.Errorf("Validate() = %v, expected %v", valid, false)
	}
	if err == nil {
		t.Errorf("Validate() = %v, expected %v", err, "expected 31 days, got 30")
	}
}

// test Validate: fail case - not in chronological order
func Test_MonthCalendar_Validate_Fail_NotInChronologicalOrder(t *testing.T) {
	tmpMonthCalendar := MonthCalendar{
		Month: mockMonthCalendar.Month,
		Days:  make([]MonthDay, daysInMonth(mockDateAnchor)),
	}
	for i := 0; i < len(tmpMonthCalendar.Days); i++ {
		tmpMonthCalendar.Days[i] = mockMonthCalendar.Days[i]
	}
	// swap the 14th and 15th day
	tmpMonthCalendar.Days[13], tmpMonthCalendar.Days[14] = tmpMonthCalendar.Days[14], tmpMonthCalendar.Days[13]

	valid, err := tmpMonthCalendar.Validate()
	if valid {
		t.Errorf("Validate() = %v, expected %v", valid, false)
	}
	if err == nil {
		t.Errorf("Validate() = %v, expected %v", err, "days passed to the calendar must follow chronological order and starts at 1, got 15, expected 14")
	}
}

// test Validate: success case
func Test_MonthCalendar_Validate_Success(t *testing.T) {
	tmpMonthCalendar := MonthCalendar{
		Month: mockMonthCalendar.Month,
		Days:  mockMonthCalendar.Days,
	}

	valid, err := tmpMonthCalendar.Validate()
	if !valid {
		t.Errorf("Validate() = %v, expected %v", valid, true)
	}
	if err != nil {
		t.Errorf("Validate() = %v, expected %v", err, nil)
	}
}
