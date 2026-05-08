package domain

import (
	"time"
)

type TGIFCalendarRepository interface {
	GetFreezeDaysInRange(rangeStart, rangeEnd time.Time) (*TGIFMapping, error)
	WipeAllBlockersInRange(startDate, endDate time.Time) error
	WriteBlockerOnDate(date time.Time, summary, description string) error
}
