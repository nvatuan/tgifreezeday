package googlecalendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/nvat/tgifreezeday/internal/domain"
	"github.com/nvat/tgifreezeday/internal/helpers"
	"github.com/nvat/tgifreezeday/internal/logging"
)

var logger = logging.GetLogger()

// Repository implements the CalendarRepository interface for Google Calendar
type Repository struct {
	service         *calendar.Service
	readCalendarID  string
	writeCalendarID string
	calendarTZ      *time.Location
}

// NewRepository creates a new Google Calendar repository for holiday calendar
func NewRepository(
	ctx context.Context,
	credentialsPath,
	countryCode,
	writeCalendarID string,
) (*Repository, error) {
	service, err := calendar.NewService(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	readCalendarID, err := GetHolidayCalendarID(countryCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get holiday calendar ID: %w", err)
	}

	// Get calendar timezone
	cal, err := service.Calendars.Get(writeCalendarID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get calendar info: %w", err)
	}

	calendarTZ, err := time.LoadLocation(cal.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to parse calendar timezone %s: %w", cal.TimeZone, err)
	}

	return &Repository{
		service:         service,
		readCalendarID:  readCalendarID,
		writeCalendarID: writeCalendarID,
		calendarTZ:      calendarTZ,
	}, nil
}

// GetFreezeDaysInRange fetches events for a range [start, end), and maps them to TGIFMapping domain model
// rangeStart is inclusive, rangeEnd is exclusive
func (r *Repository) GetFreezeDaysInRange(rangeStart, rangeEnd time.Time) (*domain.TGIFMapping, error) {
	// Fetch events from Google Calendar
	events, err := r.fetchEvents(rangeStart, rangeEnd)
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

	tgifMapping := make(domain.TGIFMapping)
	currDate := rangeStart
	for currDate.Before(rangeEnd) {
		dateKey := domain.NewDateKey(currDate)
		isHoliday := holidayMap[string(dateKey)]

		tgifDay := domain.NewTGIFDay(currDate, &tgifMapping, isHoliday)
		tgifMapping[dateKey] = tgifDay

		currDate = currDate.AddDate(0, 0, 1)
	}
	// CRITICAL: Fill month info for first/last business day calculations
	// This is required for freeze day rules to work properly
	tgifMapping.FillMonthInfo()

	return &tgifMapping, nil
}

const (
	defaultStartHour          = 8  // local time
	defaultEndHour            = 20 // local time
	defaultBlockerSignature   = "Managed by tgifreezeday, do not modify."
	defaultBlockerDescription = "" + defaultBlockerSignature
)

// WipeAllBlockersInMonth wipes all blockers in the month of the dateAnchor
// Calls WipeAllBlockersInRange with the start and end of the month
// dateAnchor is the date of the month to wipe blockers for
func (r *Repository) WipeAllBlockersInMonth(dateAnchor time.Time) error {
	year, month, _ := dateAnchor.Date()
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, dateAnchor.Location())
	endDate := time.Date(year, month+1, 0, 23, 59, 59, 999999999, dateAnchor.Location())
	return r.WipeAllBlockersInRange(startDate, endDate)
}

// get all events from writeCalendarId that has description containing defaultBlockerSignature
// then delete them
func (r *Repository) WipeAllBlockersInRange(startDate, endDate time.Time) error {
	blockerEvents, err := r.fetchBlockerEvents(startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to retrieve blocker events: %w", err)
	}

	// Delete events sequentially - Google Calendar Go client doesn't support batch requests
	for _, event := range blockerEvents {
		deleteCall := r.service.Events.Delete(r.writeCalendarID, event.Id)
		if err := deleteCall.Do(); err != nil {
			return fmt.Errorf("failed to delete blocker event %s: %w", event.Id, err)
		}
	}

	return nil
}

// fetchBlockerEvents retrieves all blocker events from the write calendar within the specified date range
func (r *Repository) fetchBlockerEvents(startDate, endDate time.Time) ([]*calendar.Event, error) {
	// Fetch events from the write calendar within the date range
	call := r.service.Events.List(r.writeCalendarID).
		TimeMin(startDate.Format(time.RFC3339)).
		TimeMax(endDate.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime")

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve events from write calendar: %w", err)
	}

	// Filter for blocker events only
	var blockerEvents []*calendar.Event
	for _, event := range events.Items {
		if event.Description != "" && strings.Contains(event.Description, defaultBlockerDescription) {
			blockerEvents = append(blockerEvents, event)
		}
	}

	return blockerEvents, nil
}

// BlockerEvent represents a blocker event for display purposes
type BlockerEvent struct {
	Id          string
	Summary     string
	Description string
	Start       time.Time
	End         time.Time
}

// ListAllBlockersInRange lists all blocker events in the specified date range
func (r *Repository) ListAllBlockersInRange(startDate, endDate time.Time) ([]*BlockerEvent, error) {
	blockerEvents, err := r.fetchBlockerEvents(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve blocker events: %w", err)
	}

	var blockers []*BlockerEvent
	for _, event := range blockerEvents {
		blocker := &BlockerEvent{
			Id:          event.Id,
			Summary:     event.Summary,
			Description: event.Description,
		}

		// Parse start time
		if event.Start != nil {
			if event.Start.DateTime != "" {
				if startTime, err := time.Parse(time.RFC3339, event.Start.DateTime); err == nil {
					blocker.Start = startTime
				}
			} else if event.Start.Date != "" {
				if startTime, err := time.Parse("2006-01-02", event.Start.Date); err == nil {
					blocker.Start = startTime
				}
			}
		}

		// Parse end time
		if event.End != nil {
			if event.End.DateTime != "" {
				if endTime, err := time.Parse(time.RFC3339, event.End.DateTime); err == nil {
					blocker.End = endTime
				}
			} else if event.End.Date != "" {
				if endTime, err := time.Parse("2006-01-02", event.End.Date); err == nil {
					blocker.End = endTime
				}
			}
		}

		// Debug logging - print full API response for this event
		logger.WithFields(map[string]interface{}{
			"event_id":          event.Id,
			"event_summary":     event.Summary,
			"event_description": event.Description,
			"full_api_response": event,
		}).Debug("Found blocker event - full API response")

		blockers = append(blockers, blocker)
	}

	return blockers, nil
}

func (r *Repository) WriteBlockerOnDate(date time.Time, summary string) error {
	// Convert to calendar timezone for proper display
	calendarDate := date.In(r.calendarTZ)

	startDateTime := time.Date(
		calendarDate.Year(), calendarDate.Month(), calendarDate.Day(),
		defaultStartHour, 0, 0, 0,
		r.calendarTZ)

	endDateTime := time.Date(
		calendarDate.Year(), calendarDate.Month(), calendarDate.Day(),
		defaultEndHour, 0, 0, 0,
		r.calendarTZ)

	call := r.service.Events.Insert(r.writeCalendarID, &calendar.Event{
		Summary:     summary,
		Start:       &calendar.EventDateTime{DateTime: startDateTime.Format(time.RFC3339)},
		End:         &calendar.EventDateTime{DateTime: endDateTime.Format(time.RFC3339)},
		Description: defaultBlockerDescription,
	})

	_, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to write default blocker on date: %w", err)
	}

	return nil
}

// fetchEvents retrieves events from Google Calendar within the specified time range
func (r *Repository) fetchEvents(timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	call := r.service.Events.List(r.readCalendarID).
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
