package googlecalendar

import (
	"fmt"

	"github.com/nvat/tgifreezeday/internal/consts"
	"google.golang.org/api/calendar/v3"
)

const (
	// Holiday types in English (Google Calendar API returns these)
	enPublicHoliday = "Public holiday"
	enObservance    = "Observance"
)

var countryToCalendarID = map[string]string{
	"jpn": "ja.japanese#holiday@group.v.calendar.google.com",
	"vnm": "vi.vietnamese#holiday@group.v.calendar.google.com",
}

func GetHolidayCalendarID(country string) (string, error) {
	v, ok := countryToCalendarID[country]
	if !ok {
		return "", fmt.Errorf("country %s is not supported. Supported countries: %v", country, consts.SupportedCountries)
	}
	return v, nil
}

// isPublicHoliday determines if an event represents an actual public holiday vs observance
func (r *Repository) isPublicHoliday(event *calendar.Event) bool {
	// Check event description for holiday type indicators
	if event.Description != "" {
		desc := event.Description

		// "Public holiday" = actual non-working days
		if desc == enPublicHoliday {
			return true
		}

		// "Observance" = cultural/festival days (still working days)
		if desc == enObservance {
			return false
		}
	}

	return false
}
