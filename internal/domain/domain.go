package domain

import (
	"fmt"
	"time"

	"github.com/nvat/tgifreezeday/internal/helpers"
)

// Domain Calendar object. Should be mapped from any Calendar source to this.
// We only care about which day:
// - is holiday or not
// - is weekends or not
// For ease of calculation, a Calendar should have all days of the month it is representing
type TGIFMonthCalendar struct {
	// The time.Time object that this Calendar is anchored to. The Month information is the only important information, but a time.Time object is used for ease of calculation.
	dateAnchor  time.Time
	isProcessed bool

	Month time.Month     // The month of the calendar.
	Days  []TGIFMonthDay // Should be 28,29,30,31 days depending on the month.

	// will be set after MonthCalendar is processed
	FirstBusinessDay *TGIFMonthDay
	LastBusinessDay  *TGIFMonthDay
}

func NewMonthCalendar(
	dateAnchor time.Time,
	days []TGIFMonthDay,
) *TGIFMonthCalendar {
	return &TGIFMonthCalendar{
		dateAnchor: dateAnchor,
		Month:      dateAnchor.Month(),
		Days:       days,
	}
}

func (c *TGIFMonthCalendar) Validate() (bool, error) {
	expectedDaysInMonth := helpers.DaysInMonth(c.dateAnchor)

	if len(c.Days) != expectedDaysInMonth {
		return false, fmt.Errorf("for month %s, expected %d days, got %d", c.Month, expectedDaysInMonth, len(c.Days))
	}

	for i, day := range c.Days {
		if day.Date.Day() != i+1 {
			return false, fmt.Errorf("days passed to the calendar must follow chronological order and starts at 1, got %d, expected %d", day.Date.Day(), i+1)
		}
	}

	return true, nil
}

// Process() calculate the necessary info after filling the MonthCalendar.Days field
// For now, this sets the FirstBusinessDay and LastBusinessDay fields of the MonthCalendar.
func (c *TGIFMonthCalendar) Process() error {
	if c.isProcessed {
		return nil
	}

	if valid, err := c.Validate(); !valid {
		return err
	}

	for i, day := range c.Days {
		if day.IsBusinessDay {
			if c.FirstBusinessDay == nil {
				c.FirstBusinessDay = &c.Days[i]
			}
			c.LastBusinessDay = &c.Days[i]
		}
	}

	c.isProcessed = true
	return nil
}

type TGIFMonthDay struct {
	Date time.Time // The date of the day.

	// Initially nil, will be set after MonthCalendar is populated
	IsHoliday        bool
	IsWeekend        bool
	IsBusinessDay    bool
	IsNonBusinessDay bool
}

func NewTGIFMonthDay(date time.Time, isHoliday bool) *TGIFMonthDay {
	isWeekend := (date.Weekday() == time.Saturday || date.Weekday() == time.Sunday)

	day := &TGIFMonthDay{
		Date:             date,
		IsWeekend:        isWeekend,
		IsHoliday:        isHoliday,
		IsBusinessDay:    !isWeekend && !isHoliday,
		IsNonBusinessDay: isWeekend || isHoliday,
	}
	return day
}
