package handler

import (
	"fmt"
	"net/http"

	"github.com/nvat/tgifreezeday/internal/version"
)

func redirectTo(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}

func pageFooterHTML() string {
	return fmt.Sprintf(`<footer style="text-align:center;padding:1.5rem;font-size:0.75rem;color:var(--pico-muted-color)">%s (%s)</footer>`,
		version.Version, version.Commit)
}
