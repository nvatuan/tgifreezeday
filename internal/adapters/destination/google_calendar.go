package destination

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/nvat/tgifreezeday/internal/core/ports"
)

// GoogleCalendarDestination implements the Destination interface for Google Calendar
type GoogleCalendarDestination struct {
	client           *calendar.Service
	freezeCalendarID string
}

// NewGoogleCalendarDestination creates a new Google Calendar destination
func NewGoogleCalendarDestination(credentialsFile, freezeCalendarID string) (ports.Destination, error) {
	ctx := context.Background()

	// Load credentials from file
	data, err := google.CredentialsFromJSON(ctx, []byte(credentialsFile),
		calendar.CalendarReadonlyScope,
		calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials file: %v", err)
	}

	// Create Google Calendar service
	srv, err := calendar.NewService(ctx, option.WithCredentials(data))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar client: %v", err)
	}

	return &GoogleCalendarDestination{
		client:           srv,
		freezeCalendarID: freezeCalendarID,
	}, nil
}

// DefaultCreateFreezeEvent creates a calendar event to block the day if it's a freeze day, the default time is 9am-6pm
func (d *GoogleCalendarDestination) DefaultCreateFreezeEvent(ctx context.Context, date time.Time) error {
	// Format dates in the specified timezone
	location := date.Location()

	// Set event start time (9am) and end time (6pm)
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, location)
	endTime := time.Date(date.Year(), date.Month(), date.Day(), 18, 0, 0, 0, location)

	// Create the event
	event := &calendar.Event{
		Summary:     "Production Release Forbidden",
		Description: "This day is marked as a freeze day. No production deployments allowed.",
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: location.String(),
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: location.String(),
		},
		ColorId:      "11",     // Red color
		Transparency: "opaque", // Show as busy
		Visibility:   "public",
	}

	// Check if an event already exists for this day
	timeMin := startTime.Format(time.RFC3339)
	timeMax := endTime.Format(time.RFC3339)

	existingEvents, err := d.client.Events.List(d.freezeCalendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(timeMin).
		TimeMax(timeMax).
		Do()
	if err != nil {
		return fmt.Errorf("unable to check existing events: %v", err)
	}

	// Look for an existing freeze day event
	var existingEvent *calendar.Event
	for _, e := range existingEvents.Items {
		if e.Summary == "Production Release Forbidden" {
			existingEvent = e
			break
		}
	}

	// Update existing event or create a new one
	if existingEvent != nil {
		// Update the existing event
		_, err = d.client.Events.Update(d.freezeCalendarID, existingEvent.Id, event).Do()
		if err != nil {
			return fmt.Errorf("unable to update freeze day event: %v", err)
		}
	} else {
		// Create a new event
		_, err = d.client.Events.Insert(d.freezeCalendarID, event).Do()
		if err != nil {
			return fmt.Errorf("unable to create freeze day event: %v", err)
		}
	}

	return nil
}
