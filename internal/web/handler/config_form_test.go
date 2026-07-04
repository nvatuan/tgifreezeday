package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	appconfig "github.com/nvat/tgifreezeday/internal/config"
)

const (
	formKeyLookback  = "lookback_days"
	formKeyLookahead = "lookahead_days"
	formKeyRulesJSON = "rules_json"
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
		{Anchor: ruleAnchorToday, Conditions: []string{"isTheFirstBusinessDayOfTheMonth", "isTheLastBusinessDayOfTheMonth"}},
		{Anchor: ruleAnchorTomorrow, Conditions: []string{"isNonBusinessDay"}},
	}
	form := url.Values{
		formKeyLookback:     {"20"},
		formKeyLookahead:    {"60"},
		"country_code":      {countryCodeJPN},
		"write_calendar_id": {"team-cal@group.calendar.google.com"},
		"event_summary":     {"🚫 PRODUCTION FREEZE"},
		"event_description": {"No prod ops today."},
		"start_time":        {"08:00"},
		"end_time":          {"20:00"},
		formKeyRulesJSON:    {rulesJSON(t, rules)},
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
	if cfg.ReadFrom.GoogleCalendar.CountryCode != countryCodeJPN {
		t.Errorf("CountryCode = %q, want %s", cfg.ReadFrom.GoogleCalendar.CountryCode, countryCodeJPN)
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
		{Anchor: ruleAnchorToday, Conditions: []string{"isTheLastBusinessDayOfTheMonth"}},
	}
	form := url.Values{
		formKeyLookback:     {"30"},
		formKeyLookahead:    {"45"},
		"country_code":      {"vnm"},
		"write_calendar_id": {"myteam@group.calendar.google.com"},
		"event_summary":     {"Freeze"},
		"event_description": {"No deployments."},
		"start_time":        {"09:00"},
		"end_time":          {"18:00"},
		formKeyRulesJSON:    {rulesJSON(t, rules)},
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
		formKeyLookback:     {"20"},
		formKeyLookahead:    {"60"},
		"country_code":      {countryCodeJPN},
		"write_calendar_id": {"cal@group.calendar.google.com"},
		"event_summary":     {"Freeze"},
		"event_description": {"No ops"},
		"start_time":        {"08:00"},
		"end_time":          {"20:00"},
		formKeyRulesJSON:    {""},
	}
	r := makeFormRequest(form)
	_, err := formToAppConfig(r)
	if err == nil {
		t.Error("formToAppConfig() expected error for empty rules_json, got nil")
	}
}

func TestFormToAppConfig_AllDay(t *testing.T) {
	rules := []formRule{
		{Anchor: ruleAnchorToday, Conditions: []string{"isTheFirstBusinessDayOfTheMonth"}},
	}
	form := url.Values{
		formKeyLookback:     {"20"},
		formKeyLookahead:    {"60"},
		"country_code":      {countryCodeJPN},
		"write_calendar_id": {"team-cal@group.calendar.google.com"},
		"event_summary":     {"🚫 Freeze"},
		"event_description": {"No prod."},
		"all_day":           {"on"},
		// start_time / end_time intentionally omitted (hidden when all_day checked)
		formKeyRulesJSON: {rulesJSON(t, rules)},
	}
	r := makeFormRequest(form)

	cfg, err := formToAppConfig(r)
	if err != nil {
		t.Fatalf("formToAppConfig() error = %v", err)
	}

	d := cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default
	if d.AllDay == nil || !*d.AllDay {
		t.Error("AllDay should be true")
	}
	// StartTime and EndTime should remain nil (not set by SetDefault when allDay=true).
	if d.StartTime != nil {
		t.Errorf("StartTime should be nil for all-day, got %q", *d.StartTime)
	}
	if d.EndTime != nil {
		t.Errorf("EndTime should be nil for all-day, got %q", *d.EndTime)
	}

	// Should validate without error
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFormToAppConfig_InvalidLookback(t *testing.T) {
	form := url.Values{
		formKeyLookback:  {"notanumber"},
		formKeyLookahead: {"60"},
		formKeyRulesJSON: {`[{"anchor":"today","conditions":["isNonBusinessDay"]}]`},
	}
	r := makeFormRequest(form)
	_, err := formToAppConfig(r)
	if err == nil {
		t.Error("formToAppConfig() expected error for invalid lookback_days, got nil")
	}
}
