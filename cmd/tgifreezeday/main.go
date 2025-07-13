package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/domain"
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

func main() {
	// Setup logger
	logger = logging.GetLogger()

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
		logger.WithField("command", command).Error("Unknown command")
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

	logger.WithFields(logrus.Fields{
		"command":   "sync",
		"dateRange": fmt.Sprintf("%s to %s", rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02")),
		"lookback":  cfg.Shared.LookbackDays,
		"lookahead": cfg.Shared.LookaheadDays,
	}).Info("Fetching freeze days")

	tgifMapping, err := repo.GetFreezeDaysInRange(rangeStart, rangeEnd)
	if err != nil {
		logger.WithError(err).Fatal("Failed to get freeze days in range")
	}

	logger.WithFields(logrus.Fields{
		"command":   "sync",
		"daysFound": len(*tgifMapping),
	}).Info("Retrieved freeze days")

	debugTgifMapping(tgifMapping)

	logger.WithField("command", "sync").Info("Wiping existing blockers in range")
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		logger.WithError(err).Fatal("Failed to wipe blockers")
	}

	freezeDayCount := 0
	for _, day := range *tgifMapping {
		if day.IsTodayFreezeDay(cfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) {
			freezeDayCount++
			summary := *cfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary

			logger.WithFields(logrus.Fields{
				"command": "sync",
				"date":    day.Date.Format("2006-01-02"),
				"summary": summary,
			}).Info("Creating blocker for freeze day")

			err := repo.WriteBlockerOnDate(day.Date, summary)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"command": "sync",
					"date":    day.Date.Format("2006-01-02"),
				}).WithError(err).Error("Failed to write blocker")
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"command":          "sync",
		"blockersCreated":  freezeDayCount,
		"totalDaysChecked": len(*tgifMapping),
	}).Info("Sync completed successfully")
}

func wipeBlockersCommand() {
	cfg, repo := setupConfigAndRepo()

	rangeStart := time.Now().UTC().AddDate(0, 0, -1*cfg.Shared.LookbackDays)
	rangeEnd := time.Now().UTC().AddDate(0, 0, cfg.Shared.LookaheadDays)

	logger.WithFields(logrus.Fields{
		"command":   "wipe-blockers",
		"dateRange": fmt.Sprintf("%s to %s", rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02")),
		"lookback":  cfg.Shared.LookbackDays,
		"lookahead": cfg.Shared.LookaheadDays,
	}).Info("Removing all blockers")

	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		logger.WithError(err).Fatal("Failed to wipe blockers")
	}

	logger.WithField("command", "wipe-blockers").Info("Wipe completed successfully")
}

func setupConfigAndRepo() (*config.Config, *googlecalendar.Repository) {
	// Load configuration
	cfg, err := config.LoadWithDefault()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load config")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.WithError(err).Fatal("Config validation failed")
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
		logger.WithError(err).Fatal("Failed to create Google Calendar repository")
	}

	return cfg, repo
}

func debugTgifMapping(tgifMapping *domain.TGIFMapping) {
	if tgifMapping == nil {
		logger.Debug("TGIFMapping is nil")
		return
	}

	logger.WithField("totalDays", len(*tgifMapping)).Debug("TGIFMapping contents")

	// Group days by month for organized output
	monthGroups := make(map[string][]*domain.TGIFDay)
	for _, day := range *tgifMapping {
		monthKey := day.Date.Format("2006-01")
		monthGroups[monthKey] = append(monthGroups[monthKey], day)
	}

	for monthKey, days := range monthGroups {
		logger.WithField("month", monthKey).Debug("Processing month")

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

			logger.WithFields(logrus.Fields{
				"date":    day.Date.Format("2006-01-02"),
				"weekday": day.Date.Weekday().String(),
				"flags":   flagStr,
			}).Debug("Day details")
		}
	}
}
