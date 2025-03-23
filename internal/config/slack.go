package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// SlackDestinationConfig defines Slack destination configuration
type SlackDestinationConfig struct {
	Token     string `yaml:"token"`
	ChannelID string `yaml:"channel_id"`
}

// ParseSlackDestinationConfig parses the raw config into a typed struct
func (c *DestinationConfig) ParseSlackDestinationConfig() (*SlackDestinationConfig, error) {
	if c.Type != "slack" {
		return nil, fmt.Errorf("destination type is not slack")
	}

	// Parse the raw config
	yamlData, err := yaml.Marshal(c.Config)
	if err != nil {
		return nil, fmt.Errorf("error marshaling slack config: %v", err)
	}

	config := &SlackDestinationConfig{}
	if err := yaml.Unmarshal(yamlData, config); err != nil {
		return nil, fmt.Errorf("error parsing slack config: %v", err)
	}

	return config, nil
}
