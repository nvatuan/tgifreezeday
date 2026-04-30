package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/session"
	"golang.org/x/oauth2"
)

const oauthStateCookie = "oauth_state"

type AuthHandler struct {
	users    *db.UserStore
	tokens   *db.TokenStore
	oauthCfg *oauth2.Config
	secret   []byte
}

func NewAuthHandler(users *db.UserStore, tokens *db.TokenStore, secret []byte) *AuthHandler {
	return &AuthHandler{
		users:    users,
		tokens:   tokens,
		oauthCfg: googlecalendar.NewOAuthConfig(),
		secret:   secret,
	}
}

func (h *AuthHandler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if _, ok := session.GetUserID(r, h.secret); ok {
		redirectTo(w, r, "/dashboard")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, loginPageHTML)
}

func (h *AuthHandler) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	state := hex.EncodeToString(stateBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((5 * time.Minute).Seconds()),
	})

	url := h.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Verify CSRF state
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		httpError(w, http.StatusBadRequest, "invalid OAuth state")
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", MaxAge: -1, Path: "/"})

	code := r.URL.Query().Get("code")
	if code == "" {
		httpError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	token, err := h.oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to exchange code: "+err.Error())
		return
	}

	// Fetch user info
	info, err := fetchUserInfo(h.oauthCfg, token)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to fetch user info: "+err.Error())
		return
	}

	// Upsert user and token
	user, err := h.users.Upsert(info.ID, info.Email, info.Name)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to upsert user: "+err.Error())
		return
	}
	if err := h.tokens.Upsert(user.ID, token); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to store token: "+err.Error())
		return
	}

	session.SetUserID(w, h.secret, user.ID)
	redirectTo(w, r, "/dashboard")
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	session.Clear(w)
	redirectTo(w, r, "/login")
}

type userInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func fetchUserInfo(cfg *oauth2.Config, token *oauth2.Token) (*userInfo, error) {
	client := cfg.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info userInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.ID == "" {
		return nil, fmt.Errorf("empty user ID from userinfo endpoint")
	}
	return &info, nil
}

const loginPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TGI Freeze Day &#8211; Login</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <style>
    body { display: flex; justify-content: center; align-items: center; min-height: 100vh; }
    .login-card { text-align: center; max-width: 400px; width: 100%; }
    h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
    p { color: var(--pico-muted-color); margin-bottom: 2rem; }
  </style>
</head>
<body>
  <main class="container">
    <div class="login-card">
      <h1>TGI Freeze Day</h1>
      <p>Manage production freeze day blockers on your team calendar.</p>
      <a href="/oauth/start" role="button">Login with Google</a>
    </div>
  </main>
</body>
</html>`
