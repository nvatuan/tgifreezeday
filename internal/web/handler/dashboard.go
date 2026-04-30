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

	rows := ""
	if len(cfgs) == 0 {
		rows = `<tr><td colspan="4" style="text-align:center;color:var(--pico-muted-color)">No configs yet. <a href="/configs/new">Create one</a>.</td></tr>`
	} else {
		for _, c := range cfgs {
			badge := statusBadge(c.Status)
			rows += fmt.Sprintf(`
			<tr>
				<td><a href="/configs/%d">%s</a></td>
				<td>%s</td>
				<td>%s</td>
				<td>
					<a href="/configs/%d" role="button" class="outline" style="padding:0.2rem 0.6rem;font-size:0.8rem">View</a>
					<a href="/configs/%d/edit" role="button" class="outline secondary" style="padding:0.2rem 0.6rem;font-size:0.8rem">Edit</a>
				</td>
			</tr>`, c.ID, html.EscapeString(c.Name), badge, html.EscapeString(c.SchemaVersion), c.ID, c.ID)
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TGI Freeze Day &#8211; Dashboard</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
</head>
<body>
  <main class="container">
    <nav>
      <ul><li><strong>TGI Freeze Day</strong></li></ul>
      <ul>
        <li>%s</li>
        <li>%s</li>
      </ul>
    </nav>
    <hgroup>
      <h2>Configs</h2>
      <p>Manage your freeze day rules and calendars.</p>
    </hgroup>
    <a href="/configs/new" role="button" style="margin-bottom:1rem">+ New Config</a>
    <table>
      <thead><tr><th>Name</th><th>Status</th><th>Schema</th><th>Actions</th></tr></thead>
      <tbody>%s</tbody>
    </table>
  </main>
</body>
</html>`, html.EscapeString(greeting), logoutForm, rows)
}

func statusBadge(status db.ConfigStatus) string {
	color := map[db.ConfigStatus]string{
		db.ConfigStatusValid:        "green",
		db.ConfigStatusInvalid:      "red",
		db.ConfigStatusUnauthorized: "orange",
		db.ConfigStatusPending:      "gray",
	}[status]
	return fmt.Sprintf(`<span style="color:%s;font-weight:bold">%s</span>`, color, html.EscapeString(string(status)))
}

// logoutForm is a small inline POST form used in every nav bar.
const logoutForm = `<form method="POST" action="/logout" style="margin:0;display:inline"><button type="submit" class="outline" style="padding:0.2rem 0.8rem">Logout</button></form>`
