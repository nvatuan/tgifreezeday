package handler

import "time"

func dateRange(lookbackDays, lookaheadDays int) (time.Time, time.Time) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -lookbackDays), today.AddDate(0, 0, lookaheadDays)
}
