package config

import (
	"fmt"

	"github.com/nvat/tgifreezeday/internal/helpers"
	"gopkg.in/yaml.v3"
)

type SharedConfig struct {
	LookbackDays  int `yaml:"lookbackDays"`
	LookaheadDays int `yaml:"lookaheadDays"`
}

type ReadFromConfig struct {
	GoogleCalendar GoogleCalendarReadConfig `yaml:"googleCalendar"`
}

type GoogleCalendarReadConfig struct {
	// ISO 3166 A-3 country code
	CountryCode        string                `yaml:"countryCode"`
	TodayIsFreezeDayIf []map[string][]string `yaml:"todayIsFreezeDayIf"`
}

type WriteToConfig struct {
	GoogleCalendar GoogleCalendarWriteConfig `yaml:"googleCalendar"`
}

type GoogleCalendarWriteConfig struct {
	ID                 string                   `yaml:"id"`
	IfTodayIsFreezeDay IfTodayIsFreezeDayConfig `yaml:"ifTodayIsFreezeDay"`
}

type IfTodayIsFreezeDayConfig struct {
	Default DefaultConfig `yaml:"default"`
}

type DefaultConfig struct {
	Summary     *string `yaml:"summary,omitempty"`
	Description *string `yaml:"description,omitempty"`
	StartTime   *string `yaml:"startTime,omitempty"`
	EndTime     *string `yaml:"endTime,omitempty"`
	AllDay      *bool   `yaml:"allDay,omitempty"`
}

type Config struct {
	Shared   SharedConfig   `yaml:"shared"`
	ReadFrom ReadFromConfig `yaml:"readFrom"`
	WriteTo  WriteToConfig  `yaml:"writeTo"`
}

func LoadWithDefaultFromByteArray(data []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config.SetDefault()
	return &config, nil
}

const defaultSummary = "Today is FREEZE-DAY. no PROD operation is allowed."
const defaultDescription = "Managed by tgifreezeday, do not modify."

// ToYAML marshals the config back to YAML bytes.
func (c *Config) ToYAML() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	return string(data), nil
}

func (c *Config) SetDefault() {
	if c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary == nil {
		c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary = helpers.StringPtr(defaultSummary)
	}
	if c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Description == nil {
		c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Description = helpers.StringPtr(defaultDescription)
	}
	// Only set time defaults for timed (non-all-day) events.
	allDay := c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.AllDay
	if allDay == nil || !*allDay {
		if c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.StartTime == nil {
			c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.StartTime = helpers.StringPtr("08:00")
		}
		if c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.EndTime == nil {
			c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.EndTime = helpers.StringPtr("20:00")
		}
	}
}
