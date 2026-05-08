package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	appconfig "github.com/nvat/tgifreezeday/internal/config"
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
	configs  *db.ConfigStore
	tokens   *db.TokenStore
	oauthCfg *oauth2.Config
}

func New(configs *db.ConfigStore, tokens *db.TokenStore, oauthCfg *oauth2.Config) *Scheduler {
	return &Scheduler{configs: configs, tokens: tokens, oauthCfg: oauthCfg}
}

// Start runs the scheduler loop. It advances any past-due configs on startup,
// then processes due configs every minute. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log := logging.GetLogger()
	if err := s.advancePastDue(); err != nil {
		log.WithError(err).Warn("scheduler: failed to advance past-due configs on startup")
	}

	ticker := time.NewTicker(time.Minute)
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

// advancePastDue moves next_sync_at forward for any configs that are overdue at startup,
// implementing the "missed windows are silently skipped" behaviour.
func (s *Scheduler) advancePastDue() error {
	now := time.Now().UTC()
	due, err := s.configs.ListDueForAutoSync(now)
	if err != nil {
		return err
	}
	log := logging.GetLogger()
	for _, cfg := range due {
		next := NextSyncAt(cfg.SyncSchedule, now)
		if err := s.configs.UpdateNextSyncAt(cfg.ID, next); err != nil {
			log.WithError(err).WithField("config_id", cfg.ID).Warn("scheduler: failed to advance past-due config")
		}
	}
	return nil
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	due, err := s.configs.ListDueForAutoSync(now.UTC())
	if err != nil {
		logging.GetLogger().WithError(err).Error("scheduler: failed to query due configs")
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
	repo, err := googlecalendar.NewRepositoryWithToken(ctx, s.oauthCfg, token,
		appCfg.ReadFrom.GoogleCalendar.CountryCode,
		appCfg.WriteTo.GoogleCalendar.ID,
	)
	if err != nil {
		return err.Error(), true
	}
	rangeStart, rangeEnd := syncDateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	tgifMapping, err := repo.GetFreezeDaysInRange(rangeStart, rangeEnd)
	if err != nil {
		return "failed to get freeze days: " + err.Error(), true
	}
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		return "failed to wipe existing blockers: " + err.Error(), true
	}
	summary := *appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary
	description := *appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Description
	count := 0
	for _, day := range *tgifMapping {
		if day.IsTodayFreezeDay(appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf) {
			if err := repo.WriteBlockerOnDate(day.Date, summary, description); err != nil {
				return fmt.Sprintf("failed to write blocker on %s: %s", day.Date.Format("2006-01-02"), err.Error()), true
			}
			count++
		}
	}
	return fmt.Sprintf("Sync complete. Created %d blocker event(s) across %d days checked.", count, len(*tgifMapping)), false
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
