package services

import (
	"context"
	"time"

	"github.com/nvat/tgifreezeday/internal/core/domain"
	"github.com/nvat/tgifreezeday/internal/core/ports"
	"github.com/nvat/tgifreezeday/pkg/rules"
)

// FreezeDayService orchestrates the freeze day logic
type FreezeDayService struct {
	dataSource   ports.DataSource
	destinations []ports.Destination
	ruleEngine   *rules.Engine
	config       *Config
}

// Config holds service configuration
type Config struct {
	DaysBeforeHoliday int
	DaysAfterHoliday  int
	DaysToLookAhead   int
}

// NewFreezeDayService creates a new freeze day service
func NewFreezeDayService(
	dataSource ports.DataSource,
	destinations []ports.Destination,
	config *Config,
) *FreezeDayService {
	engine := rules.NewEngine()

	return &FreezeDayService{
		dataSource:   dataSource,
		destinations: destinations,
		ruleEngine:   engine,
		config:       config,
	}
}

// CalculateFreezePeriods determines freeze periods based on holidays and rules
func (s *FreezeDayService) CalculateFreezePeriods(ctx context.Context) ([]domain.FreezePeriod, error) {
	now := time.Now()
	endDate := now.AddDate(0, 0, s.config.DaysToLookAhead)

	// Get holidays from data source
	holidays, err := s.dataSource.ListHolidays(ctx, now, endDate)
	if err != nil {
		return nil, err
	}

	// Configure rules
	beforeHolidayRule := rules.NewDayBeforeHolidayRule(s.config.DaysBeforeHoliday)
	afterHolidayRule := rules.NewDayAfterHolidayRule(s.config.DaysAfterHoliday)
	weekendRule := rules.NewWeekendRule()

	// Add rules to engine
	s.ruleEngine.AddRule(beforeHolidayRule)
	s.ruleEngine.AddRule(afterHolidayRule)
	s.ruleEngine.AddRule(weekendRule)

	// Add holidays to rules
	for _, holiday := range holidays {
		beforeHolidayRule.AddHoliday(holiday.StartTime, holiday.Title)
		afterHolidayRule.AddHoliday(holiday.StartTime, holiday.Title)
	}

	// Calculate freeze periods
	var periods []domain.FreezePeriod

	// For each day in the look-ahead period
	for d := 0; d <= s.config.DaysToLookAhead; d++ {
		checkDate := now.AddDate(0, 0, d)
		isFreezeDay, reason := s.ruleEngine.IsFreezePeriod(checkDate)

		if isFreezeDay {
			// Create 8am-6pm freeze period
			startTime := time.Date(
				checkDate.Year(), checkDate.Month(), checkDate.Day(),
				8, 0, 0, 0, checkDate.Location(),
			)
			endTime := time.Date(
				checkDate.Year(), checkDate.Month(), checkDate.Day(),
				18, 0, 0, 0, checkDate.Location(),
			)

			periods = append(periods, domain.FreezePeriod{
				StartDate:       startTime,
				EndDate:         endTime,
				Description:     "Production Release Forbidden: " + reason,
				RelatedHolidays: holidays,
			})
		}
	}

	return periods, nil
}

// UpdateFreezePeriods updates freeze periods in all destinations
func (s *FreezeDayService) UpdateFreezePeriods(ctx context.Context) error {
	periods, err := s.CalculateFreezePeriods(ctx)
	if err != nil {
		return err
	}

	// Update all destinations
	for _, dest := range s.destinations {
		if err := dest.UpdateFreezePeriods(ctx, periods); err != nil {
			return err
		}
	}

	return nil
}

// IsFreezeDayToday checks if today is a freeze day
func (s *FreezeDayService) IsFreezeDayToday(ctx context.Context) (bool, string, error) {
	today := time.Now()

	// Check using rule engine
	isFreezeDay, reason := s.ruleEngine.IsFreezePeriod(today)
	return isFreezeDay, reason, nil
}
