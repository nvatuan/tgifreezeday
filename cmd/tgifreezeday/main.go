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
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "sync":
		syncCommand()
	case "wipe-blockers":
		wipeBlockersCommand()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: tgifreezeday <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  sync           Sync freeze day blockers to calendar")
	fmt.Println("  wipe-blockers  Remove all existing blockers in range (specified by shared.lookbackDays/lookaheadDays in config.yaml)")
}

func syncCommand() {
	cfg, repo := setupConfigAndRepo()

	rangeStart := time.Now().UTC().AddDate(0, 0, -1*cfg.Shared.LookbackDays)
	rangeEnd := time.Now().UTC().AddDate(0, 0, cfg.Shared.LookaheadDays)

	log.Printf("tgifreezeday sync: fetching freeze days from %s to %s", rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"))

	tgifMapping, err := repo.GetFreezeDaysInRange(rangeStart, rangeEnd)
	if err != nil {
		log.Fatalf("Failed to get freeze days in range: %v", err)
	}

	log.Printf("tgifreezeday sync: got %d freeze days in range", len(*tgifMapping))
	debugTgifMapping(tgifMapping)

	log.Printf("tgifreezeday sync: wiping existing blockers in range")
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		log.Fatalf("Failed to wipe blockers: %v", err)
	}

	freezeDayCount := 0
	for _, day := range *tgifMapping {
		if day.IsTodayFreezeDay(cfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) {
			freezeDayCount++
			summary := *cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary
			log.Printf("tgifreezeday sync: creating blocker for freeze day %s", day.Date.Format("2006-01-02"))
			err := repo.WriteBlockerOnDate(day.Date, summary)
			if err != nil {
				log.Printf("tgifreezeday sync: failed to write blocker on date %s: %v", day.Date.Format("2006-01-02"), err)
			}
		}
	}

	log.Printf("tgifreezeday sync: created %d freeze day blockers", freezeDayCount)
	fmt.Println("tgifreezeday sync: completed successfully")
}

func wipeBlockersCommand() {
	cfg, repo := setupConfigAndRepo()

	rangeStart := time.Now().UTC().AddDate(0, 0, -1*cfg.Shared.LookbackDays)
	rangeEnd := time.Now().UTC().AddDate(0, 0, cfg.Shared.LookaheadDays)

	log.Printf("tgifreezeday wipe-blockers: removing all blockers from %s to %s", rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"))

	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		log.Fatalf("Failed to wipe blockers: %v", err)
	}

	fmt.Println("tgifreezeday wipe-blockers: completed successfully")
}

func setupConfigAndRepo() (*config.Config, *googlecalendar.Repository) {
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

	return cfg, repo
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
