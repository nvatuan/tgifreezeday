package domain

import "time"

type CalendarRepository interface {
	GetMonthCalendar(dateAnchor time.Time) (*MonthCalendar, error)
}
