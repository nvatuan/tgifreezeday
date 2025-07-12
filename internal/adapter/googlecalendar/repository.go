package googlecalendar

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/nvat/tgifreezeday/internal/domain"
	"github.com/nvat/tgifreezeday/internal/helpers"
)

// Repository implements the CalendarRepository interface for Google Calendar
type Repository struct {
	service    *calendar.Service
	calendarID string
}

// NewRepository creates a new Google Calendar repository for holiday calendar
func NewRepository(ctx context.Context, credentialsPath, countryCode string) (*Repository, error) {
	service, err := calendar.NewService(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	calendarID, err := GetHolidayCalendarId(countryCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get holiday calendar ID: %w", err)
	}

	return &Repository{
		service:    service,
		calendarID: calendarID,
	}, nil
}

// GetMonthCalendar fetches events for a month and maps them to MonthCalendar domain model
func (r *Repository) GetMonthCalendar(dateAnchor time.Time) (*domain.MonthCalendar, error) {
	// Get the first and last day of the month
	year, month, _ := dateAnchor.Date()
	location := dateAnchor.Location()

	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, location)
	lastDay := time.Date(year, month+1, 0, 23, 59, 59, 999999999, location) // Last moment of the month

	// Fetch events from Google Calendar
	events, err := r.fetchEvents(firstDay, lastDay)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}

	// Create holiday map for quick lookup (only for actual public holidays)
	holidayMap := make(map[string]bool)
	for _, event := range events {
		eventDate := r.extractEventDate(event)
		if !eventDate.IsZero() {
			isHoliday := r.isPublicHoliday(event)
			dateKey := helpers.DateKey(eventDate)

			if isHoliday {
				holidayMap[dateKey] = true
			}
		}
	}

	// Build MonthDay slice for the entire month
	daysInMonth := helpers.DaysInMonth(dateAnchor)
	monthDays := make([]domain.MonthDay, 0, daysInMonth)

	for day := 1; day <= daysInMonth; day++ {
		currentDate := time.Date(year, month, day, 0, 0, 0, 0, location)
		dateKey := helpers.DateKey(currentDate)

		isHoliday := holidayMap[dateKey]
		monthDay := domain.NewMonthDay(currentDate, isHoliday)
		monthDays = append(monthDays, *monthDay)
	}

	// Create and return MonthCalendar
	monthCalendar := domain.NewMonthCalendar(dateAnchor, monthDays)

	// Process the calendar to set first/last business days
	if err := monthCalendar.Process(); err != nil {
		return nil, fmt.Errorf("failed to process month calendar: %w", err)
	}

	return monthCalendar, nil
}

// fetchEvents retrieves events from Google Calendar within the specified time range
func (r *Repository) fetchEvents(timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	call := r.service.Events.List(r.calendarID).
		TimeMin(timeMin.Format(time.RFC3339)).
		TimeMax(timeMax.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime")

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve events from Google Calendar: %w", err)
	}

	return events.Items, nil
}

// extractEventDate extracts the date from a Google Calendar event
func (r *Repository) extractEventDate(event *calendar.Event) time.Time {
	if event.Start == nil {
		return time.Time{}
	}

	// Try DateTime first (for timed events)
	if event.Start.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, event.Start.DateTime); err == nil {
			return t
		}
	}

	// Try Date (for all-day events)
	if event.Start.Date != "" {
		if t, err := time.Parse("2006-01-02", event.Start.Date); err == nil {
			return t
		}
	}

	return time.Time{}
}
