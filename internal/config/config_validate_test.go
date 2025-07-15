package config

import (
	"reflect"
	"testing"
)

// configsEqual compares two Config structs, properly handling pointer fields
func configsEqual(a, b *Config) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare non-pointer fields using DeepEqual
	if !reflect.DeepEqual(a.Shared, b.Shared) {
		return false
	}
	if !reflect.DeepEqual(a.ReadFrom, b.ReadFrom) {
		return false
	}

	// Compare WriteTo fields
	if a.WriteTo.GoogleCalendar.ID != b.WriteTo.GoogleCalendar.ID {
		return false
	}

	// Compare pointer fields by value, not address
	aDefault := a.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default
	bDefault := b.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default

	// Compare Summary pointers
	if !stringPtrsEqual(aDefault.Summary, bDefault.Summary) {
		return false
	}

	// Compare Description pointers
	if !stringPtrsEqual(aDefault.Description, bDefault.Description) {
		return false
	}

	return true
}

// stringPtrsEqual compares two string pointers by their values
func stringPtrsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func Test_ConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want *Config
	}{
		{name: "valid", yaml: mockConfigYamlValid, want: mockValidParsedConfig},
		{name: "invalid_countryCode", yaml: mockConfigYamlInvalidCountryCode, want: nil},
		{name: "invalid_unsupportedDate", yaml: mockConfigYamlInvalidUnsupportedDate, want: nil},
		{name: "invalid_unsupportedCheck", yaml: mockConfigYamlInvalidUnsupportedCheck, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := LoadWithDefaultFromByteArray([]byte(test.yaml))

			valErr := got.Validate()

			if test.want == nil && valErr == nil {
				t.Errorf("config.Validate() expects error, but got nil")
			}

			if test.want != nil && !configsEqual(got, test.want) {
				t.Errorf("config.Validate() = %v, want %v", got, test.want)
			}
		})
	}
}
