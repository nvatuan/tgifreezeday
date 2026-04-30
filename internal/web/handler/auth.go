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
	secure   bool
}

func NewAuthHandler(users *db.UserStore, tokens *db.TokenStore, secret []byte, secure bool) *AuthHandler {
	return &AuthHandler{
		users:    users,
		tokens:   tokens,
		oauthCfg: googlecalendar.NewOAuthConfig(),
		secret:   secret,
		secure:   secure,
	}
}

func (h *AuthHandler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
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
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((5 * time.Minute).Seconds()),
	})

	url := h.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		httpError(w, http.StatusBadRequest, "invalid OAuth state")
		return
	}
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

	info, err := fetchUserInfo(h.oauthCfg, token)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to fetch user info: "+err.Error())
		return
	}

	user, err := h.users.Upsert(info.ID, info.Email, info.Name)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to upsert user: "+err.Error())
		return
	}
	if err := h.tokens.Upsert(user.ID, token); err != nil {
		httpError(w, http.StatusInternalServerError, "failed to store token: "+err.Error())
		return
	}

	session.SetUserID(w, h.secret, user.ID, h.secure)
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
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TGI Freeze Day</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <style>
    html, body { height: 100%; margin: 0; }
    body {
      display: flex;
      align-items: center;
      justify-content: center;
      background: var(--pico-background-color);
    }
    .login-wrap {
      width: 100%;
      max-width: 420px;
      padding: 1rem;
    }
    .login-card {
      background: var(--pico-card-background-color);
      border: 1px solid var(--pico-card-border-color);
      border-radius: 1rem;
      padding: 2.5rem 2rem;
      text-align: center;
      box-shadow: 0 8px 32px rgba(0,0,0,0.3);
    }
    .login-card .icon { font-size: 2rem; letter-spacing: 0.15em; margin-bottom: 0.75rem; }
    .login-card h1 { font-size: 1.6rem; margin: 0 0 0.5rem; }
    .login-card p { color: var(--pico-muted-color); margin-bottom: 2rem; font-size: 0.95rem; }
    .google-btn {
      display: inline-flex;
      align-items: center;
      gap: 0.6rem;
      background: #4285F4;
      color: #fff;
      border: none;
      border-radius: 0.5rem;
      padding: 0.75rem 1.5rem;
      font-size: 0.95rem;
      font-weight: 500;
      text-decoration: none;
      cursor: pointer;
      transition: background 0.15s;
    }
    .google-btn:hover { background: #3367D6; color: #fff; }
    .google-btn svg { width: 18px; height: 18px; }
  </style>
</head>
<body>
  <div class="login-wrap">
    <div class="login-card">
      <div class="icon">🙏🧔🏽‍♀️🧊🗓️️</div>
      <h1>TGI Freeze Day</h1>
      <p>Manage production freeze day blockers<br>on your team calendar.</p>
      <a href="/oauth/start" class="google-btn">
        <svg viewBox="0 0 18 18" xmlns="http://www.w3.org/2000/svg">
          <path fill="#fff" d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.717v2.258h2.908c1.702-1.567 2.684-3.875 2.684-6.615z"/>
          <path fill="#fff" d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z"/>
          <path fill="#fff" d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z"/>
          <path fill="#fff" d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z"/>
        </svg>
        Sign in with Google
      </a>
    </div>
  </div>
</body>
</html>`
