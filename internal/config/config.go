package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Configuration represents the top-level configuration structure ---
type Configuration struct {
	DataSource  DataSourceConfig  `yaml:"data_source"`
	Destination DestinationConfig `yaml:"destination"`
}

// DataSourceConfig defines configuration for data sources ---
type DataSourceType string

const (
	DataSourceTypeGoogleCalendar DataSourceType = "google_calendar"
)

var DataSourceTypeSupportedTypes = []DataSourceType{
	DataSourceTypeGoogleCalendar,
}

type DataSourceConfig struct {
	Type   DataSourceType    `yaml:"type"`
	Config map[string]string `yaml:"config"`
	Rules  []RuleConfig      `yaml:"rules"`
}

// RuleConfig defines a single rule expression
type RuleConfig struct {
	Expression string `yaml:"expression"`
}

// DestinationConfig defines configuration for destinations ---
type DestinationType string

const (
	DestinationTypeGoogleCalendar DestinationType = "google_calendar"
)

var DestinationTypeSupportedTypes = []DestinationType{
	DestinationTypeGoogleCalendar,
}

type DestinationConfig struct {
	Type   DestinationType   `yaml:"type"`
	Config map[string]string `yaml:"config"`
}

// Load loads configuration from a file ---
func Load(path string) (*Configuration, error) {
	// If path is empty, try to use default config paths
	if path == "" {
		return nil, fmt.Errorf("config file not provided")
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse YAML
	config := &Configuration{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return config, nil
}
