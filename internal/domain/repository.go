package domain

import (
	"time"
)

type TGIFReadCalendarRepository interface {
	GetMonthCalendar(dateAnchor time.Time) (*TGIFMonthCalendar, error)
}

type TGIFWriteCalendarRepository interface {
	WriteBlockerOnDate(date time.Time, summary string) error
}
