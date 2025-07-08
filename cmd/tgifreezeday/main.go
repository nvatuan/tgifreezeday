package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/nvat/tgifreezeday/internal/calendar"
	"github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/events"
	"github.com/nvat/tgifreezeday/internal/freeze"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Google Calendar client
	credentialsPath := config.GetCredentialsPath()
	calendarClient, err := calendar.NewGoogleCalendarClient(ctx, credentialsPath)
	if err != nil {
		log.Fatalf("Failed to create calendar client: %v", err)
	}

	// Create business day checker
	businessDayChecker := calendar.NewBusinessDayChecker(calendarClient)

	// Create freeze checker
	freezeChecker := freeze.NewFreezeChecker(businessDayChecker)

	// Create event manager
	eventManager := events.NewEventManager(calendarClient)

	// Get window days from environment (default to 7)
	windowDays := 7
	if windowEnv := os.Getenv("WINDOW_DAYS"); windowEnv != "" {
		if w, err := strconv.Atoi(windowEnv); err == nil {
			windowDays = w
		}
	}

	// Clean up old events first
	if err := eventManager.CleanupOldEvents(cfg, windowDays); err != nil {
		log.Printf("Warning: failed to cleanup old events: %v", err)
	}

	// Check freeze days for the next windowDays
	now := time.Now()
	for i := 0; i < windowDays; i++ {
		checkDate := now.AddDate(0, 0, i)

		// Check if this date is a freeze day
		isFreeze, err := freezeChecker.IsFreezeDay(cfg, checkDate)
		if err != nil {
			log.Printf("Error checking freeze day for %s: %v", checkDate.Format("2006-01-02"), err)
			continue
		}

		if isFreeze {
			// Check if event already exists
			hasEvent, err := eventManager.HasFreezeEventForDate(cfg, checkDate)
			if err != nil {
				log.Printf("Error checking existing event for %s: %v", checkDate.Format("2006-01-02"), err)
				continue
			}

			if !hasEvent {
				// Create freeze event
				if err := eventManager.CreateFreezeEvent(cfg, checkDate); err != nil {
					log.Printf("Error creating freeze event for %s: %v", checkDate.Format("2006-01-02"), err)
					continue
				}
				fmt.Printf("Created freeze event for %s\n", checkDate.Format("2006-01-02"))
			} else {
				fmt.Printf("Freeze event already exists for %s\n", checkDate.Format("2006-01-02"))
			}
		}
	}

	fmt.Println("tgifreezeday: completed successfully")
}
