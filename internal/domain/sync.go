package domain

import (
	"fmt"
	"time"
)

// RunSync wipes all managed blocker events in [rangeStart, rangeEnd] and rewrites
// them based on the freeze-day rules. It is the shared business logic for both
// manual sync (HTTP handler) and scheduled auto-sync (background worker).
// Returns a human-readable result message and whether it was an error.
func RunSync(
	repo TGIFCalendarRepository,
	rangeStart, rangeEnd time.Time,
	rules TodayIsFreezeDayIf,
	summary, description, startTime, endTime string,
) (string, bool) {
	tgifMapping, err := repo.GetFreezeDaysInRange(rangeStart, rangeEnd)
	if err != nil {
		return "failed to get freeze days: " + err.Error(), true
	}
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		return "failed to wipe existing blockers: " + err.Error(), true
	}
	count := 0
	for _, day := range *tgifMapping {
		if day.IsTodayFreezeDay(rules) {
			if err := repo.WriteBlockerOnDate(day.Date, summary, description, startTime, endTime); err != nil {
				return fmt.Sprintf("failed to write blocker on %s: %s", day.Date.Format("2006-01-02"), err.Error()), true
			}
			count++
		}
	}
	return fmt.Sprintf("Sync complete. Created %d blocker event(s) across %d days checked.", count, len(*tgifMapping)), false
}
