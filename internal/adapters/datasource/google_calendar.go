package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/nvat/tgifreezeday/internal/core/ports"
)

// GoogleCalendarDataSource implements the DataSource interface for Google Calendar
type GoogleCalendarDataSource struct {
	client *calendar.Service

	sourceCalendarID   string
	holidaysCalendarID string
}

// NewGoogleCalendarDataSource creates a new Google Calendar data source
func NewGoogleCalendarDataSource(credentialsFile, sourceCalendarID, holidaysCalendarID string) (ports.DataSource, error) {
	ctx := context.Background()

	// Load credentials from file
	data, err := google.CredentialsFromJSON(ctx, []byte(credentialsFile), calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials file: %v", err)
	}

	// Create Google Calendar service
	srv, err := calendar.NewService(ctx, option.WithCredentials(data))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar client: %v", err)
	}

	return &GoogleCalendarDataSource{
		client:             srv,
		sourceCalendarID:   sourceCalendarID,
		holidaysCalendarID: holidaysCalendarID,
	}, nil
}

// HasEvent checks if a specific date has an event with a specific keyword
// Check your Calendar (not Holiday Calendar) for events
func (ds *GoogleCalendarDataSource) HasEvent(date time.Time, keyword string) (bool, error) {
	// Get the start and end of the day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())

	// Get events for the day
	events, err := ds.client.Events.List(ds.sourceCalendarID).TimeMin(startOfDay.Format(time.RFC3339)).TimeMax(endOfDay.Format(time.RFC3339)).Do()
	if err != nil {
		return false, err
	}

	// If no events exist, default to false
	if len(events.Items) == 0 {
		return false, nil
	}

	// If keyword is empty, return true if any events exist
	if keyword == "" {
		return true, nil
	}

	// If keyword is not empty, return true if any events exist with the keyword
	for _, event := range events.Items {
		if strings.Contains(event.Summary, keyword) {
			return true, nil
		}
	}

	return false, nil
}

// IsWeekend checks if a specific date is a weekend
func (ds *GoogleCalendarDataSource) IsWeekend(date time.Time) (bool, error) {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday, nil
}

// Check your Holiday Calendar for holiday events
func (ds *GoogleCalendarDataSource) IsHoliday(date time.Time) (bool, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())

	// Get events for the day
	events, err := ds.client.Events.List(ds.holidaysCalendarID).TimeMin(startOfDay.Format(time.RFC3339)).TimeMax(endOfDay.Format(time.RFC3339)).Do()
	if err != nil {
		return false, err
	}

	return len(events.Items) > 0, nil
}

// IsBusinessDay checks if a specific date is a business day
func (ds *GoogleCalendarDataSource) IsBusinessDay(date time.Time) (bool, error) {
	isWeekend, err := ds.IsWeekend(date)
	if err != nil {
		return false, err
	}

	isHoliday, err := ds.IsHoliday(date)
	if err != nil {
		return false, err
	}

	return !isWeekend && !isHoliday, nil
}

// IsFirstBusinessDayOfTheMonth checks if a specific date is the first business day of the month
func (ds *GoogleCalendarDataSource) IsFirstBusinessDayOfTheMonth(date time.Time) (bool, error) {
	// Normalize the date to remove time component
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	// Get the first day of the month
	firstDayOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())

	// Loop through days starting from the first day of the month
	currentDay := firstDayOfMonth

	// Check each day until we find the first business day
	for iter := 1; iter < 31; iter++ {
		// Make sure we're still in the same month
		if currentDay.Month() != date.Month() {
			// If we've gone past the end of the month without finding a business day, return false
			return false, nil
		}

		// Check if the current day is a business day
		isBusinessDay, err := ds.IsBusinessDay(currentDay)
		if err != nil {
			return false, err
		}

		// If we found the first business day
		if isBusinessDay {
			// Compare with the input date
			return normalizedDate.Equal(currentDay), nil
		}

		// Move to the next day
		currentDay = currentDay.AddDate(0, 0, 1)
	}

	// If we get here, no business day was found in the month
	return false, nil
}

// IsLastBusinessDayOfTheMonth checks if a specific date is the last business day of the month
func (ds *GoogleCalendarDataSource) IsLastBusinessDayOfTheMonth(date time.Time) (bool, error) {
	// Normalize the date to remove time component
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	// Get the last day of the month
	nextMonth := date.AddDate(0, 1, 0)
	firstDayOfNextMonth := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, date.Location())
	lastDayOfMonth := firstDayOfNextMonth.AddDate(0, 0, -1)

	// Start from the last day of the month and work backwards
	currentDay := lastDayOfMonth

	// Check each day until we find the last business day
	for iter := 1; iter < 31; iter++ {
		// Make sure we're still in the same month
		if currentDay.Month() != date.Month() {
			// If we've gone past the beginning of the month without finding a business day, return false
			return false, nil
		}

		// Check if the current day is a business day
		isBusinessDay, err := ds.IsBusinessDay(currentDay)
		if err != nil {
			return false, err
		}

		// If we found the last business day
		if isBusinessDay {
			// Compare with the input date
			return normalizedDate.Equal(currentDay), nil
		}

		// Move to the previous day
		currentDay = currentDay.AddDate(0, 0, -1)
	}

	// If we get here, no business day was found in the month
	return false, nil
}
