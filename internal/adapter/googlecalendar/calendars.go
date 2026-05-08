package googlecalendar

import (
	"context"
	"fmt"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"golang.org/x/oauth2"
)

// CalendarItem represents a calendar the user can write to.
type CalendarItem struct {
	ID      string
	Summary string
}

// ListWritableCalendars returns all calendars where the user has owner or writer access.
func ListWritableCalendars(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, userID int64, store TokenStore) ([]*CalendarItem, error) {
	client := NewHTTPClientWithPersistence(ctx, cfg, token, userID, store)
	svc, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	var items []*CalendarItem
	call := svc.CalendarList.List()
	for {
		result, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list calendars: %w", err)
		}
		for _, c := range result.Items {
			if c.AccessRole == "owner" || c.AccessRole == "writer" {
				items = append(items, &CalendarItem{ID: c.Id, Summary: c.Summary})
			}
		}
		if result.NextPageToken == "" {
			break
		}
		call = call.PageToken(result.NextPageToken)
	}
	return items, nil
}
