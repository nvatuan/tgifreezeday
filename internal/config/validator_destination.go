package config

import "fmt"

var destinationConfigValidators = []func(*DestinationConfig) error{
	validateGoogleCalendarDestinationConfig,
}

func validateGoogleCalendarDestinationConfig(c *DestinationConfig) error {
	if c.Type != DestinationTypeGoogleCalendar {
		return nil
	}

	if _, ok := c.Config["calendar_id"]; !ok {
		return fmt.Errorf("destination: google calendar: calendar_id is required")
	}

	return nil
}
