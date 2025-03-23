package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nvat/tgifreezeday/internal/adapters/datasource"
	"github.com/nvat/tgifreezeday/internal/adapters/destination"
	"github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/core/ports"
	"github.com/nvat/tgifreezeday/internal/core/services"
)

var (
	configFile        = flag.String("config", "tgifreezeday.yaml", "Path to configuration file")
	checkUpcomingDays = flag.Int("check-upcoming", 7, "Number of days to check for upcoming freeze days")
)

func main() {
	flag.Parse()

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize data source
	var dataSource ports.DataSource
	if cfg.DataSource.Type == "google_calendar" {
		gcConfig, err := cfg.DataSource.ParseGoogleCalendarSourceConfig()
		if err != nil {
			log.Fatalf("Failed to parse Google Calendar source config: %v", err)
		}

		dataSource, err = datasource.NewGoogleCalendarDataSource(
			gcConfig.CredentialsFile,
			gcConfig.CalendarID,
		)
		if err != nil {
			log.Fatalf("Failed to initialize Google Calendar data source: %v", err)
		}
	} else {
		log.Fatalf("Unsupported data source type: %s", cfg.DataSource.Type)
	}

	// Initialize destinations
	var destinations []ports.Destination

	// Google Calendar destination
	if cfg.Destination.Type == "google_calendar" {
		gcConfig, err := cfg.Destination.ParseGoogleCalendarDestinationConfig()
		if err != nil {
			log.Fatalf("Failed to parse Google Calendar destination config: %v", err)
		}

		calDest, err := destination.NewGoogleCalendarDestination(
			gcConfig.CredentialsFile,
			gcConfig.CalendarID,
		)
		if err != nil {
			log.Fatalf("Failed to initialize Google Calendar destination: %v", err)
		}

		destinations = append(destinations, calDest)
	} else if cfg.Destination.Type == "slack" {
		slackConfig, err := cfg.Destination.ParseSlackDestinationConfig()
		if err != nil {
			log.Fatalf("Failed to parse Slack destination config: %v", err)
		}

		slackDest, err := destination.NewSlackDestination(
			slackConfig.Token,
			slackConfig.ChannelID,
		)
		if err != nil {
			log.Fatalf("Failed to initialize Slack destination: %v", err)
		}

		destinations = append(destinations, slackDest)
	} else {
		log.Fatalf("Unsupported destination type: %s", cfg.Destination.Type)
	}

	// Get rule configuration
	rulesConfig := cfg.DataSource.GetRulesConfig()

	// Create and configure the service
	service := services.NewFreezeDayService(
		dataSource,
		destinations,
		&services.Config{
			DaysBeforeHoliday: rulesConfig.DaysBeforeHoliday,
			DaysAfterHoliday:  rulesConfig.DaysAfterHoliday,
			DaysToLookAhead:   *daysToLookup, // Override with command-line parameter if provided
		},
	)

	// Default action: update freeze periods in all destinations
	err = service.UpdateFreezePeriods(ctx)
	if err != nil {
		log.Fatalf("Error updating freeze periods: %v", err)
	}

	fmt.Println("Successfully updated freeze periods")
}
