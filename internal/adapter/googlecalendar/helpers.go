package googlecalendar

import (
	"fmt"
	"strings"

	"github.com/nvat/tgifreezeday/internal/consts"
	"google.golang.org/api/calendar/v3"
)

// enPublicHoliday is the description Google uses on the English-locale holiday
// calendars. It is accepted as a fallback on every calendar in case a localized
// feed is served untranslated (i.e. Google returns English instead of the local
// language). The English observance string ("Observance") is deliberately not
// accepted — observances are still working days.
const enPublicHoliday = "Public holiday"

// holidayCalendar describes a supported country's Google holiday calendar.
type holidayCalendar struct {
	// id is the Google Calendar ID of the country's holiday calendar.
	id string
	// publicHolidayDesc is the localized description Google uses to mark an actual
	// public (non-working) holiday on this calendar (e.g. "祝日", "Ngày lễ").
	// Observance/festival days use a different description and are excluded.
	publicHolidayDesc string
}

// countryHolidayCalendar is the single source of truth mapping each supported
// country code to its holiday calendar and localized public-holiday marker.
var countryHolidayCalendar = map[string]holidayCalendar{
	"jpn": {id: "ja.japanese#holiday@group.v.calendar.google.com", publicHolidayDesc: "祝日"},
	"vnm": {id: "vi.vietnamese#holiday@group.v.calendar.google.com", publicHolidayDesc: "Ngày lễ"},
}

// publicHolidayDescByCalendarID is derived from countryHolidayCalendar so the two
// never drift apart: adding a country in one place keeps holiday recognition in sync.
var publicHolidayDescByCalendarID = func() map[string]string {
	m := make(map[string]string, len(countryHolidayCalendar))
	for _, hc := range countryHolidayCalendar {
		m[hc.id] = hc.publicHolidayDesc
	}
	return m
}()

func GetHolidayCalendarID(country string) (string, error) {
	hc, ok := countryHolidayCalendar[country]
	if !ok {
		return "", fmt.Errorf("country %s is not supported. Supported countries: %v", country, consts.SupportedCountries)
	}
	return hc.id, nil
}

// isPublicHoliday reports whether an event represents an actual public (non-working)
// holiday on the repository's read calendar, as opposed to an observance/festival day.
//
// Recognition is locale-aware: a day counts as a holiday if its description matches
// either the read calendar's localized public-holiday marker (e.g. "祝日", "Ngày lễ")
// or the English "Public holiday" fallback. Everything else (observances, empty
// descriptions, unknown calendars) is treated as a non-holiday.
func (r *Repository) isPublicHoliday(event *calendar.Event) bool {
	desc := strings.TrimSpace(event.Description)
	if desc == "" {
		return false
	}
	// English fallback: accepted on any calendar in case a localized feed is
	// served untranslated.
	if desc == enPublicHoliday {
		return true
	}
	want, ok := publicHolidayDescByCalendarID[r.readCalendarID]
	if !ok {
		return false
	}
	return desc == want
}
