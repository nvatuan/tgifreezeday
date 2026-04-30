package handler

import "net/http"

func redirectTo(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}
