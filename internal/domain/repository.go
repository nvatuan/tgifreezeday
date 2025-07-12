package domain

import (
	"time"
)

type TGIFCalendarRepository interface {
	GetFreezeDaysInRange(dateAnchor time.Time) (*TGIFMapping, error)
	WriteBlockerOnDate(date time.Time, summary string) error
}
