package handler

import (
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	appconfig "github.com/nvat/tgifreezeday/internal/config"
	"golang.org/x/oauth2"
)

type DashboardHandler struct {
	configs  *db.ConfigStore
	users    *db.UserStore
	tokens   *db.TokenStore
	oauthCfg *oauth2.Config
}

func NewDashboardHandler(configs *db.ConfigStore, users *db.UserStore, tokens *db.TokenStore, oauthCfg *oauth2.Config) *DashboardHandler {
	return &DashboardHandler{
		configs:  configs,
		users:    users,
		tokens:   tokens,
		oauthCfg: oauthCfg,
	}
}

func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	currentUser := userFromContext(r.Context())

	// Resolve filter
	var filterUserID *int64
	filterMine := r.URL.Query().Get("filter") == "mine"
	authorParam := r.URL.Query().Get("author")
	if filterMine {
		filterUserID = &currentUser.ID
	} else if authorParam != "" {
		if id, err := strconv.ParseInt(authorParam, 10, 64); err == nil {
			filterUserID = &id
		}
	}

	cfgs, err := h.configs.ListAllWithAuthor(filterUserID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to load configs")
		return
	}

	allUsers, _ := h.users.ListAll()

	// Fetch current user's calendar names in one API call for display
	calNames := map[string]string{}
	if token, err := h.tokens.Get(currentUser.ID); err == nil && token != nil {
		if cals, err := googlecalendar.ListWritableCalendars(r.Context(), h.oauthCfg, token); err == nil {
			for _, c := range cals {
				calNames[c.ID] = c.Summary
			}
		}
	}

	// Build rows
	rows := make([]dashRow, 0, len(cfgs))
	for _, c := range cfgs {
		author := c.AuthorDisplayName
		if author == "" {
			author = c.AuthorEmail
		}
		calID := ""
		if parsed, err := appconfig.LoadWithDefaultFromByteArray([]byte(c.ConfigYAML)); err == nil {
			calID = parsed.WriteTo.GoogleCalendar.ID
		}
		calDisplay := calID
		if name, ok := calNames[calID]; ok && name != "" {
			calDisplay = name
		}
		rows = append(rows, dashRow{
			ID:           c.ID,
			Name:         c.Name,
			Schema:       c.SchemaVersion,
			Status:       c.Status,
			Author:       author,
			CalendarID:   calID,
			CalendarName: calDisplay,
		})
	}

	greeting := currentUser.DisplayName
	if greeting == "" {
		greeting = currentUser.Email
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardPageHTML(greeting, rows, allUsers, filterMine, authorParam)) //nolint:errcheck
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

type dashRow struct {
	ID           int64
	Name         string
	Schema       string
	Status       db.ConfigStatus
	Author       string
	CalendarID   string
	CalendarName string
}

func dashboardPageHTML(greeting string, rows []dashRow, allUsers []*db.User, filterMine bool, authorParam string) string {
	// --- filter bar ---
	btnStyle := `style="padding:0.3rem 0.9rem;font-size:0.85rem;margin:0"`

	allClass, mineClass := `class="outline"`, `class="outline"`
	if !filterMine && authorParam == "" {
		allClass = `class=""`
	} else if filterMine {
		mineClass = `class=""`
	}

	authorOpts := `<option value="">By author…</option>`
	for _, u := range allUsers {
		label := u.DisplayName
		if label == "" {
			label = u.Email
		}
		selected := ""
		if authorParam == strconv.FormatInt(u.ID, 10) {
			selected = " selected"
		}
		authorOpts += fmt.Sprintf(`<option value="%d"%s>%s</option>`,
			u.ID, selected, html.EscapeString(trunc(label, 40)))
	}

	filterBar := fmt.Sprintf(`
<div style="display:flex;gap:0.5rem;align-items:center;margin-bottom:0.75rem">
  <a href="/dashboard" role="button" %s %s>All</a>
  <a href="/dashboard?filter=mine" role="button" %s %s>Mine</a>
  <select onchange="this.value?location.href='/dashboard?author='+this.value:location.href='/dashboard'"
          style="margin:0;padding:0.3rem 0.5rem;font-size:0.85rem;width:180px;flex-shrink:0">
    %s
  </select>
</div>
<div style="font-size:0.82rem;color:var(--pico-muted-color);padding:0.4rem 0 0.6rem;border-bottom:1px solid var(--pico-card-border-color);margin-bottom:0.75rem">
  %d config(s)
</div>`, allClass, btnStyle, mineClass, btnStyle, authorOpts, len(rows))

	// --- config cards ---
	cards := ""
	if len(rows) == 0 {
		cards = `<div style="text-align:center;padding:3rem;color:var(--pico-muted-color)">
		  <p style="font-size:2rem;margin-bottom:0.5rem">📭</p>
		  <p>No configs found.</p>
		  <a href="/configs/new" role="button">Create your first config</a>
		</div>`
	} else {
		for _, r := range rows {
			badge := statusBadge(r.Status)
			calDisplay := r.CalendarName
			if calDisplay == "" {
				calDisplay = r.CalendarID
			}
			meta := fmt.Sprintf(`schema: <strong>%s</strong> &nbsp;·&nbsp; by: %s &nbsp;·&nbsp; 📅 %s`,
				html.EscapeString(r.Schema),
				html.EscapeString(trunc(r.Author, 40)),
				html.EscapeString(trunc(calDisplay, 50)),
			)
			cards += fmt.Sprintf(`
<article style="margin-bottom:0.6rem;padding:0.9rem 1.2rem">
  <div style="display:flex;align-items:flex-start;justify-content:space-between;gap:0.75rem">
    <strong style="font-size:1rem"><a href="/configs/%d" style="text-decoration:none">%s</a></strong>
    <div style="display:flex;align-items:center;gap:0.5rem;flex-shrink:0">
      %s
      <a href="/configs/%d" role="button" class="outline" style="padding:0.2rem 0.6rem;font-size:0.82rem;margin:0">View</a>
      <a href="/configs/%d/edit" role="button" class="outline secondary" style="padding:0.2rem 0.6rem;font-size:0.82rem;margin:0">Edit</a>
    </div>
  </div>
  <div style="margin-top:0.3rem;font-size:0.82rem;color:var(--pico-muted-color)">%s</div>
</article>`,
				r.ID, html.EscapeString(trunc(r.Name, 50)),
				badge,
				r.ID, r.ID,
				meta)
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <style>
    nav.topnav { background: var(--pico-card-background-color); border-bottom: 1px solid var(--pico-card-border-color); padding: 0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; }
    nav.topnav .brand { font-weight:700; font-size:1rem; text-decoration:none; color:inherit; }
    nav.topnav .user-area { display:flex; align-items:center; gap:1rem; font-size:0.9rem; color:var(--pico-muted-color); }
    .page-content { max-width: 900px; margin: 2rem auto; padding: 0 1.5rem; }
    article a[role=button] { white-space:nowrap; }
  </style>
</head>
<body>
  <nav class="topnav">
    <a href="/dashboard" class="brand">🙏🧔🏽‍♀️🧊🗓️ TGI Freeze Day</a>
    <div class="user-area">
      <span>%s</span>
      %s
    </div>
  </nav>
  <div class="page-content">
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:1rem">
      <h2 style="margin:0">Configs</h2>
      <a href="/configs/new" role="button" style="margin:0">+ New Config</a>
    </div>
    %s
    %s
  </div>
</body>
</html>`, html.EscapeString(greeting), logoutForm, filterBar, cards)
}

func statusBadge(status db.ConfigStatus) string {
	style := map[db.ConfigStatus]string{
		db.ConfigStatusValid:        "background:#1a4731;color:#4ade80;border:1px solid #166534",
		db.ConfigStatusInvalid:      "background:#4a1122;color:#f87171;border:1px solid #7f1d1d",
		db.ConfigStatusUnauthorized: "background:#4a3300;color:#fb923c;border:1px solid #7c2d12",
		db.ConfigStatusPending:      "background:#1f2937;color:#9ca3af;border:1px solid #374151",
	}[status]
	return fmt.Sprintf(`<span style="padding:0.2rem 0.6rem;border-radius:999px;font-size:0.78rem;font-weight:600;%s">%s</span>`,
		style, html.EscapeString(string(status)))
}

// logoutForm is a small inline POST form used in every nav bar.
const logoutForm = `<form method="POST" action="/logout" style="margin:0;display:inline"><button type="submit" class="outline" style="padding:0.25rem 0.75rem;font-size:0.85rem;margin:0">Logout</button></form>`
