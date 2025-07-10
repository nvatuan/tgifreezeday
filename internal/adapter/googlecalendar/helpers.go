package googlecalendar

import (
	"fmt"

	"github.com/nvat/tgifreezeday/internal/consts"
	"google.golang.org/api/calendar/v3"
)

// SupportedCountry represents a supported country for holiday calendars
var NationalHolidayCalendarMap = map[string]string{
	consts.CountryJapan:   "en.japanese#holiday@group.v.calendar.google.com",
	consts.CountryVietnam: "en.vietnam#holiday@group.v.calendar.google.com",
}

const (
	// if the calendar in English, the following descriptions are used to determine if
	// the day is a public holiday or observance
	en_PublicHoliday = "Public holiday"
	en_Observance    = "Observance"
)

// GetHolidayCalendarId returns the calendar ID for a country
func GetHolidayCalendarId(country string) (string, error) {
	if v, ok := NationalHolidayCalendarMap[country]; !ok {
		return "", fmt.Errorf("unsupported country: %s. Supported countries: japan, vietnam", country)
	} else {
		return v, nil
	}
}

// isPublicHoliday determines if an event represents an actual public holiday vs observance
func (r *Repository) isPublicHoliday(event *calendar.Event) bool {
	// Check event description for holiday type indicators
	if event.Description != "" {
		desc := event.Description

		// "Public holiday" = actual non-working days
		if desc == en_PublicHoliday {
			return true
		}

		// "Observance" = cultural/festival days (still working days)
		if desc == en_Observance {
			return false
		}
	}

	return false
}
