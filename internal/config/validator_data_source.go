package config

import "fmt"

var dataSourceConfigValidators = []func(*DataSourceConfig) error{
	validateGoogleCalendarConfig,
}

func validateGoogleCalendarConfig(c *DataSourceConfig) error {
	if c.Type != DataSourceTypeGoogleCalendar {
		return nil
	}

	if _, ok := c.Config["calendar_id"]; !ok {
		return fmt.Errorf("data source: google calendar: calendar_id is required")
	}

	return nil
}
