package rules

import (
	"fmt"
	"time"
)

// DayBeforeHolidayRule represents a rule that freezes production the day before a holiday
type DayBeforeHolidayRule struct {
	holidays map[time.Time]string
	days     int
}

// NewDayBeforeHolidayRule creates a new rule for the days before holidays
func NewDayBeforeHolidayRule(days int) *DayBeforeHolidayRule {
	return &DayBeforeHolidayRule{
		holidays: make(map[time.Time]string),
		days:     days,
	}
}

// AddHoliday adds a holiday to the rule
func (r *DayBeforeHolidayRule) AddHoliday(date time.Time, name string) {
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	r.holidays[normalizedDate] = name
}

// Apply checks if the given date is within the freeze period before a holiday
func (r *DayBeforeHolidayRule) Apply(t time.Time) bool {
	checkDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	for holiday := range r.holidays {
		for i := 1; i <= r.days; i++ {
			freezeDay := holiday.AddDate(0, 0, -i)
			if freezeDay.Equal(checkDate) {
				return true
			}
		}
	}

	return false
}

// Description returns a description of the rule
func (r *DayBeforeHolidayRule) Description() string {
	return fmt.Sprintf("Freeze %d day(s) before holidays", r.days)
}

// DayAfterHolidayRule represents a rule that freezes production after a holiday
type DayAfterHolidayRule struct {
	holidays map[time.Time]string
	days     int
}

// NewDayAfterHolidayRule creates a new rule for the days after holidays
func NewDayAfterHolidayRule(days int) *DayAfterHolidayRule {
	return &DayAfterHolidayRule{
		holidays: make(map[time.Time]string),
		days:     days,
	}
}

// AddHoliday adds a holiday to the rule
func (r *DayAfterHolidayRule) AddHoliday(date time.Time, name string) {
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	r.holidays[normalizedDate] = name
}

// Apply checks if the given date is within the freeze period after a holiday
func (r *DayAfterHolidayRule) Apply(t time.Time) bool {
	checkDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	for holiday := range r.holidays {
		for i := 1; i <= r.days; i++ {
			freezeDay := holiday.AddDate(0, 0, i)
			if freezeDay.Equal(checkDate) {
				return true
			}
		}
	}

	return false
}

// Description returns a description of the rule
func (r *DayAfterHolidayRule) Description() string {
	return fmt.Sprintf("Freeze %d day(s) after holidays", r.days)
}

// WeekendRule represents a rule that identifies weekends
type WeekendRule struct{}

// NewWeekendRule creates a new weekend rule
func NewWeekendRule() *WeekendRule {
	return &WeekendRule{}
}

// Apply checks if the given date is a weekend
func (r *WeekendRule) Apply(t time.Time) bool {
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

// Description returns a description of the rule
func (r *WeekendRule) Description() string {
	return "Weekend freeze day"
}
