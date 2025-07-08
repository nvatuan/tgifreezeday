package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	// GoogleAppClientCredJSONPath is the environment variable name for Google credentials path
	GoogleAppClientCredJSONPath = "GOOGLE_APP_CLIENT_CRED_JSON_PATH"
)

// Config is the root config struct matching the YAML example in README
// Only base fields for now

type Config struct {
	ReadFrom struct {
		GoogleCalendar struct {
			ID                 string                `yaml:"id"`
			TodayIsFreezeDayIf []map[string][]string `yaml:"todayIsFreezeDayIf"`
		} `yaml:"googleCalendar"`
	} `yaml:"readFrom"`
	WriteTo struct {
		GoogleCalendar struct {
			ID                 string `yaml:"id"`
			IfTodayIsFreezeDay struct {
				Default struct {
					Summary *string `yaml:"summary"`
				} `yaml:"default"`
			} `yaml:"ifTodayIsFreezeDay"`
		} `yaml:"googleCalendar"`
	} `yaml:"writeTo"`
}

// Load loads config from file/env
func Load() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Check if Google credentials are set
	if os.Getenv(GoogleAppClientCredJSONPath) == "" {
		return nil, fmt.Errorf("%s environment variable is required", GoogleAppClientCredJSONPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if config.ReadFrom.GoogleCalendar.ID == "" {
		return nil, fmt.Errorf("readFrom.googleCalendar.id is required")
	}
	if config.WriteTo.GoogleCalendar.ID == "" {
		return nil, fmt.Errorf("writeTo.googleCalendar.id is required")
	}
	if len(config.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) == 0 {
		return nil, fmt.Errorf("readFrom.googleCalendar.todayIsFreezeDayIf cannot be empty")
	}

	return &config, nil
}

// GetCredentialsPath returns the path to Google credentials file
func GetCredentialsPath() string {
	return os.Getenv(GoogleAppClientCredJSONPath)
}
