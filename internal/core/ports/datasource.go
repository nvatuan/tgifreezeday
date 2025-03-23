package ports

import (
	"time"
)

// DataSource defines the interface for retrieving calendar data
type DataSource interface {
	// These methods use Holiday Calendar
	// IsFirstBusinessDayOfTheMonth checks if a specific date is the first business day of the month
	IsFirstBusinessDayOfTheMonth(date time.Time) (bool, error)

	// IsLastBusinessDayOfTheMonth checks if a specific date is the last business day of the month
	IsLastBusinessDayOfTheMonth(date time.Time) (bool, error)

	// IsWeekend checks if a specific date is a weekend (Saturday or Sunday)
	IsWeekend(date time.Time) (bool, error)

	// IsHoliday checks if a specific date is a holiday
	IsHoliday(date time.Time) (bool, error)

	// IsBusinessDay checks if a specific date is a business day
	IsBusinessDay(date time.Time) (bool, error)

	// These methods use Calendar (not the Holiday Calendar)
	// HasEvent checks if a specific date has an event with a specific keyword
	HasEvent(date time.Time, keyword string) (bool, error)
}
