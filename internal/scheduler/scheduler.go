package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	appconfig "github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/domain"
	"github.com/nvat/tgifreezeday/internal/logging"
	"golang.org/x/oauth2"
)

// JST is the fixed UTC+9 timezone used for all schedule calculations.
// Exported so callers (e.g. the UI layer) can format timestamps consistently
// without duplicating the timezone definition.
// JST has no DST, so +24h arithmetic is always safe here.
var JST = time.FixedZone("JST", 9*60*60)

// NextSyncAt returns the next scheduled time strictly after `from` for the given schedule.
// weekly  → every Monday 09:00 JST
// monthly → 1st of every month 09:00 JST
func NextSyncAt(schedule string, from time.Time) time.Time {
	fromJST := from.In(JST)
	switch schedule {
	case db.SyncScheduleWeekly:
		target := time.Date(fromJST.Year(), fromJST.Month(), fromJST.Day(), 9, 0, 0, 0, JST)
		for target.Weekday() != time.Monday || !target.After(from) {
			target = target.Add(24 * time.Hour)
		}
		return target.UTC()
	case db.SyncScheduleMonthly:
		target := time.Date(fromJST.Year(), fromJST.Month(), 1, 9, 0, 0, 0, JST)
		for !target.After(from) {
			target = time.Date(target.Year(), target.Month()+1, 1, 9, 0, 0, 0, JST)
		}
		return target.UTC()
	default:
		return time.Time{}
	}
}

type Scheduler struct {
	configs       *db.ConfigStore
	tokens        *db.TokenStore
	oauthCfg      *oauth2.Config
	tickerMinutes int
}

// New creates a Scheduler. tickerMinutes controls how often the scheduler polls
// for due configs; set via SCHED_TICKER_FREQUENCY_MIN (default 15, must be > 0).
func New(configs *db.ConfigStore, tokens *db.TokenStore, oauthCfg *oauth2.Config, tickerMinutes int) *Scheduler {
	return &Scheduler{
		configs:       configs,
		tokens:        tokens,
		oauthCfg:      oauthCfg,
		tickerMinutes: tickerMinutes,
	}
}

// Start runs the scheduler loop. On startup any configs whose next_sync_at is
// already in the past are picked up on the first tick and synced immediately.
// Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log := logging.GetLogger()
	log.WithField("ticker_minutes", s.tickerMinutes).Info("scheduler: starting")

	ticker := time.NewTicker(time.Duration(s.tickerMinutes) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			s.tick(ctx, t)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	log := logging.GetLogger()
	log.WithField("time", now.UTC().Format(time.RFC3339)).Debug("scheduler: tick")

	due, err := s.configs.ListDueForAutoSync(now.UTC())
	if err != nil {
		log.WithError(err).Error("scheduler: failed to query due configs")
		return
	}
	for _, cfg := range due {
		s.syncConfig(ctx, cfg)
	}
}

func (s *Scheduler) syncConfig(ctx context.Context, cfg *db.Config) {
	log := logging.GetLogger().WithField("config_id", cfg.ID)

	msg, isErr := s.runSync(ctx, cfg)
	// Capture time AFTER runSync so next_sync_at is computed from the actual
	// completion time, not the start time (avoids re-firing immediately when sync
	// takes long enough to straddle a schedule boundary).
	syncedAt := time.Now().UTC()

	prefix := "✅ "
	if isErr {
		prefix = "❌ "
	}
	resultMsg := prefix + msg

	next := NextSyncAt(cfg.SyncSchedule, syncedAt)
	if err := s.configs.RecordAutoSync(cfg.ID, syncedAt, resultMsg, next); err != nil {
		log.WithError(err).Error("scheduler: failed to record auto-sync result")
	}
	log.WithField("result", resultMsg).Info("scheduler: auto-sync completed")
}

func (s *Scheduler) runSync(ctx context.Context, cfg *db.Config) (string, bool) {
	appCfg, err := parseAppConfig(cfg.ConfigYAML)
	if err != nil {
		return err.Error(), true
	}
	token, err := s.tokens.Get(cfg.UserID)
	if err != nil {
		return "failed to get owner token: " + err.Error(), true
	}
	if token == nil {
		return "no OAuth token for config owner — owner must log in", true
	}
	repo, err := googlecalendar.NewRepositoryWithToken(ctx, s.oauthCfg, token, cfg.UserID, s.tokens,
		appCfg.ReadFrom.GoogleCalendar.CountryCode,
		appCfg.WriteTo.GoogleCalendar.ID,
	)
	if err != nil {
		return err.Error(), true
	}
	rangeStart, rangeEnd := syncDateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	return domain.RunSync(
		repo,
		rangeStart, rangeEnd,
		domain.TodayIsFreezeDayIf(appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf),
		*appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary,
		*appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Description,
	)
}

func parseAppConfig(yamlContent string) (*appconfig.Config, error) {
	cfg, err := appconfig.LoadWithDefaultFromByteArray([]byte(yamlContent))
	if err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	return cfg, nil
}

func syncDateRange(lookbackDays, lookaheadDays int) (time.Time, time.Time) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -lookbackDays), today.AddDate(0, 0, lookaheadDays)
}
