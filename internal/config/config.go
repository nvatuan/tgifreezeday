package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/nvat/tgifreezeday/internal/consts"
	"gopkg.in/yaml.v3"
)

const (
	GoogleAppClientCredJSONPathEnv = "GOOGLE_APP_CLIENT_CRED_JSON_PATH"
)

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
	Summary string `yaml:"summary"`
}

type Config struct {
	ReadFrom ReadFromConfig `yaml:"readFrom"`
	WriteTo  WriteToConfig  `yaml:"writeTo"`
}

func Load() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// ValidateCountry checks if a country is supported
func ValidateCountry(country string) error {
	if !slices.Contains(consts.SupportedCountries, country) {
		return fmt.Errorf("unsupported country: %s. Supported countries: %v", country, consts.SupportedCountries)
	}
	return nil
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

	// Validate country
	if err := ValidateCountry(c.ReadFrom.GoogleCalendar.CountryCode); err != nil {
		return fmt.Errorf("invalid readFrom.googleCalendar.countryCode: %w", err)
	}

	// Validate freeze day rules
	if len(c.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) == 0 {
		return fmt.Errorf("readFrom.googleCalendar.todayIsFreezeDayIf cannot be empty")
	}

	return nil
}
