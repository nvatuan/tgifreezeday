package config

import (
	"reflect"
	"testing"
)

func Test_ConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want *Config
	}{
		{name: "valid", yaml: mock_configYamlValid, want: mockValidParsedConfig},
		{name: "invalid_countryCode", yaml: mock_configYamlInvalid_countryCode, want: nil},
		{name: "invalid_unsupportedDate", yaml: mock_configYamlInvalid_unsupportedDate, want: nil},
		{name: "invalid_unsupportedCheck", yaml: mock_configYamlInvalid_unsupportedCheck, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := LoadWithDefaultFromByteArray([]byte(test.yaml))

			if got.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary == nil {
				println("summary is nil")
			} else {
				println("summary is %v", *got.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary)
			}
			valErr := got.Validate()
			if got.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary == nil {
				println("summary is nil")
			} else {
				println("summary is %v", *got.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary)
			}

			if test.want == nil && valErr == nil {
				t.Errorf("config.Validate() expects error, but got nil")
			}

			if test.want != nil && !reflect.DeepEqual(got, test.want) {
				t.Errorf("config.Validate() = %v, want %v", got, test.want)
			}
		})
	}
}
