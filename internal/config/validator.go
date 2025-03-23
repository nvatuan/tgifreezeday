package config

import (
	"fmt"
	"slices"
)

func (c *Configuration) Validate() error {
	if err := c.DataSource.Validate(); err != nil {
		return fmt.Errorf("invalid data source configuration: %v", err)
	}
	if err := c.Destination.Validate(); err != nil {
		return fmt.Errorf("invalid destination configuration: %v", err)
	}
	return nil
}

func (c *DataSourceConfig) Validate() error {
	if !slices.Contains(DataSourceTypeSupportedTypes, c.Type) {
		return fmt.Errorf("invalid data source type: %s", c.Type)
	}
	for _, validator := range dataSourceConfigValidators {
		if err := validator(c); err != nil {
			return fmt.Errorf("invalid data source configuration: %v", err)
		}
	}
	return nil
}

func (c *DestinationConfig) Validate() error {
	if !slices.Contains(DestinationTypeSupportedTypes, c.Type) {
		return fmt.Errorf("invalid destination type: %s", c.Type)
	}
	for _, validator := range destinationConfigValidators {
		if err := validator(c); err != nil {
			return fmt.Errorf("invalid destination configuration: %v", err)
		}
	}
	return nil
}
