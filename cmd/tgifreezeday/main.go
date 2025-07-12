package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/config"
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

	today := time.Now().UTC()
	monthAhead := 1 // TODO: make this configurable
	for i := 0; i <= monthAhead; i++ {
		dateAnchor := today.AddDate(0, i, 0)
		log.Printf("tgifreezeday: hanlding month: %s", dateAnchor.Month().String())

		monthCalendar, err := repo.GetMonthCalendar(dateAnchor)
		if err != nil {
			log.Fatalf("Failed to get month calendar: %v", err)
		}

		repo.WipeAllBlockersInMonth(dateAnchor)
		for _, day := range monthCalendar.Days {
			if day.IsBusinessDay {
				repo.WriteBlockerOnDate(day.Date, "tgifreezeday: business day")
			}
		}
	}
	fmt.Println("tgifreezeday: completed successfully")
}
