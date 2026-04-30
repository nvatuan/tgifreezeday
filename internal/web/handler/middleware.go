package handler

import (
	"context"
	"net/http"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/nvat/tgifreezeday/internal/session"
)

type contextKey string

const userCtxKey contextKey = "user"

// RequireAuth redirects to /login if the user is not authenticated.
// On success, stores the *db.User in the request context.
func RequireAuth(users *db.UserStore, secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := session.GetUserID(r, secret)
		if !ok {
			redirectTo(w, r, "/login")
			return
		}
		user, err := users.GetByID(userID)
		if err != nil || user == nil {
			logging.GetLogger().WithField("user_id", userID).Warn("session references unknown user, clearing")
			session.Clear(w)
			redirectTo(w, r, "/login")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(userCtxKey).(*db.User)
	return u
}
