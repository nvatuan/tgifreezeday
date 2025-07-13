package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/domain"
)

func main() {
	// Load configuration
	cfg, err := config.LoadWithDefault()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	// Get Google credentials path
	credPath := os.Getenv(config.GoogleAppClientCredJSONPathEnv)

	// Create Google Calendar repository
	ctx := context.Background()
	repo, err := googlecalendar.NewRepository(ctx,
		credPath,
		cfg.ReadFrom.GoogleCalendar.CountryCode,
		cfg.WriteTo.GoogleCalendar.ID,
	)
	if err != nil {
		log.Fatalf("Failed to create Google Calendar repository: %v", err)
	}

	rangeStart := time.Now().UTC().AddDate(0, 0, -1*cfg.ReadFrom.GoogleCalendar.LookbackDays)
	rangeEnd := time.Now().UTC().AddDate(0, 0, cfg.ReadFrom.GoogleCalendar.LookaheadDays)

	log.Printf("tgifreezeday: fetching freeze days from %s to %s", rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"))

	tgifMapping, err := repo.GetFreezeDaysInRange(rangeStart, rangeEnd)
	if err != nil {
		log.Fatalf("Failed to get freeze days in range: %v", err)
	}

	log.Printf("tgifreezeday: got %d freeze days in range", len(*tgifMapping))
	debugTgifMapping(tgifMapping)

	log.Printf("tgifreezeday: wiping existing blockers in range")
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		log.Fatalf("Failed to wipe blockers: %v", err)
	}

	freezeDayCount := 0
	for _, day := range *tgifMapping {
		if day.IsTodayFreezeDay(cfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) {
			freezeDayCount++
			summary := *cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary
			log.Printf("tgifreezeday: creating blocker for freeze day %s", day.Date.Format("2006-01-02"))
			err := repo.WriteBlockerOnDate(day.Date, summary)
			if err != nil {
				log.Printf("tgifreezeday: failed to write blocker on date %s: %v", day.Date.Format("2006-01-02"), err)
			}
		}
	}

	log.Printf("tgifreezeday: created %d freeze day blockers", freezeDayCount)
	fmt.Println("tgifreezeday: completed successfully")
}

func debugTgifMapping(tgifMapping *domain.TGIFMapping) {
	if tgifMapping == nil {
		log.Printf("DEBUG: tgifMapping is nil")
		return
	}

	log.Printf("DEBUG: TGIFMapping contains %d days", len(*tgifMapping))

	// Group days by month for organized output
	monthGroups := make(map[string][]*domain.TGIFDay)
	for _, day := range *tgifMapping {
		monthKey := day.Date.Format("2006-01")
		monthGroups[monthKey] = append(monthGroups[monthKey], day)
	}

	for monthKey, days := range monthGroups {
		log.Printf("DEBUG: Month %s:", monthKey)

		// Sort days by date
		for i := range days {
			for j := i + 1; j < len(days); j++ {
				if days[i].Date.After(days[j].Date) {
					days[i], days[j] = days[j], days[i]
				}
			}
		}

		for _, day := range days {
			flags := []string{}
			if day.IsHoliday {
				flags = append(flags, "Holiday")
			}
			if day.IsWeekend {
				flags = append(flags, "Weekend")
			}
			if day.IsBusinessDay {
				flags = append(flags, "BusinessDay")
			}
			if day.IsFirstBusinessDayOfMonth != nil && *day.IsFirstBusinessDayOfMonth {
				flags = append(flags, "FirstBusinessDay")
			}
			if day.IsLastBusinessDayOfMonth != nil && *day.IsLastBusinessDayOfMonth {
				flags = append(flags, "LastBusinessDay")
			}

			flagStr := strings.Join(flags, ", ")
			if flagStr == "" {
				flagStr = "None"
			}

			log.Printf("  %s (%s): %s", day.Date.Format("2006-01-02"), day.Date.Weekday().String(), flagStr)
		}
	}
}
