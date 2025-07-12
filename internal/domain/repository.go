package domain

import (
	"time"
)

type TGIFCalendarRepository interface {
	GetMonthCalendar(dateAnchor time.Time) (*MonthCalendar, error)
	WriteBlockerOnDate(date time.Time, summary string) error
}
