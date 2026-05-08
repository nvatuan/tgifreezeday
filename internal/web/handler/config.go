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
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/nvat/tgifreezeday/internal/perm"
	"github.com/nvat/tgifreezeday/internal/scheduler"
	"golang.org/x/oauth2"
	googleapi "google.golang.org/api/googleapi"
)

var log = logging.GetLogger()

// jstDisplay aliases scheduler.JST for formatting timestamps in the UI.
var jstDisplay = scheduler.JST

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
	items, err := googlecalendar.ListWritableCalendars(ctx, h.oauthCfg, token)
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
	schemaYAML, _ := appconfig.SchemaYAML(appconfig.CurrentSchemaVersion)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configFormHTML("New Config", h.basePath+"/configs", h.basePath+"/dashboard", appconfig.CurrentSchemaVersion, "", "", string(schemaYAML), "", false, db.SyncScheduleNone, cals, h.basePath)) //nolint:errcheck
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
	yamlContent := r.FormValue("config_yaml")
	syncSchedule := parseSyncSchedule(r.FormValue("sync_schedule"))

	if name == "" {
		cals := h.fetchCalendars(r.Context(), user.ID)
		schemaYAML, _ := appconfig.SchemaYAML(appconfig.CurrentSchemaVersion)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, configFormHTML("New Config", h.basePath+"/configs", h.basePath+"/dashboard", appconfig.CurrentSchemaVersion, name, yamlContent, string(schemaYAML), "Name is required.", false, syncSchedule, cals, h.basePath)) //nolint:errcheck
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configDetailHTML(h.basePath, cfg, user.ID, role)) //nolint:errcheck
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
	schemaYAML, _ := appconfig.SchemaYAML(cfg.SchemaVersion)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	action := fmt.Sprintf(h.basePath+"/configs/%d", id)
	backURL := fmt.Sprintf(h.basePath+"/configs/%d", id)
	fmt.Fprint(w, configFormHTML("Edit Config", action, backURL, cfg.SchemaVersion, cfg.Name, cfg.ConfigYAML, string(schemaYAML), "", true, cfg.SyncSchedule, cals, h.basePath)) //nolint:errcheck
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
	yamlContent := r.FormValue("config_yaml")
	syncSchedule := parseSyncSchedule(r.FormValue("sync_schedule"))

	if name == "" {
		httpError(w, http.StatusBadRequest, "name is required")
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
	_, err = googlecalendar.NewRepositoryWithToken(ctx, h.oauthCfg, token,
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
	return googlecalendar.NewRepositoryWithToken(ctx, h.oauthCfg, token,
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
	rangeStart, rangeEnd := dateRange(appCfg.Shared.LookbackDays, appCfg.Shared.LookaheadDays)
	return domain.RunSync(
		repo,
		rangeStart, rangeEnd,
		domain.TodayIsFreezeDayIf(appCfg.ReadFrom.GoogleCalendar.TodayIsFreezeDayIf),
		*appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Summary,
		*appCfg.WriteTo.GoogleCalendar.IfTodayIsFreezeDay.Default.Description,
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

func calendarOptions(cals []*googlecalendar.CalendarItem) string {
	if len(cals) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<option value="">-- select to fill calendar ID in YAML --</option>`)
	for _, c := range cals {
		fmt.Fprintf(&sb, `<option value="%s">%s</option>`,
			html.EscapeString(c.ID), html.EscapeString(c.Summary+" ("+c.ID+")"))
	}
	opts := sb.String()
	return fmt.Sprintf(`
<details style="margin-bottom:1rem">
  <summary style="cursor:pointer;font-weight:600">📅 Pick target calendar</summary>
  <div style="margin-top:0.75rem">
    <select id="cal-picker" onchange="applyCalendarId(this.value)">
      %s
    </select>
    <small style="display:block;margin-top:0.4rem;color:var(--pico-muted-color)">Selecting a calendar inserts its ID into the YAML below.</small>
  </div>
</details>
<script>
function applyCalendarId(id) {
  if (!id) return;
  var safeId = JSON.stringify(id); // safely quoted, no injection
  var re = /(writeTo[\s\S]*?googleCalendar[\s\S]*?id:\s*)["']?[^\n"']*["']?/;
  if (window.cmEditor) {
    var updated = window.cmEditor.getValue().replace(re, '$1' + safeId);
    window.cmEditor.setValue(updated);
  } else {
    var ta = document.getElementById('config_yaml');
    ta.value = ta.value.replace(re, '$1' + safeId);
  }
  document.getElementById('cal-picker').value = '';
}
</script>`, opts)
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

func configDetailHTML(basePath string, cfg *db.Config, currentUserID int64, role perm.Role) string {
	badge := statusBadgeHTML(cfg.Status, cfg.StatusMessage)
	escapedName := html.EscapeString(cfg.Name)
	escapedSchema := html.EscapeString(cfg.SchemaVersion)
	escapedYAML := html.EscapeString(cfg.ConfigYAML)
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
      title="Re-check the config YAML and verify calendar write access">
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

	autoSyncBadge := ""
	if autoSyncEnabled {
		autoSyncBadge = fmt.Sprintf(` &nbsp;·&nbsp; <strong style="color:#60a5fa">⏰ auto-sync: %s</strong>`,
			html.EscapeString(cfg.SyncSchedule))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🧊</text></svg>">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/codemirror.min.css">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/theme/dracula.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/themes/prism-tomorrow.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/plugins/line-numbers/prism-line-numbers.min.css">
  <script src="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js" defer></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/codemirror.min.js" defer></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/mode/yaml/yaml.min.js" defer></script>
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
    .CodeMirror { height: auto; font-size: 0.88rem; line-height: 1.5; border: 1px solid var(--pico-card-border-color); border-radius: 0.5rem; }
    .CodeMirror-cursor { display: none !important; }
    pre[class*="language-"] { line-height: 1.5 !important; white-space: pre-wrap; word-break: break-word; font-size: 0.84rem; border-radius: 0.5rem; }
    .line-numbers .line-numbers-rows > span::before { line-height: 1.5; }
    #blockers-panel pre { margin-top: 0.5rem; }
  </style>
  <script>
    document.addEventListener('htmx:afterSwap', function() { Prism.highlightAll(); });
    document.addEventListener('DOMContentLoaded', function() {
      var ta = document.getElementById('yaml-viewer');
      if (ta && typeof CodeMirror !== 'undefined') {
        CodeMirror.fromTextArea(ta, {
          mode: 'yaml',
          theme: 'dracula',
          lineNumbers: true,
          lineWrapping: true,
          readOnly: true,
          viewportMargin: Infinity,
          cursorBlinkRate: -1
        });
      }
    });
  </script>
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

  <details open style="margin-top:1rem">
    <summary style="cursor:pointer;font-weight:600">Config YAML</summary>
    <div style="margin-top:0.5rem">
      <textarea id="yaml-viewer" style="display:none">%s</textarea>
    </div>
  </details>

  <div id="blockers-panel" style="margin-top:1.5rem"></div>
</div>
</body>
</html>`,
		escapedName, logoutForm(basePath),
		escapedName,
		escapedName, editBtnHTML,
		escapedSchema, badge, autoSyncBadge,
		autoSyncInfoHTML(cfg),
		syncActionsHTML, cfg.ID,
		escapedYAML,
	)
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

func configFormHTML(title, action, backURL, schemaVersion, name, yamlContent, _ /* schemaYAML */, formErr string, isEdit bool, syncSchedule string, cals []*googlecalendar.CalendarItem, basePath string) string {
	errHTML := ""
	if formErr != "" {
		errHTML = fmt.Sprintf(`<div style="background:#4a1122;border:1px solid #7f1d1d;color:#f87171;padding:0.75rem 1rem;border-radius:0.5rem;margin-bottom:1rem">%s</div>`,
			html.EscapeString(formErr))
	}

	var breadcrumb, pageHeader string
	if isEdit {
		breadcrumb = fmt.Sprintf(`<a href="`+basePath+`/dashboard">Configs</a> &rsaquo; <a href="%s">%s</a> &rsaquo; Edit`,
			html.EscapeString(backURL), html.EscapeString(name))
		pageHeader = fmt.Sprintf(`Edit: %s`, html.EscapeString(name))
	} else {
		breadcrumb = `<a href="` + basePath + `/dashboard">Configs</a> &rsaquo; New Config`
		pageHeader = `New Config`
	}

	defaultYAML := `shared:
  lookbackDays: 20
  lookaheadDays: 20

readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
    - today:
      - isTheFirstBusinessDayOfTheMonth
    - today:
      - isTheLastBusinessDayOfTheMonth
    - tomorrow:
      - isNonBusinessDay

writeTo:
  googleCalendar:
    id: "ngo.van.anh.tuan@moneyforward.co.jp"`

	if yamlContent == "" {
		yamlContent = defaultYAML
	}

	deleteBtn := ""
	if isEdit {
		deleteBtn = fmt.Sprintf(`<form method="POST" action="%s/delete" style="margin:0" onsubmit="return confirm('Delete this config?')"><button type="submit" class="outline contrast">Delete</button></form>`,
			html.EscapeString(action))
	}

	calPicker := calendarOptions(cals)

	schemaPicker := fmt.Sprintf(`
<label for="schema_version">Schema Version
  <div style="display:flex;align-items:center;gap:0.5rem">
    <select id="schema_version" name="schema_version" style="flex:1;margin:0">
      <option value="v1"%s>v1 (current)</option>
    </select>
    <a href="`+basePath+`/schema/%s" target="_blank" title="View schema reference for %s"
       style="font-size:1.1rem;text-decoration:none;flex-shrink:0">ℹ️</a>
  </div>
</label>`,
		func() string {
			if schemaVersion == "v1" {
				return " selected"
			}
			return ""
		}(),
		html.EscapeString(schemaVersion),
		html.EscapeString(schemaVersion),
	)

	autoSyncPicker := fmt.Sprintf(`
<label for="sync_schedule">Auto-Sync
  <select id="sync_schedule" name="sync_schedule">
    %s
  </select>
  <small style="color:var(--pico-muted-color)">Weekly fires every Monday 09:00 JST. Monthly fires on the 1st at 09:00 JST. When enabled, manual Sync and Wipe are disabled.</small>
</label>`, syncScheduleOptions(syncSchedule))

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🧊</text></svg>">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/codemirror.min.css">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/theme/dracula.min.css">
  <style>
    nav.topnav { background: var(--pico-card-background-color); border-bottom: 1px solid var(--pico-card-border-color); padding: 0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; }
    nav.topnav .brand { font-weight:700; text-decoration:none; color:inherit; }
    .page-content { max-width: 760px; margin: 2rem auto; padding: 0 1.5rem; }
    .breadcrumb { font-size:0.82rem; color:var(--pico-muted-color); margin-bottom:0.4rem; }
    .breadcrumb a { color:var(--pico-muted-color); text-decoration:none; }
    .breadcrumb a:hover { text-decoration:underline; }
    .back-btn { font-size:1.4rem; text-decoration:none; color:var(--pico-muted-color); line-height:1; flex-shrink:0; }
    .back-btn:hover { color:var(--pico-color); }
    .form-actions { display:flex; gap:0.6rem; align-items:center; flex-wrap:wrap; }
    .form-actions button, .form-actions a[role=button] { margin:0; width:auto; padding:0.45rem 1.1rem; font-size:0.9rem; }
    .CodeMirror {
      height: 480px;
      font-size: 0.88rem;
      font-family: monospace;
      border: 1px solid var(--pico-form-element-border-color);
      border-radius: var(--pico-border-radius);
      line-height: 1.5;
    }
    .CodeMirror-scroll { padding-bottom: 0.5rem; }
    label.yaml-label { display:block; margin-bottom:0.25rem; font-weight:500; }
  </style>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/codemirror.min.js"></script>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.16/mode/yaml/yaml.min.js"></script>
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
    %s
    %s
    %s
    <div style="margin-bottom:1rem">
      <label class="yaml-label" for="config_yaml">Config YAML</label>
      <textarea id="config_yaml" name="config_yaml" style="display:none">%s</textarea>
    </div>
    <div class="form-actions">
      <button type="submit">Save</button>
      %s
      <a href="%s" role="button" class="outline secondary">Cancel</a>
    </div>
  </form>
</div>
<script>
window.cmEditor = CodeMirror.fromTextArea(document.getElementById('config_yaml'), {
  mode: 'yaml',
  theme: 'dracula',
  lineNumbers: true,
  lineWrapping: true,
  tabSize: 2,
  indentWithTabs: false,
  autofocus: false,
  extraKeys: { Tab: function(cm) { cm.replaceSelection('  '); } }
});
document.getElementById('config-form').addEventListener('submit', function() {
  window.cmEditor.save();
});
</script>
</body>
</html>`,
		html.EscapeString(title),
		logoutForm(basePath),
		breadcrumb,
		html.EscapeString(backURL),
		pageHeader,
		errHTML,
		html.EscapeString(action),
		html.EscapeString(name),
		schemaPicker,
		autoSyncPicker,
		calPicker,
		html.EscapeString(yamlContent),
		deleteBtn,
		html.EscapeString(backURL),
	)
}
