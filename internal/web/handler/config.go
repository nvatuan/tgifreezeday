package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	appconfig "github.com/nvat/tgifreezeday/internal/config"
	"golang.org/x/oauth2"
	googleapi "google.golang.org/api/googleapi"
)

type ConfigHandler struct {
	configs  *db.ConfigStore
	tokens   *db.TokenStore
	oauthCfg *oauth2.Config
}

func NewConfigHandler(configs *db.ConfigStore, tokens *db.TokenStore) *ConfigHandler {
	return &ConfigHandler{
		configs:  configs,
		tokens:   tokens,
		oauthCfg: googlecalendar.NewOAuthConfig(),
	}
}

// idFromPath parses the {id} path value (Go 1.22+).
func idFromPath(r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}

// HandleNew renders the config creation form.
func (h *ConfigHandler) HandleNew(w http.ResponseWriter, r *http.Request) {
	schemaYAML, _ := appconfig.SchemaYAML(appconfig.CurrentSchemaVersion)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configFormHTML("New Config", "", "", string(schemaYAML), "", false))
}

// HandleCreate processes the config creation form.
func (h *ConfigHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form")
		return
	}
	name := r.FormValue("name")
	yamlContent := r.FormValue("config_yaml")

	if name == "" {
		schemaYAML, _ := appconfig.SchemaYAML(appconfig.CurrentSchemaVersion)
		fmt.Fprint(w, configFormHTML("New Config", name, yamlContent, string(schemaYAML), "Name is required.", false))
		return
	}

	cfg, err := h.configs.Create(user.ID, name, appconfig.CurrentSchemaVersion, yamlContent)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to create config")
		return
	}

	// Validate in background (update status)
	go h.validateAndUpdateStatus(cfg.ID, user.ID, yamlContent)

	redirectTo(w, r, fmt.Sprintf("/configs/%d", cfg.ID))
}

// HandleDetail renders the config detail page.
func (h *ConfigHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configDetailHTML(cfg))
}

// HandleEdit renders the config edit form pre-populated.
func (h *ConfigHandler) HandleEdit(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}
	schemaYAML, _ := appconfig.SchemaYAML(cfg.SchemaVersion)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configFormHTML("Edit Config", cfg.Name, cfg.ConfigYAML, string(schemaYAML), "", true))
}

// HandleUpdate processes the config edit form.
func (h *ConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form")
		return
	}

	// Support _method=DELETE for delete button in edit form
	if r.FormValue("_method") == "DELETE" {
		h.doDelete(w, r, id, user.ID)
		return
	}

	name := r.FormValue("name")
	yamlContent := r.FormValue("config_yaml")

	if err := h.configs.Update(id, user.ID, name, yamlContent); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to update config")
		return
	}

	go h.validateAndUpdateStatus(id, user.ID, yamlContent)

	redirectTo(w, r, fmt.Sprintf("/configs/%d", id))
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
	if err := h.configs.Delete(id, userID); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to delete config")
		return
	}
	redirectTo(w, r, "/dashboard")
}

// HandleValidate re-validates a config and returns status badge HTML (HTMX).
func (h *ConfigHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}

	status, msg := h.validateConfig(user.ID, cfg.ConfigYAML)
	_ = h.configs.UpdateStatus(id, status, msg)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, statusBadgeHTML(status, msg))
}

// HandleSync runs sync for a config and returns result HTML (HTMX).
func (h *ConfigHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}

	msg, isErr := h.runSync(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, actionResultHTML("Sync", msg, isErr))
}

// HandleWipe wipes blockers for a config and returns result HTML (HTMX).
func (h *ConfigHandler) HandleWipe(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}

	msg, isErr := h.runWipe(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, actionResultHTML("Wipe", msg, isErr))
}

// HandleListBlockers returns a blockers table partial (HTMX).
func (h *ConfigHandler) HandleListBlockers(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, ok := idFromPath(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	cfg, err := h.configs.Get(id, user.ID)
	if err != nil || cfg == nil {
		httpError(w, http.StatusNotFound, "config not found")
		return
	}

	html := h.listBlockers(r.Context(), user.ID, cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// --- internal helpers ---

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

func (h *ConfigHandler) validateConfig(userID int64, yamlContent string) (db.ConfigStatus, string) {
	appCfg, err := h.parseAppConfig(yamlContent)
	if err != nil {
		return db.ConfigStatusInvalid, err.Error()
	}

	token, err := h.getToken(userID)
	if err != nil {
		return db.ConfigStatusInvalid, err.Error()
	}

	ctx := context.Background()
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
	status, msg := h.validateConfig(userID, yamlContent)
	_ = h.configs.UpdateStatus(configID, status, msg)
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

	lookback := appCfg.Shared.LookbackDays
	lookahead := appCfg.Shared.LookaheadDays
	rangeStart, rangeEnd := dateRange(lookback, lookahead)

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

	return fmt.Sprintf("Sync complete. Created %d blocker event(s) in %d days checked.", count, len(*tgifMapping)), false
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
		return `<p><em>No blockers found in the date range.</em></p>`
	}

	rows := ""
	for _, b := range blockers {
		rows += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td></tr>`,
			b.Start.Format("2006-01-02 15:04"),
			b.Summary,
			b.ID,
		)
	}
	return fmt.Sprintf(`
<table>
  <thead><tr><th>Date</th><th>Summary</th><th>Event ID</th></tr></thead>
  <tbody>%s</tbody>
</table>`, rows)
}

// --- HTML templates as strings ---

func statusBadgeHTML(status db.ConfigStatus, msg string) string {
	badge := statusBadge(status)
	if msg != "" {
		return fmt.Sprintf(`%s <small style="color:var(--pico-muted-color)">%s</small>`, badge, msg)
	}
	return badge
}

func actionResultHTML(action, msg string, isErr bool) string {
	color := "green"
	if isErr {
		color = "red"
	}
	return fmt.Sprintf(`<div style="padding:0.5rem;border-left:3px solid %s;margin-top:0.5rem">
  <strong>%s result:</strong> %s
</div>`, color, action, msg)
}

func configDetailHTML(cfg *db.Config) string {
	badge := statusBadgeHTML(cfg.Status, cfg.StatusMessage)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <script src="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js" defer></script>
</head>
<body>
<main class="container">
  <nav>
    <ul><li><a href="/dashboard">&#8592; Dashboard</a></li></ul>
    <ul><li><a href="/logout">Logout</a></li></ul>
  </nav>
  <hgroup>
    <h2>%s</h2>
    <p>Schema: %s &nbsp;|&nbsp; Status: <span id="status-badge">%s</span></p>
  </hgroup>

  <div style="display:flex;gap:0.5rem;flex-wrap:wrap;margin-bottom:1rem">
    <button hx-post="/configs/%d/validate" hx-target="#status-badge" hx-swap="innerHTML" class="outline">
      Validate
    </button>
    <button hx-post="/configs/%d/sync" hx-target="#action-result" hx-swap="innerHTML">
      Sync
    </button>
    <button hx-post="/configs/%d/wipe" hx-target="#action-result" hx-swap="innerHTML" class="outline secondary">
      Wipe Blockers
    </button>
    <button hx-get="/configs/%d/blockers" hx-target="#blockers-panel" hx-swap="innerHTML" class="outline">
      List Blockers
    </button>
    <a href="/configs/%d/edit" role="button" class="outline">Edit</a>
  </div>

  <div id="action-result"></div>

  <details>
    <summary>Config YAML</summary>
    <pre><code>%s</code></pre>
  </details>

  <div id="blockers-panel" style="margin-top:1rem"></div>
</main>
</body>
</html>`,
		cfg.Name, cfg.Name, cfg.SchemaVersion, badge,
		cfg.ID, cfg.ID, cfg.ID, cfg.ID, cfg.ID,
		cfg.ConfigYAML,
	)
}

func configFormHTML(title, name, yamlContent, schemaYAML, formErr string, isEdit bool) string {
	errHTML := ""
	if formErr != "" {
		errHTML = fmt.Sprintf(`<div style="color:red;padding:0.5rem;border-left:3px solid red">%s</div>`, formErr)
	}

	placeholder := `shared:
  lookbackDays: 20
  lookaheadDays: 60

readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
    - today: [isTheFirstBusinessDayOfTheMonth]
    - today: [isTheLastBusinessDayOfTheMonth]
    - tomorrow: [isNonBusinessDay]

writeTo:
  googleCalendar:
    id: "your-calendar-id@group.calendar.google.com"
    ifTodayIsFreezeDay:
      default:
        summary: "FREEZE DAY - No Deployments"`

	deleteBtn := ""
	if isEdit {
		deleteBtn = `<button type="submit" name="_method" value="DELETE" class="outline contrast" onclick="return confirm('Delete this config?')">Delete</button>`
	}

	_ = schemaYAML // schema shown as tooltip/details in future iterations

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <script src="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js" defer></script>
</head>
<body>
<main class="container">
  <nav>
    <ul><li><a href="/dashboard">&#8592; Dashboard</a></li></ul>
    <ul><li><a href="/logout">Logout</a></li></ul>
  </nav>
  <h2>%s</h2>
  %s
  <form method="POST">
    <label for="name">Config Name
      <input type="text" id="name" name="name" value="%s" placeholder="My team freeze config" required>
    </label>
    <label for="config_yaml">Config YAML
      <textarea id="config_yaml" name="config_yaml" rows="20" placeholder="%s" style="font-family:monospace">%s</textarea>
    </label>
    <div style="display:flex;gap:0.5rem">
      <button type="submit">Save</button>
      %s
      <a href="/dashboard" role="button" class="outline secondary">Cancel</a>
    </div>
  </form>
</main>
</body>
</html>`, title, title, errHTML, name, placeholder, yamlContent, deleteBtn)
}
