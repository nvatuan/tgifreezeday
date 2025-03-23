package destination

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/nvat/tgifreezeday/internal/core/domain"
	"github.com/nvat/tgifreezeday/internal/core/ports"
)

// SlackDestination implements the Destination interface for Slack notifications
type SlackDestination struct {
	client    *slack.Client
	channelID string
}

// NewSlackDestination creates a new Slack destination
func NewSlackDestination(token, channelID string) (ports.Destination, error) {
	client := slack.New(token)

	return &SlackDestination{
		client:    client,
		channelID: channelID,
	}, nil
}

// UpdateFreezePeriods updates freeze periods
// For Slack, this is a no-op as Slack is notification-only
func (d *SlackDestination) UpdateFreezePeriods(ctx context.Context, periods []domain.FreezePeriod) error {
	// For each freeze period, send a notification
	for _, period := range periods {
		err := d.NotifyFreezePeriod(ctx, period)
		if err != nil {
			return err
		}
	}

	return nil
}

// NotifyFreezePeriod sends a Slack notification about an upcoming freeze period
func (d *SlackDestination) NotifyFreezePeriod(ctx context.Context, period domain.FreezePeriod) error {
	// Format dates for display
	startStr := period.StartDate.Format("Mon, Jan 2 2006 at 3:04PM")
	endStr := period.EndDate.Format("Mon, Jan 2 2006 at 3:04PM")

	// Create message with attachments for rich formatting
	attachment := slack.Attachment{
		Color:      "#FF0000", // Red for warning
		Title:      "Production Release Freeze Period",
		Text:       period.Description,
		MarkdownIn: []string{"text", "fields"},
		Fields: []slack.AttachmentField{
			{
				Title: "Period",
				Value: fmt.Sprintf("%s to %s", startStr, endStr),
				Short: false,
			},
		},
		Footer: "TGIFreezeDay Notification",
		Ts:     json.Number(fmt.Sprintf("%d", time.Now().Unix())),
	}

	// Add related holidays if available
	if len(period.RelatedHolidays) > 0 {
		var holidayText string
		for i, holiday := range period.RelatedHolidays {
			if i > 0 {
				holidayText += ", "
			}
			holidayText += holiday.Title
		}

		attachment.Fields = append(attachment.Fields, slack.AttachmentField{
			Title: "Related Holidays",
			Value: holidayText,
			Short: false,
		})
	}

	// Post the message to Slack
	_, _, err := d.client.PostMessageContext(
		ctx,
		d.channelID,
		slack.MsgOptionText("Upcoming Production Release Freeze:", false),
		slack.MsgOptionAttachments(attachment),
	)

	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %v", err)
	}

	return nil
}

// DefaultCreateFreezeEvent creates a calendar event to block the day if it's a freeze day
// For Slack, this sends a notification about the freeze day
func (d *SlackDestination) DefaultCreateFreezeEvent(ctx context.Context, date time.Time) error {
	// Format the date
	dateStr := date.Format("Monday, January 2, 2006")

	// Create a message with attachments for rich formatting
	attachment := slack.Attachment{
		Color:      "#FF0000", // Red for warning
		Title:      "Production Release Freeze Day",
		Text:       "This day is marked as a freeze day. No production deployments allowed.",
		MarkdownIn: []string{"text", "fields"},
		Fields: []slack.AttachmentField{
			{
				Title: "Freeze Date",
				Value: dateStr,
				Short: false,
			},
			{
				Title: "Freeze Time",
				Value: "9:00 AM - 6:00 PM",
				Short: false,
			},
		},
		Footer: "TGIFreezeDay Notification",
		Ts:     json.Number(fmt.Sprintf("%d", time.Now().Unix())),
	}

	// Post the message to Slack
	_, _, err := d.client.PostMessageContext(
		ctx,
		d.channelID,
		slack.MsgOptionText("Production Release Freeze Day Notification:", false),
		slack.MsgOptionAttachments(attachment),
	)

	if err != nil {
		return fmt.Errorf("failed to send Slack notification for freeze day: %v", err)
	}

	return nil
}
