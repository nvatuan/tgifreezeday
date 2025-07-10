package domain

import (
	"testing"
	"time"

	"github.com/nvat/tgifreezeday/internal/helpers"
)

// mock data for testing
var (
	mockDateAnchor    = time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	mockMonthDays     = []MonthDay{}
	mockMonthCalendar = MonthCalendar{}
)

func init() {
	mockMonthDays = make([]MonthDay, 0, helpers.DaysInMonth(mockDateAnchor)) // start with 0 length, capacity for all days
	for i := 1; i <= helpers.DaysInMonth(mockDateAnchor); i++ {
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
		Days:  make([]MonthDay, helpers.DaysInMonth(mockDateAnchor)-1),
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
		Days:  make([]MonthDay, helpers.DaysInMonth(mockDateAnchor)),
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
