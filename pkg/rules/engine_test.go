package rules

import (
	"testing"
	"time"
)

func TestWeekendRule(t *testing.T) {
	rule := NewWeekendRule()

	// Test with a Saturday
	saturday := time.Date(2023, 5, 6, 12, 0, 0, 0, time.UTC)
	if !rule.Apply(saturday) {
		t.Errorf("Expected Saturday to be a weekend, but it wasn't")
	}

	// Test with a Sunday
	sunday := time.Date(2023, 5, 7, 12, 0, 0, 0, time.UTC)
	if !rule.Apply(sunday) {
		t.Errorf("Expected Sunday to be a weekend, but it wasn't")
	}

	// Test with a weekday (Monday)
	monday := time.Date(2023, 5, 8, 12, 0, 0, 0, time.UTC)
	if rule.Apply(monday) {
		t.Errorf("Expected Monday to not be a weekend, but it was")
	}
}

func TestDayBeforeHolidayRule(t *testing.T) {
	rule := NewDayBeforeHolidayRule(1)

	// Add a holiday
	holiday := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC) // May 1st
	rule.AddHoliday(holiday, "Labor Day")

	// Test the day before the holiday
	dayBefore := time.Date(2023, 4, 30, 12, 0, 0, 0, time.UTC) // April 30th
	if !rule.Apply(dayBefore) {
		t.Errorf("Expected the day before a holiday to be a freeze day, but it wasn't")
	}

	// Test the holiday itself
	if rule.Apply(holiday) {
		t.Errorf("The holiday itself should not be affected by the day-before rule")
	}

	// Test a random day
	randomDay := time.Date(2023, 5, 15, 12, 0, 0, 0, time.UTC)
	if rule.Apply(randomDay) {
		t.Errorf("Expected a random day to not be a freeze day, but it was")
	}
}

func TestDayAfterHolidayRule(t *testing.T) {
	rule := NewDayAfterHolidayRule(1)

	// Add a holiday
	holiday := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC) // May 1st
	rule.AddHoliday(holiday, "Labor Day")

	// Test the day after the holiday
	dayAfter := time.Date(2023, 5, 2, 12, 0, 0, 0, time.UTC) // May 2nd
	if !rule.Apply(dayAfter) {
		t.Errorf("Expected the day after a holiday to be a freeze day, but it wasn't")
	}

	// Test the holiday itself
	if rule.Apply(holiday) {
		t.Errorf("The holiday itself should not be affected by the day-after rule")
	}

	// Test a random day
	randomDay := time.Date(2023, 5, 15, 12, 0, 0, 0, time.UTC)
	if rule.Apply(randomDay) {
		t.Errorf("Expected a random day to not be a freeze day, but it was")
	}
}

func TestRuleEngine(t *testing.T) {
	engine := NewEngine()

	// Add rules
	weekendRule := NewWeekendRule()
	beforeHolidayRule := NewDayBeforeHolidayRule(1)

	// Add a holiday
	holiday := time.Date(2023, 5, 2, 12, 0, 0, 0, time.UTC) // May 2nd (Tuesday)
	beforeHolidayRule.AddHoliday(holiday, "Holiday")

	engine.AddRule(weekendRule)
	engine.AddRule(beforeHolidayRule)

	// Test a weekend
	saturday := time.Date(2023, 5, 6, 12, 0, 0, 0, time.UTC)
	isFreezeDay, reason := engine.IsFreezePeriod(saturday)
	if !isFreezeDay {
		t.Errorf("Expected Saturday to be a freeze day, but it wasn't")
	}
	if reason != "Weekend freeze day" {
		t.Errorf("Expected reason to be 'Weekend freeze day', but got '%s'", reason)
	}

	// Test the day before a holiday (Monday May 1st)
	dayBefore := time.Date(2023, 5, 1, 12, 0, 0, 0, time.UTC)
	isFreezeDay, reason = engine.IsFreezePeriod(dayBefore)
	if !isFreezeDay {
		t.Errorf("Expected day before holiday to be a freeze day, but it wasn't")
	}
	if reason != "Freeze 1 day(s) before holidays" {
		t.Errorf("Expected reason to be 'Freeze 1 day(s) before holidays', but got '%s'", reason)
	}

	// Test a regular day
	regularDay := time.Date(2023, 5, 4, 12, 0, 0, 0, time.UTC)
	isFreezeDay, _ = engine.IsFreezePeriod(regularDay)
	if isFreezeDay {
		t.Errorf("Expected a regular day to not be a freeze day, but it was")
	}
}
