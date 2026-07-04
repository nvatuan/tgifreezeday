package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	appconfig "github.com/nvat/tgifreezeday/internal/config"
	"github.com/nvat/tgifreezeday/internal/domain"
	"github.com/nvat/tgifreezeday/internal/helpers"
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/nvat/tgifreezeday/internal/perm"
	"github.com/nvat/tgifreezeday/internal/scheduler"
	"golang.org/x/oauth2"
	googleapi "google.golang.org/api/googleapi"
)

var log = logging.GetLogger()

// jstDisplay aliases scheduler.JST for formatting timestamps in the UI.
var jstDisplay = scheduler.JST

const (
	countryCodeJPN = "jpn"
	countryCodeVNM = "vnm"

	ruleAnchorToday    = "today"
	ruleAnchorTomorrow = "tomorrow"
)

type ConfigHandler struct {
	configs     *db.ConfigStore
	tokens      *db.TokenStore
	oauthCfg    *oauth2.Config
	validateSem chan struct{}
	basePath    string
}

func NewConfigHandler(configs *db.ConfigStore, tokens *db.TokenStore, oauthCfg *oauth2.Config, basePath string) *ConfigHandler {
	return &ConfigHandler{
		configs:     configs,
		tokens:      tokens,
		oauthCfg:    oauthCfg,
		validateSem: make(chan struct{}, 5),
		basePath:    basePath,
	}
}

func idFromPath(r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}

func (h *ConfigHandler) fetchCalendars(ctx context.Context, userID int64) []*googlecalendar.CalendarItem {
	token, err := h.getToken(userID)
	if err != nil {
		return nil
	}
	items, err := googlecalendar.ListWritableCalendars(ctx, h.oauthCfg, token, userID, h.tokens)
	if err != nil {
		log.WithError(err).Warn("failed to list writable calendars for form")
		return nil
	}
	return items
}

// HandleNew renders the config creation form.
func (h *ConfigHandler) HandleNew(w http.ResponseWriter, r *http.Request) {
	if role := roleFromContext(r.Context()); !role.CanCreate() {
		httpError(w, http.StatusForbidden, "you do not have permission to create configs")
		return
	}
	user := userFromContext(r.Context())
	cals := h.fetchCalendars(r.Context(), user.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configFormHTML("New Config", h.basePath+"/configs", h.basePath+"/dashboard", defaultFormData(), "", false, cals, h.basePath)) //nolint:errcheck
}

// HandleCreate processes the config creation form.
func (h *ConfigHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if role := roleFromContext(r.Context()); !role.CanCreate() {
		httpError(w, http.StatusForbidden, "you do not have permission to create configs")
		return
	}
	user := userFromContext(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form")
		return
	}
	name := r.FormValue("name")
	syncSchedule := parseSyncSchedule(r.FormValue("sync_schedule"))

	renderFormErr := func(formErr string) {
		cals := h.fetchCalendars(r.Context(), user.ID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fd := formDataFromRequest(r)
		fd.Name = name
		fd.SyncSchedule = syncSchedule
		fmt.Fprint(w, configFormHTML("New Config", h.basePath+"/configs", h.basePath+"/dashboard", fd, formErr, false, cals, h.basePath)) //nolint:errcheck
	}

	if name == "" {
		renderFormErr("Name is required.")
		return
	}

	appCfg, err := formToAppConfig(r)
	if err != nil {
		renderFormErr(err.Error())
		return
	}
	yamlContent, err := appCfg.ToYAML()
	if err != nil {
		renderFormErr("Failed to build config: " + err.Error())
		return
	}

	var nextSyncAt *time.Time
	if syncSchedule != db.SyncScheduleNone {
		t := scheduler.NextSyncAt(syncSchedule, time.Now())
		nextSyncAt = &t
	}

	cfg, err := h.configs.Create(user.ID, name, appconfig.CurrentSchemaVersion, yamlContent, syncSchedule, nextSyncAt)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to create config")
		return
	}

	go h.validateAndUpdateStatus(cfg.ID, user.ID, yamlContent)
	redirectTo(w, r, fmt.Sprintf(h.basePath+"/configs/%d", cfg.ID))
}

// HandleDetail renders the config detail page.
func (h *ConfigHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	role := roleFromContext(r.Context())

	// Parse once; resolve calendar name from the result.
	var parsedCfg *appconfig.Config
	calendarName := ""
	if appCfg, parseErr := appconfig.LoadWithDefaultFromByteArray([]byte(cfg.ConfigYAML)); parseErr == nil {
		parsedCfg = appCfg
		calID := appCfg.WriteTo.GoogleCalendar.ID
		for _, cal := range h.fetchCalendars(r.Context(), user.ID) {
			if cal.ID == calID {
				calendarName = cal.Summary
				break
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configDetailHTML(h.basePath, cfg, user.ID, role, parsedCfg, calendarName)) //nolint:errcheck
}

// HandleEdit renders the config edit form pre-populated.
func (h *ConfigHandler) HandleEdit(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanEditConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to edit this config")
		return
	}
	cals := h.fetchCalendars(r.Context(), user.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	action := fmt.Sprintf(h.basePath+"/configs/%d", id)
	backURL := fmt.Sprintf(h.basePath+"/configs/%d", id)

	// Parse existing YAML to pre-populate the structured form; fall back to defaults on error.
	var fd configFormData
	appCfg, parseErr := h.parseAppConfig(cfg.ConfigYAML)
	if parseErr != nil {
		fd = defaultFormData()
		fd.Name = cfg.Name
		fd.SyncSchedule = cfg.SyncSchedule
	} else {
		fd = configToFormData(cfg, appCfg)
	}
	fmt.Fprint(w, configFormHTML("Edit Config", action, backURL, fd, "", true, cals, h.basePath)) //nolint:errcheck
}

// HandleUpdate processes the config edit form.
func (h *ConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form")
		return
	}

	name := r.FormValue("name")
	syncSchedule := parseSyncSchedule(r.FormValue("sync_schedule"))

	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanEditConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to edit this config")
		return
	}

	action := fmt.Sprintf(h.basePath+"/configs/%d", id)
	backURL := fmt.Sprintf(h.basePath+"/configs/%d", id)

	renderFormErr := func(formErr string) {
		cals := h.fetchCalendars(r.Context(), user.ID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fd := formDataFromRequest(r)
		fd.Name = name
		fd.SyncSchedule = syncSchedule
		fmt.Fprint(w, configFormHTML("Edit Config", action, backURL, fd, formErr, true, cals, h.basePath)) //nolint:errcheck
	}

	if name == "" {
		renderFormErr("Name is required.")
		return
	}

	appCfg, err := formToAppConfig(r)
	if err != nil {
		renderFormErr(err.Error())
		return
	}
	yamlContent, err := appCfg.ToYAML()
	if err != nil {
		renderFormErr("Failed to build config: " + err.Error())
		return
	}

	nextSyncAt := computeNextSyncAt(cfg.SyncSchedule, cfg.NextSyncAt, syncSchedule)

	if err := h.configs.Update(id, cfg.UserID, name, yamlContent, syncSchedule, nextSyncAt); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to update config")
		return
	}
	go h.validateAndUpdateStatus(id, user.ID, yamlContent)
	redirectTo(w, r, fmt.Sprintf(h.basePath+"/configs/%d", id))
}

// HandleDelete deletes a config.
func (h *ConfigHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	h.doDelete(w, r, id, user.ID)
}

func (h *ConfigHandler) doDelete(w http.ResponseWriter, r *http.Request, id, userID int64) {
	cfg, err := h.getConfig(r.Context(), id, userID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanEditConfig(cfg.UserID, userID) {
		httpError(w, http.StatusForbidden, "you do not have permission to delete this config")
		return
	}
	if err := h.configs.Delete(id, cfg.UserID); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to delete config")
		return
	}
	redirectTo(w, r, h.basePath+"/dashboard")
}

// HandleValidate re-validates a config. Returns HTMX OOB response:
// - swaps #action-result with a "Validate finished: X → Y" message
// - OOB-swaps #status-badge with the updated badge
func (h *ConfigHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanSyncConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to validate this config")
		return
	}
	oldStatus := cfg.Status
	newStatus, msg := h.validateConfig(r.Context(), user.ID, cfg.ConfigYAML)
	if err := h.configs.UpdateStatus(id, newStatus, msg); err != nil {
		log.WithError(err).Error("failed to update config status after validate")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, validateResultHTML(oldStatus, newStatus, msg)) //nolint:errcheck
}

// HandleSync runs sync and returns result HTML (HTMX).
func (h *ConfigHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanSyncConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to sync this config")
		return
	}
	if cfg.SyncSchedule != db.SyncScheduleNone {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, actionResultHTML("Sync", "Manual operations are disabled while Auto-Sync is on. Disable Auto-Sync first if you want to sync or wipe manually.", true)) //nolint:errcheck
		return
	}
	msg, isErr := h.runSync(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, actionResultHTML("Sync", msg, isErr)) //nolint:errcheck
}

// HandleWipe wipes blockers and returns result HTML (HTMX).
func (h *ConfigHandler) HandleWipe(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanSyncConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to wipe this config")
		return
	}
	if cfg.SyncSchedule != db.SyncScheduleNone {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, actionResultHTML("Wipe", "Manual operations are disabled while Auto-Sync is on. Disable Auto-Sync first if you want to sync or wipe manually.", true)) //nolint:errcheck
		return
	}
	msg, isErr := h.runWipe(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, actionResultHTML("Wipe", msg, isErr)) //nolint:errcheck
}

// HandleUpdateAutoSync updates only the sync_schedule for a config. Returns HX-Redirect to the detail page.
func (h *ConfigHandler) HandleUpdateAutoSync(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10) // 1 KB — only one small field
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	if role := roleFromContext(r.Context()); !role.CanEditConfig(cfg.UserID, user.ID) {
		httpError(w, http.StatusForbidden, "you do not have permission to edit this config")
		return
	}
	newSchedule := parseSyncSchedule(r.FormValue("sync_schedule"))
	nextSyncAt := computeNextSyncAt(cfg.SyncSchedule, cfg.NextSyncAt, newSchedule)
	if err := h.configs.UpdateSyncSchedule(id, cfg.UserID, newSchedule, nextSyncAt); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to update auto-sync")
		return
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf(h.basePath+"/configs/%d", id))
	w.WriteHeader(http.StatusNoContent)
}

// HandleListBlockers returns a blockers table partial (HTMX).
func (h *ConfigHandler) HandleListBlockers(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.getConfig(r.Context(), id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	partial := h.listBlockers(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, partial) //nolint:errcheck,gosec
}

// --- internal helpers ---

func (h *ConfigHandler) getConfig(ctx context.Context, id, userID int64) (*db.Config, error) {
	if roleFromContext(ctx) == perm.RolePower {
		return h.configs.GetByID(id)
	}
	return h.configs.Get(id, userID)
}

func (h *ConfigHandler) getToken(userID int64) (*oauth2.Token, error) {
	token, err := h.tokens.Get(userID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, fmt.Errorf("no token found — please log in again")
	}
	return token, nil
}

func (h *ConfigHandler) parseAppConfig(yamlContent string) (*appconfig.Config, error) {
	cfg, err := appconfig.LoadWithDefaultFromByteArray([]byte(yamlContent))
	if err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	return cfg, nil
}

func (h *ConfigHandler) validateConfig(ctx context.Context, userID int64, yamlContent string) (db.ConfigStatus, string) {
	appCfg, err := h.parseAppConfig(yamlContent)
	if err != nil {
		return db.ConfigStatusInvalid, err.Error()
	}
	token, err := h.getToken(userID)
	if err != nil {
		return db.ConfigStatusInvalid, err.Error()
	}
	_, err = googlecalendar.NewRepositoryWithToken(ctx, h.oauthCfg, token, userID, h.tokens,
		appCfg.ReadFrom.GoogleCalendar.CountryCode,
		appCfg.WriteTo.GoogleCalendar.ID,
	)
	if err != nil {
		var gapiErr *googleapi.Error
		if errors.As(err, &gapiErr) && gapiErr.Code == http.StatusForbidden {
			return db.ConfigStatusUnauthorized, "no write permission on the target calendar"
		}
		return db.ConfigStatusInvalid, err.Error()
	}
	return db.ConfigStatusValid, ""
}

func (h *ConfigHandler) validateAndUpdateStatus(configID, userID int64, yamlContent string) {
	select {
	case h.validateSem <- struct{}{}:
		defer func() { <-h.validateSem }()
	default:
		log.WithField("config_id", configID).Warn("validation semaphore full, skipping background validation")
		return
	}
	status, msg := h.validateConfig(context.Background(), userID, yamlContent)
	if err := h.configs.UpdateStatus(configID, status, msg); err != nil {
		log.WithError(err).Error("failed to update config status after background validation")
	}
}

func (h *ConfigHandler) buildRepo(ctx context.Context, userID int64, cfg *appconfig.Config) (*googlecalendar.Repository, error) {
	token, err := h.getToken(userID)
	if err != nil {
		return nil, err
	}
	return googlecalendar.NewRepositoryWithToken(ctx, h.oauthCfg, token, userID, h.tokens,
		cfg.ReadFrom.GoogleCalendar.CountryCode,
		cfg.WriteTo.GoogleCalendar.ID,
	)
}

func (h *ConfigHandler) runSync(ctx context.Context, userID int64, cfg *db.Config) (string, bool) {
	appCfg, err := h.parseAppConfig(cfg.ConfigYAML)
	if err != nil {
		return err.Error(), true
	}
	repo, err := h.buildRepo(ctx, userID, appCfg)
	if err != nil {
		return err.Error(), true
	}
	d := appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default
	allDay := d.AllDay != nil && *d.AllDay
	startTime, endTime := "", ""
	if !allDay {
		startTime = *d.StartTime
		endTime = *d.EndTime
	}
	rangeStart, rangeEnd := dateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	return domain.RunSync(
		repo,
		rangeStart, rangeEnd,
		domain.TodayIsFreezeDayIf(appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf),
		*d.Summary,
		*d.Description,
		startTime,
		endTime,
		allDay,
	)
}

func (h *ConfigHandler) runWipe(ctx context.Context, userID int64, cfg *db.Config) (string, bool) {
	appCfg, err := h.parseAppConfig(cfg.ConfigYAML)
	if err != nil {
		return err.Error(), true
	}
	repo, err := h.buildRepo(ctx, userID, appCfg)
	if err != nil {
		return err.Error(), true
	}
	rangeStart, rangeEnd := dateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	if err := repo.WipeAllBlockersInRange(rangeStart, rangeEnd); err != nil {
		return "failed to wipe blockers: " + err.Error(), true
	}
	return "Wipe complete. All managed blockers removed in the date range.", false
}

type blockerItem struct {
	Date    string `json:"date"`
	Summary string `json:"summary"`
	ID      string `json:"id"`
}

// configFormData holds the structured fields for the config create/edit form.
type configFormData struct {
	Name          string
	LookbackDays  int
	LookaheadDays int
	CountryCode   string
	CalendarID    string
	Summary       string
	Description   string
	StartTime     string
	EndTime       string
	AllDay        bool
	SyncSchedule  string
	// Rules is the todayIsFreezeDayIf slice — each map has exactly one key (anchor) → conditions.
	Rules []map[string][]string
}

// formRule is the JSON shape used by the JS rules builder.
type formRule struct {
	Anchor     string   `json:"anchor"`
	Conditions []string `json:"conditions"`
}

func defaultFormData() configFormData {
	return configFormData{
		LookbackDays:  20,
		LookaheadDays: 60,
		CountryCode:   countryCodeJPN,
		Summary:       "🚫 PRODUCTION FREEZE - No Deployments",
		Description:   "Production operations restricted today.",
		StartTime:     "08:00",
		EndTime:       "20:00",
		AllDay:        false,
		SyncSchedule:  db.SyncScheduleNone,
		Rules: []map[string][]string{
			{ruleAnchorToday: {"isTheFirstBusinessDayOfTheMonth"}},
			{ruleAnchorToday: {"isTheLastBusinessDayOfTheMonth"}},
			{ruleAnchorTomorrow: {"isNonBusinessDay"}},
		},
	}
}

// configToFormData converts a stored config + its parsed appconfig into form data.
func configToFormData(cfg *db.Config, appCfg *appconfig.Config) configFormData {
	data := configFormData{
		Name:          cfg.Name,
		SyncSchedule:  cfg.SyncSchedule,
		LookbackDays:  appCfg.Shared.LookbackDays,
		LookaheadDays: appCfg.Shared.LookaheadDays,
		CountryCode:   appCfg.ReadFrom.GoogleCalendar.CountryCode,
		CalendarID:    appCfg.WriteTo.GoogleCalendar.ID,
		Rules:         appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf,
	}
	d := appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default
	if d.Summary != nil {
		data.Summary = *d.Summary
	}
	if d.Description != nil {
		data.Description = *d.Description
	}
	if d.StartTime != nil {
		data.StartTime = *d.StartTime
	}
	if d.EndTime != nil {
		data.EndTime = *d.EndTime
	}
	if d.AllDay != nil {
		data.AllDay = *d.AllDay
	}
	return data
}

// formToAppConfig builds a Config from the structured form POST fields.
func formToAppConfig(r *http.Request) (*appconfig.Config, error) {
	lookbackDays, err := strconv.Atoi(r.FormValue("lookback_days"))
	if err != nil {
		return nil, fmt.Errorf("invalid lookback_days: must be a number")
	}
	lookaheadDays, err := strconv.Atoi(r.FormValue("lookahead_days"))
	if err != nil {
		return nil, fmt.Errorf("invalid lookahead_days: must be a number")
	}

	rulesJSON := r.FormValue("rules_json")
	if rulesJSON == "" {
		return nil, fmt.Errorf("freeze rules are required")
	}
	var jsRules []formRule
	if err := json.Unmarshal([]byte(rulesJSON), &jsRules); err != nil {
		return nil, fmt.Errorf("invalid rules_json: %w", err)
	}
	if len(jsRules) == 0 {
		return nil, fmt.Errorf("at least one freeze rule group is required")
	}
	rules := make([]map[string][]string, 0, len(jsRules))
	for _, jr := range jsRules {
		if len(jr.Conditions) == 0 {
			return nil, fmt.Errorf("each rule group must have at least one condition")
		}
		rules = append(rules, map[string][]string{jr.Anchor: jr.Conditions})
	}

	summary := r.FormValue("event_summary")
	description := r.FormValue("event_description")
	calendarID := r.FormValue("write_calendar_id")
	countryCode := r.FormValue("country_code")
	allDay := r.FormValue("all_day") == "on"

	var allDayPtr *bool
	var startTimePtr, endTimePtr *string
	if allDay {
		allDayPtr = helpers.BoolPtr(true)
	} else {
		startTimePtr = helpers.StringPtr(r.FormValue("start_time"))
		endTimePtr = helpers.StringPtr(r.FormValue("end_time"))
	}

	cfg := &appconfig.Config{
		Shared: appconfig.SharedConfig{
			LookbackDays:  lookbackDays,
			LookaheadDays: lookaheadDays,
		},
		ReadFrom: appconfig.ReadFromConfig{
			GoogleCalendar: appconfig.GoogleCalendarReadConfig{
				CountryCode:        countryCode,
				TodayIsFreezeDayIf: rules,
			},
		},
		WriteTo: appconfig.WriteToConfig{
			GoogleCalendar: appconfig.GoogleCalendarWriteConfig{
				ID: calendarID,
				IfTodayIsFreezeDay: appconfig.IfTodayIsFreezeDayConfig{
					Default: appconfig.DefaultConfig{
						Summary:     helpers.StringPtr(summary),
						Description: helpers.StringPtr(description),
						StartTime:   startTimePtr,
						EndTime:     endTimePtr,
						AllDay:      allDayPtr,
					},
				},
			},
		},
	}
	cfg.SetDefault()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// rulesToJSON converts the todayIsFreezeDayIf slice to the JSON used by the JS rules builder.
func rulesToJSON(rules []map[string][]string) string {
	jsRules := make([]formRule, 0, len(rules))
	for _, r := range rules {
		for anchor, conditions := range r {
			jsRules = append(jsRules, formRule{Anchor: anchor, Conditions: conditions})
		}
	}
	b, err := json.Marshal(jsRules)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// formDataFromRequest rebuilds a configFormData from posted form values (for re-rendering on error).
func formDataFromRequest(r *http.Request) configFormData {
	lookback, _ := strconv.Atoi(r.FormValue("lookback_days"))
	lookahead, _ := strconv.Atoi(r.FormValue("lookahead_days"))
	var rules []map[string][]string
	var jsRules []formRule
	if err := json.Unmarshal([]byte(r.FormValue("rules_json")), &jsRules); err == nil {
		for _, jr := range jsRules {
			rules = append(rules, map[string][]string{jr.Anchor: jr.Conditions})
		}
	}
	if len(rules) == 0 {
		rules = defaultFormData().Rules
	}
	return configFormData{
		LookbackDays:  lookback,
		LookaheadDays: lookahead,
		CountryCode:   r.FormValue("country_code"),
		CalendarID:    r.FormValue("write_calendar_id"),
		Summary:       r.FormValue("event_summary"),
		Description:   r.FormValue("event_description"),
		StartTime:     r.FormValue("start_time"),
		EndTime:       r.FormValue("end_time"),
		AllDay:        r.FormValue("all_day") == "on",
		Rules:         rules,
	}
}

func (h *ConfigHandler) listBlockers(ctx context.Context, userID int64, cfg *db.Config) string {
	appCfg, err := h.parseAppConfig(cfg.ConfigYAML)
	if err != nil {
		return actionResultHTML("List Blockers", err.Error(), true)
	}
	repo, err := h.buildRepo(ctx, userID, appCfg)
	if err != nil {
		return actionResultHTML("List Blockers", err.Error(), true)
	}
	rangeStart, rangeEnd := dateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	blockers, err := repo.ListAllBlockersInRange(rangeStart, rangeEnd)
	if err != nil {
		return actionResultHTML("List Blockers", "failed to list blockers: "+err.Error(), true)
	}
	if len(blockers) == 0 {
		return `<p style="color:var(--pico-muted-color);text-align:center;padding:1rem"><em>No blocker events found in the date range.</em></p>`
	}

	items := make([]blockerItem, 0, len(blockers))
	for _, b := range blockers {
		items = append(items, blockerItem{
			Date:    b.Start.Format("2006-01-02"),
			Summary: b.Summary,
			ID:      b.ID,
		})
	}
	jsonBytes, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return actionResultHTML("List Blockers", "failed to format result: "+err.Error(), true)
	}

	rangeLabel := fmt.Sprintf("%s → %s",
		rangeStart.Format("2006-01-02"),
		rangeEnd.Format("2006-01-02"),
	)
	return fmt.Sprintf(`
<div>
  <div style="font-size:0.88rem;color:var(--pico-muted-color);margin-bottom:0.5rem">
    Blocker Events &nbsp;·&nbsp; <strong style="color:var(--pico-color)">%d found</strong> &nbsp;·&nbsp; range %s
  </div>
  <pre class="line-numbers language-json" style="white-space:pre-wrap;word-break:break-word;margin:0"><code>%s</code></pre>
</div>`, len(items), html.EscapeString(rangeLabel), html.EscapeString(string(jsonBytes)))
}

// --- schedule helpers ---

// parseSyncSchedule normalises the form value. Any unrecognised value (including
// wrong-case variants) is silently treated as "none" — form tampering is harmless here.
func parseSyncSchedule(s string) string {
	switch s {
	case db.SyncScheduleWeekly, db.SyncScheduleMonthly:
		return s
	default:
		return db.SyncScheduleNone
	}
}

// computeNextSyncAt returns the appropriate next_sync_at when a config is saved.
// If the schedule hasn't changed, we preserve the existing next_sync_at.
// If it changed, we compute a new one (or clear it for "none").
func computeNextSyncAt(oldSchedule string, oldNextSyncAt *time.Time, newSchedule string) *time.Time {
	if oldSchedule == newSchedule {
		return oldNextSyncAt
	}
	if newSchedule == db.SyncScheduleNone {
		return nil
	}
	t := scheduler.NextSyncAt(newSchedule, time.Now())
	return &t
}

// --- HTML fragments ---

func validateResultHTML(oldStatus, newStatus db.ConfigStatus, msg string) string {
	var summary string
	if oldStatus == newStatus {
		summary = fmt.Sprintf("Status unchanged: <strong>%s</strong>", html.EscapeString(string(newStatus)))
	} else {
		summary = fmt.Sprintf("Status updated: <strong>%s</strong> &rarr; <strong>%s</strong>",
			html.EscapeString(string(oldStatus)), html.EscapeString(string(newStatus)))
	}
	isErr := newStatus == db.ConfigStatusInvalid || newStatus == db.ConfigStatusUnauthorized
	bg, border, color := "#1a4731", "#166534", "#4ade80"
	if isErr {
		bg, border, color = "#4a1122", "#7f1d1d", "#f87171"
	}

	detail := ""
	if msg != "" {
		detail = fmt.Sprintf(`<div style="margin-top:0.3rem;font-size:0.85rem;opacity:0.8">%s</div>`, html.EscapeString(msg))
	}

	return fmt.Sprintf(`
<div style="background:%s;border:1px solid %s;color:%s;padding:0.75rem 1rem;border-radius:0.5rem;margin-top:0.75rem;font-size:0.9rem">
  <strong>Validate finished.</strong> %s%s
</div>
<span id="status-badge" hx-swap-oob="true">%s</span>`,
		bg, border, color,
		summary, detail,
		statusBadgeHTML(newStatus, ""),
	)
}

func statusBadgeHTML(status db.ConfigStatus, msg string) string {
	badge := statusBadge(status)
	if msg != "" {
		return fmt.Sprintf(`%s <small style="color:var(--pico-muted-color);font-size:0.8rem">%s</small>`, badge, html.EscapeString(msg))
	}
	return badge
}

func actionResultHTML(action, msg string, isErr bool) string {
	bg, border, color := "#1a4731", "#166534", "#4ade80"
	if isErr {
		bg, border, color = "#4a1122", "#7f1d1d", "#f87171"
	}
	return fmt.Sprintf(`<div style="background:%s;border:1px solid %s;color:%s;padding:0.75rem 1rem;border-radius:0.5rem;margin-top:0.75rem;font-size:0.9rem">
  <strong>%s:</strong> %s
</div>`, bg, border, color, html.EscapeString(action), html.EscapeString(msg))
}

// calendarPickerHTML renders an optional dropdown that fills the calendar ID field when selected.
func calendarPickerHTML(cals []*googlecalendar.CalendarItem, selectedID string) string {
	if len(cals) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<option value="">-- pick to fill ID below --</option>`)
	for _, c := range cals {
		sel := ""
		if c.ID == selectedID {
			sel = " selected"
		}
		fmt.Fprintf(&sb, `<option value="%s"%s>%s</option>`,
			html.EscapeString(c.ID), sel,
			html.EscapeString(c.Summary+" ("+c.ID+")"))
	}
	return fmt.Sprintf(`
<div style="display:flex;gap:0.5rem;align-items:flex-end;margin-bottom:0.25rem">
  <div style="flex:1">
    <label for="cal-picker" style="margin-bottom:0.25rem;font-size:0.88rem;color:var(--pico-muted-color)">Pick from writable calendars</label>
    <select id="cal-picker" onchange="document.getElementById('write_calendar_id').value=this.value;this.value=''">
      %s
    </select>
  </div>
</div>`, sb.String())
}

func autoSyncInfoHTML(cfg *db.Config) string {
	if cfg.SyncSchedule == db.SyncScheduleNone {
		return ""
	}

	scheduleLabel := map[string]string{
		db.SyncScheduleWeekly:  "weekly (Mon 09:00 JST)",
		db.SyncScheduleMonthly: "monthly (1st 09:00 JST)",
	}[cfg.SyncSchedule]

	lastSyncHTML := `<span style="color:var(--pico-muted-color)">No auto-sync has run yet.</span>`
	if cfg.LastAutoSyncedAt != nil {
		lastAt := cfg.LastAutoSyncedAt.In(jstDisplay).Format("2006-01-02 15:04 JST")
		result := ""
		if cfg.LastAutoSyncResult != nil {
			result = " — " + html.EscapeString(*cfg.LastAutoSyncResult)
		}
		lastSyncHTML = fmt.Sprintf(`<strong>%s</strong>%s`, html.EscapeString(lastAt), result)
	}

	nextSyncHTML := `<span style="color:var(--pico-muted-color)">—</span>`
	if cfg.NextSyncAt != nil {
		nextAt := cfg.NextSyncAt.In(jstDisplay).Format("2006-01-02 15:04 JST")
		nextSyncHTML = fmt.Sprintf(`<strong>%s</strong>`, html.EscapeString(nextAt))
	}

	return fmt.Sprintf(`
<div style="background:var(--pico-card-background-color);border:1px solid var(--pico-card-border-color);border-radius:0.5rem;padding:0.75rem 1rem;margin-bottom:1rem;font-size:0.88rem">
  <div style="font-weight:600;margin-bottom:0.4rem">⏰ Auto-Sync: %s</div>
  <div style="color:var(--pico-muted-color)">Last run: %s</div>
  <div style="color:var(--pico-muted-color);margin-top:0.2rem">Next run: %s</div>
</div>`, html.EscapeString(scheduleLabel), lastSyncHTML, nextSyncHTML)
}

func autoSyncTriggerHTML(cfg *db.Config, canEdit bool) string {
	scheduleLabel := map[string]string{
		db.SyncScheduleNone:    "off",
		db.SyncScheduleWeekly:  "weekly",
		db.SyncScheduleMonthly: "monthly",
	}[cfg.SyncSchedule]
	if scheduleLabel == "" {
		scheduleLabel = "off"
	}

	escaped := html.EscapeString(scheduleLabel)
	baseStyle := "border-bottom:1px dotted currentColor;color:#60a5fa;background:none;border-top:none;border-left:none;border-right:none;padding:0;font-size:inherit;font-family:inherit"

	if !canEdit {
		return fmt.Sprintf(` &nbsp;·&nbsp; Auto Sync: <span id="autosync-trigger"><span style="%s;cursor:default" title="Auto-Sync schedule">%s</span></span>`,
			baseStyle, escaped)
	}
	return fmt.Sprintf(` &nbsp;·&nbsp; Auto Sync: <span id="autosync-trigger"><button type="button" style="%s;cursor:pointer" title="Configure Auto-Sync" onclick="document.getElementById('autosync-modal').showModal()">%s</button></span>`,
		baseStyle, escaped)
}

func autoSyncModalHTML(basePath string, cfg *db.Config, canEdit bool) string {
	if !canEdit {
		return ""
	}
	return fmt.Sprintf(`
<dialog id="autosync-modal" style="max-width:480px;border-radius:0.75rem;background:var(--pico-card-background-color);border:1px solid var(--pico-card-border-color);color:var(--pico-color)">
  <article>
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:1rem">
      <strong style="font-size:1.1rem">⏰ Auto-Sync</strong>
      <button type="button" onclick="document.getElementById('autosync-modal').close()" style="background:none;border:none;cursor:pointer;font-size:1.3rem;color:var(--pico-muted-color);padding:0;line-height:1">×</button>
    </div>
    <p style="font-size:0.88rem;color:var(--pico-muted-color);margin-bottom:1.25rem">
      Auto-Sync automatically reads public holidays and writes blocker events to your calendar on a recurring schedule — no manual action needed.
      When enabled, manual <strong>Sync</strong> and <strong>Wipe</strong> operations are disabled to prevent conflicts.
    </p>
    <form hx-post="`+basePath+`/configs/%d/auto-sync" hx-target="#autosync-modal-error" hx-swap="innerHTML"
          hx-on::htmx:response-error="document.getElementById('autosync-modal-error').textContent='Save failed ('+event.detail.xhr.status+') — please try again'">
      <label for="modal-sync-schedule">Schedule
        <select id="modal-sync-schedule" name="sync_schedule">
          %s
        </select>
      </label>
      <div id="autosync-modal-error" style="min-height:1.5rem;margin-top:0.5rem"></div>
      <div style="display:flex;gap:0.5rem;justify-content:flex-end;margin-top:1rem">
        <button type="button" class="outline secondary" onclick="document.getElementById('autosync-modal').close()">Cancel</button>
        <button type="submit">Save</button>
      </div>
    </form>
  </article>
</dialog>`,
		cfg.ID, syncScheduleOptions(cfg.SyncSchedule))
}

func configDetailHTML(basePath string, cfg *db.Config, currentUserID int64, role perm.Role, appCfg *appconfig.Config, calendarName string) string {
	badge := statusBadgeHTML(cfg.Status, cfg.StatusMessage)
	escapedName := html.EscapeString(cfg.Name)
	escapedSchema := html.EscapeString(cfg.SchemaVersion)
	canSync := role.CanSyncConfig(cfg.UserID, currentUserID)
	canEdit := role.CanEditConfig(cfg.UserID, currentUserID)
	autoSyncEnabled := cfg.SyncSchedule != db.SyncScheduleNone

	editBtnHTML := ""
	if canEdit {
		editBtnHTML = fmt.Sprintf(`<a href="`+basePath+`/configs/%d/edit" role="button" class="outline" style="margin:0;padding:0.4rem 1rem;font-size:0.88rem">&#9998; Edit</a>`, cfg.ID)
	}

	const disabledTitle = "Manual operations are disabled while Auto-Sync is on. Disable Auto-Sync first if you want to sync or wipe manually."

	syncActionsHTML := ""
	if canSync {
		validateBtn := fmt.Sprintf(`
    <button
      hx-post="`+basePath+`/configs/%d/validate"
      hx-target="#action-result"
      hx-swap="innerHTML"
      hx-on::before-request="document.getElementById('action-result').innerHTML='<p class=ack>🔍 Validating&#8230;</p>';document.getElementById('status-badge').innerHTML='<em class=ack>checking&#8230;</em>'"
      class="outline"
      title="Re-validate the config and verify calendar write access">
      🔍 Validate
    </button>`, cfg.ID)

		var syncBtn, wipeBtn string
		if autoSyncEnabled {
			syncBtn = fmt.Sprintf(`
    <button disabled title="%s" style="cursor:not-allowed;opacity:0.5">
      ▶ Sync
    </button>`, html.EscapeString(disabledTitle))
			wipeBtn = fmt.Sprintf(`
    <button disabled title="%s" class="outline" style="cursor:not-allowed;opacity:0.5">
      🗑 Wipe Blockers
    </button>`, html.EscapeString(disabledTitle))
		} else {
			syncBtn = fmt.Sprintf(`
    <button
      hx-post="`+basePath+`/configs/%d/sync"
      hx-target="#action-result"
      hx-swap="innerHTML"
      hx-on::before-request="document.getElementById('action-result').innerHTML='<p class=ack>⏳ Syncing — reading holidays and writing blockers&#8230;</p>'"
      title="Read public holidays, calculate freeze days, and create blocker events on your calendar">
      ▶ Sync
    </button>`, cfg.ID)
			wipeBtn = fmt.Sprintf(`
    <button
      hx-post="`+basePath+`/configs/%d/wipe"
      hx-target="#action-result"
      hx-swap="innerHTML"
      hx-on::before-request="document.getElementById('action-result').innerHTML='<p class=ack>⏳ Wiping blockers&#8230;</p>'"
      class="outline"
      title="Remove all managed blocker events in the lookback/lookahead date range">
      🗑 Wipe Blockers
    </button>`, cfg.ID)
		}
		syncActionsHTML = validateBtn + syncBtn + wipeBtn
	}

	autoSyncTrigger := autoSyncTriggerHTML(cfg, canEdit)

	// Build human-readable config detail cards.
	configCardsHTML := configDetailCardsHTML(appCfg, calendarName)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🧊</text></svg>">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/themes/prism-tomorrow.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/plugins/line-numbers/prism-line-numbers.min.css">
  <script src="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js" defer></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/prism.min.js" defer></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-json.min.js" defer></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/plugins/line-numbers/prism-line-numbers.min.js" defer></script>
  <style>
    nav.topnav { background: var(--pico-card-background-color); border-bottom: 1px solid var(--pico-card-border-color); padding: 0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; }
    nav.topnav .brand { font-weight:700; text-decoration:none; color:inherit; }
    .page-content { max-width: 860px; margin: 2rem auto; padding: 0 1.5rem; }
    .breadcrumb { font-size:0.82rem; color:var(--pico-muted-color); margin-bottom:0.4rem; }
    .breadcrumb a { color:var(--pico-muted-color); text-decoration:none; }
    .breadcrumb a:hover { text-decoration:underline; }
    .page-header { display:flex; align-items:center; gap:0.75rem; margin-bottom:0.4rem; }
    .back-btn { font-size:1.4rem; text-decoration:none; color:var(--pico-muted-color); line-height:1; flex-shrink:0; }
    .back-btn:hover { color:var(--pico-color); }
    .action-bar { display:flex; gap:0.5rem; flex-wrap:wrap; margin-bottom:1.25rem; }
    .action-bar button, .action-bar a[role=button] { margin:0; padding:0.45rem 1rem; font-size:0.88rem; }
    .ack { opacity:0.65; font-style:italic; padding:0.5rem 0; font-size:0.9rem; }
    .detail-card { background:var(--pico-card-background-color); border:1px solid var(--pico-card-border-color); border-radius:0.5rem; padding:0.9rem 1.1rem; margin-bottom:0.75rem; }
    .detail-card h4 { margin:0 0 0.6rem; font-size:0.82rem; text-transform:uppercase; letter-spacing:0.05em; color:var(--pico-muted-color); }
    .detail-row { display:flex; gap:1rem; flex-wrap:wrap; }
    .detail-field { flex:1; min-width:140px; }
    .detail-field label { font-size:0.78rem; color:var(--pico-muted-color); display:block; margin-bottom:0.15rem; }
    .detail-field .val { font-size:0.92rem; }
    .rule-group { margin-bottom:0.4rem; padding:0.4rem 0.7rem; background:rgba(255,255,255,0.04); border-radius:0.35rem; font-size:0.88rem; }
    pre[class*="language-"] { line-height:1.5 !important; white-space:pre-wrap; word-break:break-word; font-size:0.84rem; border-radius:0.5rem; }
    .line-numbers .line-numbers-rows > span::before { line-height: 1.5; }
    #blockers-panel pre { margin-top: 0.5rem; }
    #autosync-modal { position:fixed; top:50%%; left:50%%; transform:translate(-50%%,-50%%); margin:0; }
    #autosync-modal::backdrop { background:rgba(0,0,0,0.6); }
  </style>
  <script>document.addEventListener('htmx:afterSwap', function() { Prism.highlightAll(); });</script>
</head>
<body>
<nav class="topnav">
  <a href="`+basePath+`/dashboard" class="brand">🙏🧔🏽‍♀️👉🧊🗓️ TGI Freeze Day</a>
  <div>%s</div>
</nav>
<div class="page-content">
  <div class="breadcrumb"><a href="`+basePath+`/dashboard">Configs</a> &rsaquo; %s</div>
  <div style="display:flex;align-items:flex-start;justify-content:space-between;flex-wrap:wrap;gap:0.5rem;margin-bottom:1rem">
    <div class="page-header">
      <a href="`+basePath+`/dashboard" class="back-btn" title="Back to Configs">&#8592;</a>
      <h2 style="margin:0">%s</h2>
    </div>
    %s
  </div>
  <div style="font-size:0.85rem;color:var(--pico-muted-color);margin-bottom:1rem">
    schema: %s &nbsp;·&nbsp; Status: <span id="status-badge">%s</span>%s
  </div>

  %s

  <div class="action-bar">
    %s
    <button
      hx-get="`+basePath+`/configs/%d/blockers"
      hx-target="#blockers-panel"
      hx-swap="innerHTML"
      hx-on::before-request="document.getElementById('blockers-panel').innerHTML='<p class=ack>⏳ Loading blockers&#8230;</p>'"
      class="outline"
      title="List all currently managed blocker events in the date range">
      📋 List Blockers
    </button>
  </div>

  <div id="action-result"></div>

  %s

  <div id="blockers-panel" style="margin-top:1.5rem"></div>
</div>

%s
`+pageFooterHTML()+`
</body>
</html>`,
		escapedName, logoutForm(basePath),
		escapedName,
		escapedName, editBtnHTML,
		escapedSchema, badge, autoSyncTrigger,
		autoSyncInfoHTML(cfg),
		syncActionsHTML, cfg.ID,
		configCardsHTML,
		autoSyncModalHTML(basePath, cfg, canEdit),
	)
}

// sectionHeaderHTML renders a section divider with an optional (?) tooltip.
func sectionHeaderHTML(label, tooltip string) string {
	tipHTML := ""
	if tooltip != "" {
		tipHTML = fmt.Sprintf(` <span title="%s" style="cursor:help;font-weight:normal;font-size:0.82rem;opacity:0.6;margin-left:0.3rem">(?)</span>`,
			html.EscapeString(tooltip))
	}
	return fmt.Sprintf(`<div class="section-header">%s%s</div>`, html.EscapeString(label), tipHTML)
}

// conditionLabel returns a human-readable label for a condition key.
func conditionLabel(c string) string {
	switch c {
	case "isTheFirstBusinessDayOfTheMonth":
		return "1st business day of month"
	case "isTheLastBusinessDayOfTheMonth":
		return "last business day of month"
	case "isNonBusinessDay":
		return "non-business day (weekend / holiday)"
	default:
		// return raw key for forward-compat — add labels here when adding new conditions
		return c
	}
}

// countryLabel returns a human-readable label for a country code.
func countryLabel(code string) string {
	switch code {
	case countryCodeJPN:
		return "Japan (jpn)"
	case countryCodeVNM:
		return "Vietnam (vnm)"
	default:
		return code
	}
}

// configDetailCardsHTML renders human-readable detail cards from the parsed config.
// Falls back to a generic parse-error notice when appCfg is nil.
// calendarName is the human-readable name for the write calendar (empty string = show ID only).
func configDetailCardsHTML(appCfg *appconfig.Config, calendarName string) string {
	if appCfg == nil {
		return `<div class="detail-card"><h4>Config (parse error)</h4><p style="font-size:0.84rem;color:var(--pico-muted-color)">Could not parse config YAML.</p></div>`
	}

	d := appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default

	// Date range card
	dateRangeCard := fmt.Sprintf(`
<div class="detail-card">
  <h4>Date Range <span title="How far back and forward to scan for freeze days." style="cursor:help;font-weight:normal;font-size:0.8rem;opacity:0.5">(?)</span></h4>
  <div class="detail-row">
    <div class="detail-field"><label>Lookback</label><div class="val">%d days</div></div>
    <div class="detail-field"><label>Lookahead</label><div class="val">%d days</div></div>
  </div>
</div>`, appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)

	// Holiday source card
	holidayCard := fmt.Sprintf(`
<div class="detail-card">
  <h4>Holiday Source <span title="The public holiday calendar to read from, used to identify non-business days." style="cursor:help;font-weight:normal;font-size:0.8rem;opacity:0.5">(?)</span></h4>
  <div class="detail-field"><label>Country</label><div class="val">%s</div></div>
</div>`, html.EscapeString(countryLabel(appCfg.ReadFrom.GoogleCalendar.CountryCode)))

	// Freeze rules card
	var rulesSB strings.Builder
	for i, rule := range appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf {
		if i > 0 {
			rulesSB.WriteString(`<div style="font-size:0.78rem;font-weight:700;color:#60a5fa;text-align:center;margin:0.3rem 0;letter-spacing:0.05em">OR</div>`)
		}
		for anchor, conditions := range rule {
			var condParts []string
			for _, c := range conditions {
				condParts = append(condParts, html.EscapeString(conditionLabel(c)))
			}
			condHTML := strings.Join(condParts, ` <span style="font-size:0.75rem;font-weight:700;color:#a78bfa;margin:0 0.2rem">AND</span> `)
			fmt.Fprintf(&rulesSB, `<div class="rule-group"><strong>%s</strong> <span style="color:var(--pico-muted-color);font-size:0.82rem">is:</span> %s</div>`,
				html.EscapeString(anchor), condHTML)
		}
	}
	freezeCard := fmt.Sprintf(`
<div class="detail-card">
  <h4>Freeze Rules <span title="Today is a freeze day if any group matches (OR). Within a group, all conditions must match (AND)." style="cursor:help;font-weight:normal;font-size:0.8rem;opacity:0.5">(?)</span></h4>
  <div style="font-size:0.82rem;color:var(--pico-muted-color);margin-bottom:0.5rem">Today is a freeze day if:</div>
  %s
</div>`, rulesSB.String())

	// Calendar card — show friendly name when available.
	calID := appCfg.WriteTo.GoogleCalendar.ID
	var calDisplay string
	if calendarName != "" {
		calDisplay = `<strong>` + html.EscapeString(calendarName) + `</strong>` +
			`<div style="font-size:0.8rem;color:var(--pico-muted-color);word-break:break-all;margin-top:0.2rem">` + html.EscapeString(calID) + `</div>`
	} else {
		calDisplay = `<span style="color:var(--pico-muted-color);font-size:0.85rem">(name unavailable)</span>` +
			`<div style="font-size:0.8rem;color:var(--pico-muted-color);word-break:break-all;margin-top:0.2rem">` + html.EscapeString(calID) + `</div>`
	}
	calendarCard := fmt.Sprintf(`
<div class="detail-card">
  <h4>Target Calendar <span title="The Google Calendar where blocker events are written on freeze days." style="cursor:help;font-weight:normal;font-size:0.8rem;opacity:0.5">(?)</span></h4>
  <div class="detail-field"><div class="val">%s</div></div>
</div>`, calDisplay)

	// Blocker event card
	evSummary := ""
	evDescription := ""
	startTime := ""
	endTime := ""
	isAllDay := d.AllDay != nil && *d.AllDay
	if d.Summary != nil {
		evSummary = *d.Summary
	}
	if d.Description != nil {
		evDescription = *d.Description
	}
	if d.StartTime != nil {
		startTime = *d.StartTime
	}
	if d.EndTime != nil {
		endTime = *d.EndTime
	}
	var timingHTML string
	if isAllDay {
		timingHTML = `<div class="detail-field"><label>Timing</label><div class="val">All-day event</div></div>`
	} else {
		timingHTML = fmt.Sprintf(`
  <div class="detail-row">
    <div class="detail-field"><label>Start</label><div class="val">%s</div></div>
    <div class="detail-field"><label>End</label><div class="val">%s</div></div>
  </div>`, html.EscapeString(startTime), html.EscapeString(endTime))
	}
	eventCard := fmt.Sprintf(`
<div class="detail-card">
  <h4>Blocker Event <span title="The calendar event created on each freeze day to signal no deployments allowed." style="cursor:help;font-weight:normal;font-size:0.8rem;opacity:0.5">(?)</span></h4>
  <div class="detail-field" style="margin-bottom:0.5rem"><label>Summary</label><div class="val">%s</div></div>
  <div class="detail-field" style="margin-bottom:0.5rem"><label>Description</label><div class="val" style="font-size:0.85rem;white-space:pre-wrap">%s</div></div>
  %s
</div>`,
		html.EscapeString(evSummary),
		html.EscapeString(evDescription),
		timingHTML)

	return dateRangeCard + holidayCard + freezeCard + calendarCard + eventCard
}

func syncScheduleOptions(selected string) string {
	options := []struct {
		value, label string
	}{
		{db.SyncScheduleNone, "Off (manual only)"},
		{db.SyncScheduleWeekly, "Weekly (Mon 09:00 JST)"},
		{db.SyncScheduleMonthly, "Monthly (1st 09:00 JST)"},
	}
	var sb strings.Builder
	for _, o := range options {
		sel := ""
		if o.value == selected {
			sel = " selected"
		}
		fmt.Fprintf(&sb, `<option value="%s"%s>%s</option>`,
			html.EscapeString(o.value), sel, html.EscapeString(o.label))
	}
	return sb.String()
}

func configFormHTML(title, action, backURL string, data configFormData, formErr string, isEdit bool, cals []*googlecalendar.CalendarItem, basePath string) string {
	errHTML := ""
	if formErr != "" {
		errHTML = fmt.Sprintf(`<div style="background:#4a1122;border:1px solid #7f1d1d;color:#f87171;padding:0.75rem 1rem;border-radius:0.5rem;margin-bottom:1rem">%s</div>`,
			html.EscapeString(formErr))
	}

	var breadcrumb, pageHeader string
	if isEdit {
		breadcrumb = fmt.Sprintf(`<a href="`+basePath+`/dashboard">Configs</a> &rsaquo; <a href="%s">%s</a> &rsaquo; Edit`,
			html.EscapeString(backURL), html.EscapeString(data.Name))
		pageHeader = fmt.Sprintf(`Edit: %s`, html.EscapeString(data.Name))
	} else {
		breadcrumb = `<a href="` + basePath + `/dashboard">Configs</a> &rsaquo; New Config`
		pageHeader = `New Config`
	}

	deleteBtn := ""
	if isEdit {
		deleteBtn = fmt.Sprintf(`<form method="POST" action="%s/delete" style="margin:0" onsubmit="return confirm('Delete this config?')"><button type="submit" class="outline contrast">Delete</button></form>`,
			html.EscapeString(action))
	}

	calPicker := calendarPickerHTML(cals, data.CalendarID)

	allDayChecked := ""
	timeFieldsStyle := ""
	timeRequired := " required"
	timeDisabled := ""
	if data.AllDay {
		allDayChecked = " checked"
		timeFieldsStyle = `style="opacity:0.4;pointer-events:none"`
		timeRequired = ""
		timeDisabled = " disabled"
	}

	autoSyncPicker := fmt.Sprintf(`
<label for="sync_schedule">Auto-Sync
  <select id="sync_schedule" name="sync_schedule">
    %s
  </select>
  <small style="color:var(--pico-muted-color)">Weekly fires every Monday 09:00 JST. Monthly fires on the 1st at 09:00 JST. When enabled, manual Sync and Wipe are disabled.</small>
</label>`, syncScheduleOptions(data.SyncSchedule))

	// Country select
	countryOptions := ""
	for _, cc := range []struct{ val, label string }{
		{countryCodeJPN, "Japan (jpn)"},
		{countryCodeVNM, "Vietnam (vnm)"},
	} {
		sel := ""
		if cc.val == data.CountryCode {
			sel = " selected"
		}
		countryOptions += fmt.Sprintf(`<option value="%s"%s>%s</option>`, cc.val, sel, cc.label)
	}

	initialRulesJSON := rulesToJSON(data.Rules)
	if initialRulesJSON == "null" || initialRulesJSON == "" {
		initialRulesJSON = "[]"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🧊</text></svg>">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <style>
    nav.topnav { background:var(--pico-card-background-color); border-bottom:1px solid var(--pico-card-border-color); padding:0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; }
    nav.topnav .brand { font-weight:700; text-decoration:none; color:inherit; }
    .page-content { max-width:760px; margin:2rem auto; padding:0 1.5rem; }
    .breadcrumb { font-size:0.82rem; color:var(--pico-muted-color); margin-bottom:0.4rem; }
    .breadcrumb a { color:var(--pico-muted-color); text-decoration:none; }
    .breadcrumb a:hover { text-decoration:underline; }
    .back-btn { font-size:1.4rem; text-decoration:none; color:var(--pico-muted-color); line-height:1; flex-shrink:0; }
    .back-btn:hover { color:var(--pico-color); }
    .form-actions { display:flex; gap:0.6rem; align-items:center; flex-wrap:wrap; }
    .form-actions button,.form-actions a[role=button] { margin:0; width:auto; padding:0.45rem 1.1rem; font-size:0.9rem; }
    .section-header { font-size:0.78rem; font-weight:600; text-transform:uppercase; letter-spacing:0.05em; color:var(--pico-muted-color); border-bottom:1px solid var(--pico-card-border-color); padding-bottom:0.3rem; margin:1.25rem 0 0.75rem; }
    .two-col { display:grid; grid-template-columns:1fr 1fr; gap:0.75rem; }
    .rule-group-box { background:var(--pico-card-background-color); border:1px solid var(--pico-card-border-color); border-radius:0.4rem; padding:0.65rem 0.75rem; margin-bottom:0.5rem; }
    .rule-cond-row { display:flex; align-items:center; gap:0.5rem; margin-bottom:0.35rem; }
    .rule-cond-row select { flex:1; margin:0; padding:0.3rem 0.5rem; font-size:0.85rem; }
    .rule-cond-row button { margin:0; padding:0.25rem 0.55rem; font-size:0.8rem; }
    .btn-small { padding:0.3rem 0.65rem !important; font-size:0.82rem !important; }
    .or-divider { text-align:center; font-size:0.8rem; color:var(--pico-muted-color); margin:0.25rem 0; }
  </style>
</head>
<body>
<nav class="topnav">
  <a href="`+basePath+`/dashboard" class="brand">🙏🧔🏽‍♀️👉🧊🗓️ TGI Freeze Day</a>
  <div>%s</div>
</nav>
<div class="page-content">
  <div class="breadcrumb">%s</div>
  <div style="display:flex;align-items:center;gap:0.75rem;margin-bottom:1.5rem">
    <a href="%s" class="back-btn" title="Go back">&#8592;</a>
    <h2 style="margin:0">%s</h2>
  </div>
  %s
  <form id="config-form" method="POST" action="%s">
    <label for="name">Config Name
      <input type="text" id="name" name="name" value="%s" placeholder="e.g. Japan prod freeze" required>
    </label>

    `+sectionHeaderHTML("Date Range", "How far back and forward to scan for freeze days. Lookback covers past days already elapsed; lookahead covers upcoming days.")+`
    <div class="two-col">
      <label for="lookback_days">Lookback days
        <input type="number" id="lookback_days" name="lookback_days" value="%d" min="20" max="60" required>
        <small style="color:var(--pico-muted-color)">20–60</small>
      </label>
      <label for="lookahead_days">Lookahead days
        <input type="number" id="lookahead_days" name="lookahead_days" value="%d" min="20" max="60" required>
        <small style="color:var(--pico-muted-color)">20–60</small>
      </label>
    </div>

    `+sectionHeaderHTML("Holiday Source", "The public holiday calendar to read from. Used to identify non-business days (weekends + public holidays) in the target country.")+`
    <label for="country_code">Country
      <select id="country_code" name="country_code">%s</select>
    </label>

    `+sectionHeaderHTML("Freeze Rules", "Defines when today counts as a freeze day. Groups are OR'd — if any group matches, today is a freeze day. Within a group, all conditions must match (AND).")+`
    <p style="font-size:0.85rem;color:var(--pico-muted-color);margin-bottom:0.75rem">
      Today is a freeze day if <em>any</em> rule group matches (OR). Within a group, <em>all</em> conditions must match (AND).
    </p>
    <div id="rules-container"></div>
    <button type="button" class="outline btn-small" onclick="addRuleGroup()" style="margin-bottom:1rem">+ OR group</button>
    <input type="hidden" id="rules_json" name="rules_json">

    `+sectionHeaderHTML("Target Calendar", "The Google Calendar where blocker events will be written on freeze days. Must be a calendar you have write access to.")+`
    %s
    <label for="write_calendar_id">Calendar ID
      <input type="text" id="write_calendar_id" name="write_calendar_id" value="%s" placeholder="team-cal@group.calendar.google.com" required>
    </label>

    `+sectionHeaderHTML("Blocker Event", "The calendar event created on each freeze day. Title and description appear in Google Calendar to signal no deployments allowed.")+`
    <label for="event_summary">Summary
      <input type="text" id="event_summary" name="event_summary" value="%s" maxlength="250" placeholder="🚫 PRODUCTION FREEZE - No Deployments" required>
      <small style="color:var(--pico-muted-color)">Max 250 characters</small>
    </label>
    <label for="event_description">Description
      <textarea id="event_description" name="event_description" rows="4" maxlength="8000" placeholder="Production operations restricted today.">%s</textarea>
      <small style="color:var(--pico-muted-color)">HTML supported: &lt;br&gt;, &lt;ul&gt;&lt;li&gt;, &lt;a href=""&gt;, &lt;strong&gt;, &lt;em&gt;. Max 8000 characters.</small>
    </label>
    <div style="margin-bottom:1rem">
      <label style="display:flex;align-items:center;gap:0.6rem;cursor:pointer;font-weight:500">
        <input type="checkbox" id="all_day" name="all_day" onchange="toggleAllDay(this)" style="width:1.1rem;height:1.1rem;cursor:pointer"%s>
        All-day event
      </label>
      <small style="color:var(--pico-muted-color);display:block;margin-top:0.25rem">Creates a full-day calendar event instead of a timed one</small>
    </div>
    <div id="time-fields" %s>
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:1rem">
        <label for="start_time">Start time
          <input type="time" id="start_time" name="start_time" value="%s"%s%s>
        </label>
        <label for="end_time">End time
          <input type="time" id="end_time" name="end_time" value="%s"%s%s>
        </label>
      </div>
    </div>

    `+sectionHeaderHTML("Auto-Sync", "Runs Sync automatically on a schedule so you don't have to click manually. When enabled, manual Sync and Wipe are disabled to prevent conflicts.")+`
    %s

    <div class="form-actions" style="margin-top:1.5rem">
      <button type="submit">Save</button>
      %s
      <a href="%s" role="button" class="outline secondary">Cancel</a>
    </div>
  </form>
</div>
<script>
var ANCHORS = [
  {value:'today', label:'today'},
  {value:'tomorrow', label:'tomorrow'},
  {value:'nextDay', label:'next day'}
];
var CONDITIONS = [
  {value:'isTheFirstBusinessDayOfTheMonth', label:'1st business day of month'},
  {value:'isTheLastBusinessDayOfTheMonth', label:'last business day of month'},
  {value:'isNonBusinessDay', label:'non-business day (weekend / holiday)'}
];

var rules = %s;
if (!Array.isArray(rules) || rules.length === 0) {
  rules = [{anchor:'today', conditions:['isTheFirstBusinessDayOfTheMonth']}];
}

function anchorSelect(groupIdx, val) {
  var s = '<select onchange="rules['+groupIdx+'].anchor=this.value">';
  ANCHORS.forEach(function(a) {
    s += '<option value="'+a.value+'"'+(a.value===val?' selected':'')+'>'+a.label+'</option>';
  });
  return s+'</select>';
}

function condSelect(groupIdx, condIdx, val) {
  var s = '<select onchange="rules['+groupIdx+'].conditions['+condIdx+']=this.value">';
  CONDITIONS.forEach(function(c) {
    s += '<option value="'+c.value+'"'+(c.value===val?' selected':'')+'>'+c.label+'</option>';
  });
  return s+'</select>';
}

function renderRules() {
  var container = document.getElementById('rules-container');
  var html = '';
  rules.forEach(function(group, gi) {
    html += '<div class="rule-group-box">';
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem">';
    html += anchorSelect(gi, group.anchor);
    html += '<span style="font-size:0.85rem;color:var(--pico-muted-color)">is:</span>';
    if (rules.length > 1) {
      html += '<button type="button" class="outline contrast btn-small" style="margin-left:auto" onclick="removeGroup('+gi+')" title="Remove this OR group">✕ group</button>';
    }
    html += '</div>';
    group.conditions.forEach(function(cond, ci) {
      html += '<div class="rule-cond-row">';
      if (ci === 0) {
        html += '<span style="font-size:0.8rem;color:var(--pico-muted-color);flex-shrink:0;min-width:2.5rem">is:</span>';
      } else {
        html += '<span style="font-size:0.75rem;font-weight:700;color:#a78bfa;flex-shrink:0;min-width:2.5rem">AND</span>';
      }
      html += condSelect(gi, ci, cond);
      if (group.conditions.length > 1) {
        html += '<button type="button" class="outline btn-small" onclick="removeCond('+gi+','+ci+')" title="Remove condition">✕</button>';
      }
      html += '</div>';
    });
    html += '<button type="button" class="outline btn-small" onclick="addCond('+gi+')" style="margin-top:0.25rem;margin-left:2.9rem">+ AND condition</button>';
    html += '</div>';
    if (gi < rules.length - 1) {
      html += '<div class="or-divider" style="font-weight:700;color:#60a5fa;letter-spacing:0.08em">— OR —</div>';
    }
  });
  container.innerHTML = html;
}

function addRuleGroup() {
  rules.push({anchor:'today', conditions:['isTheFirstBusinessDayOfTheMonth']});
  renderRules();
}

function removeGroup(gi) {
  if (rules.length <= 1) return;
  rules.splice(gi, 1);
  renderRules();
}

function addCond(gi) {
  rules[gi].conditions.push('isTheFirstBusinessDayOfTheMonth');
  renderRules();
}

function removeCond(gi, ci) {
  if (rules[gi].conditions.length <= 1) return;
  rules[gi].conditions.splice(ci, 1);
  renderRules();
}

function toggleAllDay(cb) {
  var container = document.getElementById('time-fields');
  var inputs = container.querySelectorAll('input[type=time]');
  if (cb.checked) {
    container.style.opacity = '0.4';
    container.style.pointerEvents = 'none';
    inputs.forEach(function(inp) {
      inp.disabled = true;
      inp.removeAttribute('required');
    });
  } else {
    container.style.opacity = '1';
    container.style.pointerEvents = '';
    inputs.forEach(function(inp) {
      inp.disabled = false;
      inp.setAttribute('required', '');
    });
  }
}

document.addEventListener('DOMContentLoaded', function() {
  var cb = document.getElementById('all_day');
  if (cb && cb.checked) toggleAllDay(cb);
});

document.getElementById('config-form').addEventListener('submit', function() {
  document.getElementById('rules_json').value = JSON.stringify(rules);
});

renderRules();
</script>
`+pageFooterHTML()+`
</body>
</html>`,
		html.EscapeString(title),
		logoutForm(basePath),
		breadcrumb,
		html.EscapeString(backURL),
		pageHeader,
		errHTML,
		html.EscapeString(action),
		html.EscapeString(data.Name),
		data.LookbackDays,
		data.LookaheadDays,
		countryOptions,
		calPicker,
		html.EscapeString(data.CalendarID),
		html.EscapeString(data.Summary),
		html.EscapeString(data.Description),
		allDayChecked,
		timeFieldsStyle,
		html.EscapeString(data.StartTime),
		timeDisabled,
		timeRequired,
		html.EscapeString(data.EndTime),
		timeDisabled,
		timeRequired,
		autoSyncPicker,
		deleteBtn,
		html.EscapeString(backURL),
		initialRulesJSON,
	)
}
