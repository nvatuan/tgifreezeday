package domain

import (
	"time"
)

// Event represents a calendar event with freeze day information
type Event struct {
	ID          string
	Title       string
	Description string
	StartTime   time.Time
	EndTime     time.Time
	IsHoliday   bool
}

// FreezePeriod represents a period when production changes are frozen
type FreezePeriod struct {
	StartDate       time.Time
	EndDate         time.Time
	Description     string
	RelatedHolidays []Event
}

// Rule defines a condition for determining freeze days
type Rule interface {
	Apply(time.Time) bool
	Description() string
}
