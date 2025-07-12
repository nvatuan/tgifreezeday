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
	repo, err := googlecalendar.NewRepository(ctx, credPath, cfg.ReadFrom.GoogleCalendar.CountryCode)
	if err != nil {
		log.Fatalf("Failed to create Google Calendar repository: %v", err)
	}

	// Test with current date (or a specific date for testing)
	testDate := time.Date(2025, 7, 11, 0, 0, 0, 0, time.Local) // July 11, 2025
	fmt.Printf("Testing with date anchor: %s\n", testDate.Format("2006-01-02"))
	fmt.Printf("Holiday Calendar Country: %s\n", cfg.ReadFrom.GoogleCalendar.CountryCode)

	// Get month calendar
	monthCalendar, err := repo.GetMonthCalendar(testDate)
	if err != nil {
		log.Fatalf("Failed to get month calendar: %v", err)
	}

	// Print results
	printMonthCalendar(monthCalendar)

	fmt.Println("tgifreezeday: completed successfully")
}

func printMonthCalendar(mc *domain.TGIFMonthCalendar) {
	fmt.Printf("Month: %s\n", mc.Month.String())
	fmt.Printf("Total days: %d\n", len(mc.Days))

	if mc.FirstBusinessDay != nil {
		fmt.Printf("First business day: %s (day %d)\n",
			mc.FirstBusinessDay.Date.Format("2006-01-02"),
			mc.FirstBusinessDay.Date.Day())
	} else {
		fmt.Println("First business day: None")
	}

	if mc.LastBusinessDay != nil {
		fmt.Printf("Last business day: %s (day %d)\n",
			mc.LastBusinessDay.Date.Format("2006-01-02"),
			mc.LastBusinessDay.Date.Day())
	} else {
		fmt.Println("Last business day: None")
	}

	fmt.Println("\nDay-by-day breakdown:")
	fmt.Println("Date       | Weekday   | Holiday | Weekend | Business | Non-Business")
	fmt.Println(strings.Repeat("-", 70))

	for _, day := range mc.Days {
		fmt.Printf("%s | %-9s | %-7t | %-7t | %-8t | %-12t\n",
			day.Date.Format("2006-01-02"),
			day.Date.Weekday().String(),
			day.IsHoliday,
			day.IsWeekend,
			day.IsBusinessDay,
			day.IsNonBusinessDay)
	}
}
