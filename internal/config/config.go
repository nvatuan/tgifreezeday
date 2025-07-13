package config

import (
	"fmt"
	"os"

	"github.com/nvat/tgifreezeday/internal/helpers"
	"gopkg.in/yaml.v3"
)

const (
	GoogleAppClientCredJSONPathEnv = "GOOGLE_APP_CLIENT_CRED_JSON_PATH"
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
	Summary *string `yaml:"summary"`
}

type Config struct {
	Shared   SharedConfig   `yaml:"shared"`
	ReadFrom ReadFromConfig `yaml:"readFrom"`
	WriteTo  WriteToConfig  `yaml:"writeTo"`
}

func LoadWithDefault() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return LoadWithDefaultFromByteArray(data)
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

func (c *Config) SetDefault() {
	if c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary == nil {
		c.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary = helpers.StringPtr(defaultSummary)
	}
}
