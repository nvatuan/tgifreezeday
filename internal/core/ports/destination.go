package ports

import (
	"context"
	"time"
)

// Destination defines the interface for actions to be performed based on rule evaluation
type Destination interface {
	// DefaultCreateFreezeEvent creates a calendar event to block the day if it's a freeze day, the default time is 9am-6pm
	DefaultCreateFreezeEvent(ctx context.Context, date time.Time) error
}
