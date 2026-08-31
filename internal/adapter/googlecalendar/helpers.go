package googlecalendar

import (
	"fmt"

	"github.com/nvat/tgifreezeday/internal/consts"
	"google.golang.org/api/calendar/v3"
)

var countryToCalendarID = map[string]string{
	"jpn": "ja.japanese#holiday@group.v.calendar.google.com",
	"vnm": "vi.vietnamese#holiday@group.v.calendar.google.com",
}

// calendarPublicHolidayDesc maps each localized Google holiday calendar to the exact
// event description it uses to mark an actual public (non-working) holiday.
//
// The localized calendars describe holidays in their own language, NOT the English
// "Public holiday" string. Observance/festival days use a different description
// (e.g. "祭日…", "Ngày lễ kỷ niệm…") and are intentionally excluded — they are still
// working days. Any description that is not the exact public-holiday marker below is
// treated as a non-holiday.
var calendarPublicHolidayDesc = map[string]string{
	"ja.japanese#holiday@group.v.calendar.google.com":   "祝日",
	"vi.vietnamese#holiday@group.v.calendar.google.com": "Ngày lễ",
}

func GetHolidayCalendarID(country string) (string, error) {
	v, ok := countryToCalendarID[country]
	if !ok {
		return "", fmt.Errorf("country %s is not supported. Supported countries: %v", country, consts.SupportedCountries)
	}
	return v, nil
}

// isPublicHoliday reports whether an event represents an actual public (non-working)
// holiday on the repository's read calendar, as opposed to an observance/festival day.
//
// Recognition is locale-aware: each supported holiday calendar has its own
// public-holiday description string (see calendarPublicHolidayDesc). Only an exact
// match counts as a holiday; everything else (observances, empty descriptions,
// unknown calendars) is treated as a non-holiday.
func (r *Repository) isPublicHoliday(event *calendar.Event) bool {
	want, ok := calendarPublicHolidayDesc[r.readCalendarID]
	if !ok {
		return false
	}
	return event.Description == want
}
