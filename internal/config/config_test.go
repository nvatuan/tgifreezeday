package config

import (
	"strings"
	"testing"

	"github.com/nvat/tgifreezeday/internal/helpers"
)

// anchorToday and condNonBusiness are local constants to avoid hitting the
// goconst min-occurrences threshold when combined with the shared test data.
const (
	anchorToday     = "today"
	condNonBusiness = "isNonBusinessDay"
)

func TestToYAML_RoundTrip(t *testing.T) {
	// Reuse mock data from testdata to avoid goconst threshold on string literals.
	original := mockValidParsedConfig

	yamlStr, err := original.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}
	if yamlStr == "" {
		t.Fatal("ToYAML() returned empty string")
	}

	// Parse back and validate the round-trip
	parsed, err := LoadWithDefaultFromByteArray([]byte(yamlStr))
	if err != nil {
		t.Fatalf("LoadWithDefaultFromByteArray() after ToYAML() error = %v", err)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatalf("Validate() after round-trip error = %v", err)
	}

	if !configsEqual(original, parsed) {
		t.Errorf("round-trip mismatch:\noriginal: %+v\nparsed:   %+v", original, parsed)
	}
}

func TestToYAML_ContainsExpectedFields(t *testing.T) {
	cfg := &Config{
		Shared: SharedConfig{LookbackDays: 30, LookaheadDays: 45},
		ReadFrom: ReadFromConfig{
			GoogleCalendar: GoogleCalendarReadConfig{
				CountryCode:        "vnm",
				TodayIsFreezeDayIf: []map[string][]string{{anchorToday: {condNonBusiness}}},
			},
		},
		WriteTo: WriteToConfig{
			GoogleCalendar: GoogleCalendarWriteConfig{
				ID: "myteam@group.calendar.google.com",
				IfTodayIsFreezeDay: IfTodayIsFreezeDayConfig{
					Default: DefaultConfig{
						Summary:     helpers.StringPtr("Freeze"),
						Description: helpers.StringPtr("No prod ops"),
						StartTime:   helpers.StringPtr("09:00"),
						EndTime:     helpers.StringPtr("18:00"),
					},
				},
			},
		},
	}

	yamlStr, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}

	checks := []string{
		"lookbackDays: 30",
		"lookaheadDays: 45",
		"countryCode: vnm",
		"myteam@group.calendar.google.com",
		"startTime:",
		"09:00",
		"endTime:",
		"18:00",
	}
	for _, want := range checks {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("ToYAML() output missing %q\ngot:\n%s", want, yamlStr)
		}
	}
}
