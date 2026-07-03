package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	appconfig "github.com/nvat/tgifreezeday/internal/config"
)

// makeFormRequest builds a POST *http.Request with the given form values.
func makeFormRequest(values url.Values) *http.Request {
	r, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	return r
}

func rulesJSON(t *testing.T, rules []formRule) string {
	t.Helper()
	b, err := json.Marshal(rules)
	if err != nil {
		t.Fatalf("json.Marshal rules: %v", err)
	}
	return string(b)
}

func TestFormToAppConfig_ValidInput(t *testing.T) {
	rules := []formRule{
		{Anchor: "today", Conditions: []string{"isTheFirstBusinessDayOfTheMonth", "isTheLastBusinessDayOfTheMonth"}},
		{Anchor: "tomorrow", Conditions: []string{"isNonBusinessDay"}},
	}
	form := url.Values{
		"lookback_days":     {"20"},
		"lookahead_days":    {"60"},
		"country_code":      {countryCodeJPN},
		"write_calendar_id": {"team-cal@group.calendar.google.com"},
		"event_summary":     {"🚫 PRODUCTION FREEZE"},
		"event_description": {"No prod ops today."},
		"start_time":        {"08:00"},
		"end_time":          {"20:00"},
		"rules_json":        {rulesJSON(t, rules)},
	}
	r := makeFormRequest(form)

	cfg, err := formToAppConfig(r)
	if err != nil {
		t.Fatalf("formToAppConfig() error = %v", err)
	}

	if cfg.Shared.LookbackDays != 20 {
		t.Errorf("LookbackDays = %d, want 20", cfg.Shared.LookbackDays)
	}
	if cfg.Shared.LookaheadDays != 60 {
		t.Errorf("LookaheadDays = %d, want 60", cfg.Shared.LookaheadDays)
	}
	if cfg.ReadFrom.GoogleCalendar.CountryCode != "jpn" {
		t.Errorf("CountryCode = %q, want jpn", cfg.ReadFrom.GoogleCalendar.CountryCode)
	}
	if cfg.WriteTo.GoogleCalendar.ID != "team-cal@group.calendar.google.com" {
		t.Errorf("CalendarID = %q", cfg.WriteTo.GoogleCalendar.ID)
	}
	if got := cfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf; len(got) != 2 {
		t.Errorf("len(Rules) = %d, want 2", len(got))
	}
	if cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary == nil ||
		*cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary != "🚫 PRODUCTION FREEZE" {
		t.Errorf("Summary mismatch")
	}
}

func TestFormToAppConfig_RoundTrip(t *testing.T) {
	rules := []formRule{
		{Anchor: "today", Conditions: []string{"isTheLastBusinessDayOfTheMonth"}},
	}
	form := url.Values{
		"lookback_days":     {"30"},
		"lookahead_days":    {"45"},
		"country_code":      {"vnm"},
		"write_calendar_id": {"myteam@group.calendar.google.com"},
		"event_summary":     {"Freeze"},
		"event_description": {"No deployments."},
		"start_time":        {"09:00"},
		"end_time":          {"18:00"},
		"rules_json":        {rulesJSON(t, rules)},
	}
	r := makeFormRequest(form)

	cfg, err := formToAppConfig(r)
	if err != nil {
		t.Fatalf("formToAppConfig() error = %v", err)
	}

	yamlStr, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}
	if yamlStr == "" {
		t.Fatal("ToYAML() returned empty string")
	}

	// Re-parse and validate
	parsed, err := appconfig.LoadWithDefaultFromByteArray([]byte(yamlStr))
	if err != nil {
		t.Fatalf("LoadWithDefaultFromByteArray() error = %v", err)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatalf("Validate() after round-trip error = %v", err)
	}
	if parsed.Shared.LookbackDays != 30 {
		t.Errorf("LookbackDays after round-trip = %d, want 30", parsed.Shared.LookbackDays)
	}
}

func TestFormToAppConfig_MissingRules(t *testing.T) {
	form := url.Values{
		"lookback_days":     {"20"},
		"lookahead_days":    {"60"},
		"country_code":      {"jpn"},
		"write_calendar_id": {"cal@group.calendar.google.com"},
		"event_summary":     {"Freeze"},
		"event_description": {"No ops"},
		"start_time":        {"08:00"},
		"end_time":          {"20:00"},
		"rules_json":        {""},
	}
	r := makeFormRequest(form)
	_, err := formToAppConfig(r)
	if err == nil {
		t.Error("formToAppConfig() expected error for empty rules_json, got nil")
	}
}

func TestFormToAppConfig_InvalidLookback(t *testing.T) {
	form := url.Values{
		"lookback_days":  {"notanumber"},
		"lookahead_days": {"60"},
		"rules_json":     {`[{"anchor":"today","conditions":["isNonBusinessDay"]}]`},
	}
	r := makeFormRequest(form)
	_, err := formToAppConfig(r)
	if err == nil {
		t.Error("formToAppConfig() expected error for invalid lookback_days, got nil")
	}
}
