package handler

import (
	"fmt"
	"html"
	"net/http"

	appconfig "github.com/nvat/tgifreezeday/internal/config"
)

// HandleSchemaRef renders the schema reference page for a given version.
func HandleSchemaRef(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	schemaYAML, ok := appconfig.SchemaYAML(version)
	if !ok {
		httpError(w, http.StatusNotFound, "schema version not found: "+version)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, schemaRefHTML(version, string(schemaYAML)))
}

func schemaRefHTML(version, yamlContent string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Schema %s &#8211; TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/themes/prism-tomorrow.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1/plugins/line-numbers/prism-line-numbers.min.css">
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/prism.min.js" defer></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-yaml.min.js" defer></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/plugins/line-numbers/prism-line-numbers.min.js" defer></script>
  <style>
    nav.topnav { background: var(--pico-card-background-color); border-bottom: 1px solid var(--pico-card-border-color); padding: 0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; }
    nav.topnav .brand { font-weight:700; text-decoration:none; color:inherit; }
    .page-content { max-width: 860px; margin: 2rem auto; padding: 0 1.5rem; }
    .breadcrumb { font-size:0.82rem; color:var(--pico-muted-color); margin-bottom:0.4rem; }
    .breadcrumb a { color:var(--pico-muted-color); text-decoration:none; }
    .breadcrumb a:hover { text-decoration:underline; }
    .back-btn { font-size:1.4rem; text-decoration:none; color:var(--pico-muted-color); line-height:1; }
    .back-btn:hover { color:var(--pico-color); }
    pre[class*="language-"] { line-height:1.5 !important; white-space:pre-wrap; word-break:break-word; font-size:0.84rem; border-radius:0.5rem; }
    .line-numbers .line-numbers-rows > span::before { line-height:1.5; }
    .readonly-badge { background:#1f2937; color:#9ca3af; border:1px solid #374151; padding:0.15rem 0.5rem; border-radius:999px; font-size:0.75rem; font-weight:600; }
  </style>
</head>
<body>
<nav class="topnav">
  <a href="/dashboard" class="brand">🛑 TGI Freeze Day</a>
</nav>
<div class="page-content">
  <div class="breadcrumb">Schema &rsaquo; %s</div>
  <div style="display:flex;align-items:center;gap:0.75rem;margin-bottom:0.5rem">
    <a href="javascript:history.back()" class="back-btn" title="Go back">&#8592;</a>
    <h2 style="margin:0">Config Schema <code>%s</code></h2>
    <span class="readonly-badge">read-only reference</span>
  </div>
  <p style="color:var(--pico-muted-color);font-size:0.9rem;margin-bottom:1.5rem">
    This documents every field accepted in the Config YAML editor, including types, constraints, and descriptions.
  </p>
  <pre class="line-numbers language-yaml"><code>%s</code></pre>
</div>
</body>
</html>`,
		html.EscapeString(version),
		html.EscapeString(version),
		html.EscapeString(version),
		html.EscapeString(yamlContent),
	)
}
