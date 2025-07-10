package helpers

import "time"

func DaysInMonth(dateAnchor time.Time) int {
	return time.Date(dateAnchor.Year(), dateAnchor.Month()+1, 0, 0, 0, 0, 0, dateAnchor.Location()).Day()
}

func DateKey(date time.Time) string {
	return date.Format("2006-01-02")
}
