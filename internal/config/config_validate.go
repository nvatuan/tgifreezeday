package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/nvat/tgifreezeday/internal/consts"
)

// v1 config:
// // readFrom:
// //   googleCalendar:
// //     countryCode: <supported country code> # "jpn", "vnm", A-3 ISO 3166 country code
// //     todayIsFreezeDayIf:
// //     - [yesterday, today, tomorrow]: # with this block, rules are AND together. To do OR, specify multiple items with same key.
// //       - isTheFirstBusinessDayOfTheMonth
// //       - isTheLastBusinessDayOfTheMonth
// //       - isNonBusinessDay
// // writeTo:
// //   googleCalendar:
// //     id: <google calendary id to read>
// //     ifTodayIsFreezeDay:
// //       default:
// //         summary: "string|null" # if `null`, use default message

var supportedDates = []string{
	"today",
	"tomorrow",
	"nextDay",
}
var supportedChecks = []string{
	"isTheFirstBusinessDayOfTheMonth",
	"isTheLastBusinessDayOfTheMonth",
	"isNonBusinessDay",
}

func (c *Config) Validate() error {
	// Validate Google credentials path
	credPath := os.Getenv(GoogleAppClientCredJSONPathEnv)
	if credPath == "" {
		return fmt.Errorf("environment variable %s is required", GoogleAppClientCredJSONPathEnv)
	}

	// Validate credentials file exists
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return fmt.Errorf("google credentials file not found: %s", credPath)
	}

	// Validate Shared block
	if err := c.Validate_Shared_LookbackAndLookaheadDays(); err != nil {
		return fmt.Errorf("invalid shared.lookbackDays or lookaheadDays: %w", err)
	}

	// Validate ReadFrom block
	// // Validate country
	if err := c.Validate_ReadFrom_GoogleCalendar_CountryCode(); err != nil {
		return fmt.Errorf("invalid readFrom.googleCalendar.countryCode: %w", err)
	}

	// // Validate freeze day rules
	if err := c.Validate_ReadFrom_GoogleCalendar_TodayIsFreezeDayIf(); err != nil {
		return fmt.Errorf("invalid readFrom.googleCalendar.todayIsFreezeDayIf: %w", err)
	}

	// Validate WriteTo block
	// // Validate google calendar id
	if err := c.Validate_WriteTo_GoogleCalendar_ID(); err != nil {
		return fmt.Errorf("invalid writeTo.googleCalendar.id: %w", err)
	}

	if err := c.SetDefaultAndValidate_WriteTo_GoogleCalendar_IfTodayIsFreezeDay(); err != nil {
		return fmt.Errorf("invalid writeTo.googleCalendar.ifTodayIsFreezeDay: %w", err)
	}

	return nil
}

func (c *Config) Validate_ReadFrom_GoogleCalendar_TodayIsFreezeDayIf() error {
	if len(c.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) == 0 {
		return fmt.Errorf("readFrom.googleCalendar.todayIsFreezeDayIf cannot be empty")
	}
	for _, rule := range c.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf {
		for date, checks := range rule {
			if !slices.Contains(supportedDates, date) {
				return fmt.Errorf("unsupported date: %s. Supported dates: %v", date, supportedDates)
			}
			for _, check := range checks {
				if !slices.Contains(supportedChecks, check) {
					return fmt.Errorf("unsupported check: %s. Supported checks: %v", check, supportedChecks)
				}
			}
		}
	}
	return nil
}

// ValidateCountry checks if a country is supported
func (c *Config) Validate_ReadFrom_GoogleCalendar_CountryCode() error {
	country := c.ReadFrom.GoogleCalendar.CountryCode
	if !slices.Contains(consts.SupportedCountries, country) {
		return fmt.Errorf("unsupported country: %s. Supported countries: %v", country, consts.SupportedCountries)
	}
	return nil
}

// Validate lookback and lookahead days
func (c *Config) Validate_Shared_LookbackAndLookaheadDays() error {
	if c.Shared.LookbackDays < 20 {
		return fmt.Errorf("shared.lookbackDays cannot be less than 20")
	}
	if c.Shared.LookaheadDays < 20 {
		return fmt.Errorf("shared.lookaheadDays cannot be less than 20")
	}

	if c.Shared.LookbackDays > 60 {
		return fmt.Errorf("shared.lookbackDays cannot be greater than 60")
	}
	if c.Shared.LookaheadDays > 60 {
		return fmt.Errorf("shared.lookaheadDays cannot be greater than 60")
	}
	return nil
}

func (c *Config) Validate_WriteTo_GoogleCalendar_ID() error {
	if c.WriteTo.GoogleCalendar.ID == "" {
		return fmt.Errorf("writeTo.googleCalendar.id cannot be empty")
	}
	return nil
}

// Validate the event to write on the WriteTo calendar
// SIDE EFFECT!! if summary is nil, set it to default message
func (c *Config) SetDefaultAndValidate_WriteTo_GoogleCalendar_IfTodayIsFreezeDay() error {
	// enforce limits so that it won't be rejected by Google Calendar API
	if len([]rune(*c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary)) > 250 {
		return fmt.Errorf("writeTo.googleCalendar.ifTodayIsFreezeDay.default.summary cannot be longer than 250 characters")
	}
	return nil
}
