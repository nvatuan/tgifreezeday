package handler

import (
	"fmt"
	"html"
	"net/http"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
)

type DashboardHandler struct {
	configs *db.ConfigStore
}

func NewDashboardHandler(configs *db.ConfigStore) *DashboardHandler {
	return &DashboardHandler{configs: configs}
}

func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	cfgs, err := h.configs.ListByUser(user.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to load configs")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardPageHTML(user.DisplayName, user.Email, cfgs))
}

func dashboardPageHTML(displayName, email string, cfgs []*db.Config) string {
	greeting := displayName
	if greeting == "" {
		greeting = email
	}

	configCards := ""
	if len(cfgs) == 0 {
		configCards = `
		<div style="text-align:center;padding:3rem;color:var(--pico-muted-color)">
		  <p style="font-size:2rem;margin-bottom:0.5rem">📭</p>
		  <p>No configs yet.</p>
		  <a href="/configs/new" role="button">Create your first config</a>
		</div>`
	} else {
		for _, c := range cfgs {
			badge := statusBadge(c.Status)
			configCards += fmt.Sprintf(`
			<article style="margin-bottom:1rem;padding:1.25rem 1.5rem">
			  <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:0.5rem">
			    <div>
			      <strong><a href="/configs/%d" style="text-decoration:none">%s</a></strong>
			      <div style="margin-top:0.25rem;font-size:0.85rem;color:var(--pico-muted-color)">schema: %s</div>
			    </div>
			    <div style="display:flex;align-items:center;gap:0.75rem">
			      %s
			      <a href="/configs/%d" role="button" class="outline" style="padding:0.3rem 0.8rem;font-size:0.85rem;margin:0">View</a>
			      <a href="/configs/%d/edit" role="button" class="outline secondary" style="padding:0.3rem 0.8rem;font-size:0.85rem;margin:0">Edit</a>
			    </div>
			  </div>
			</article>`,
				c.ID, html.EscapeString(c.Name),
				html.EscapeString(c.SchemaVersion),
				badge,
				c.ID, c.ID)
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
    nav.topnav .brand { font-weight: 700; font-size: 1rem; }
    nav.topnav .user-area { display:flex; align-items:center; gap:1rem; font-size:0.9rem; color:var(--pico-muted-color); }
    .page-content { max-width: 860px; margin: 2rem auto; padding: 0 1.5rem; }
  </style>
</head>
<body>
  <nav class="topnav">
    <span class="brand">🙏🧔🏽‍♀️🧊🗓️ TGI Freeze Day</span>
    <div class="user-area">
      <span>%s</span>
      %s
    </div>
  </nav>
  <div class="page-content">
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:1.5rem">
      <h2 style="margin:0">Configs</h2>
      <a href="/configs/new" role="button" style="margin:0">+ New Config</a>
    </div>
    %s
  </div>
</body>
</html>`, html.EscapeString(greeting), logoutForm, configCards)
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
