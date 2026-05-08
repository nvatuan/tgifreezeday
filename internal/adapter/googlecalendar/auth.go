package googlecalendar

import (
	"context"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"

	"github.com/nvat/tgifreezeday/internal/logging"
)

const (
	oauthScopeEmail   = "https://www.googleapis.com/auth/userinfo.email"
	oauthScopeProfile = "https://www.googleapis.com/auth/userinfo.profile"
)

// TokenStore is implemented by db.TokenStore and allows writing refreshed tokens back to persistent storage.
type TokenStore interface {
	Upsert(userID int64, token *oauth2.Token) error
}

// NewOAuthConfig builds an OAuth2 config from environment variables.
// Reads GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, GOOGLE_OAUTH_REDIRECT_URL.
func NewOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		Scopes: []string{
			calendar.CalendarScope,
			oauthScopeEmail,
			oauthScopeProfile,
		},
		Endpoint: google.Endpoint,
	}
}

// NewHTTPClientFromToken creates an authorized HTTP client from a stored token.
// Tokens refreshed in memory are not persisted; use NewHTTPClientWithPersistence for
// long-lived operations that must survive refresh token rotation.
func NewHTTPClientFromToken(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) *http.Client {
	return cfg.Client(ctx, token)
}

// persistingTokenSource wraps an oauth2.TokenSource and writes refreshed tokens back to the DB.
// This prevents auth failures when Google rotates refresh tokens.
type persistingTokenSource struct {
	base    oauth2.TokenSource
	userID  int64
	store   TokenStore
	current *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	t, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if t.AccessToken != p.current.AccessToken {
		if err := p.store.Upsert(p.userID, t); err != nil {
			logging.GetLogger().WithError(err).Warn("failed to persist refreshed OAuth token")
		}
		p.current = t
	}
	return t, nil
}

// NewHTTPClientWithPersistence creates an authorized HTTP client that writes refreshed tokens
// back to the DB via store. Use this for all Google Calendar API calls made on behalf of a stored user.
func NewHTTPClientWithPersistence(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, userID int64, store TokenStore) *http.Client {
	base := cfg.TokenSource(ctx, token)
	src := &persistingTokenSource{base: base, userID: userID, store: store, current: token}
	return oauth2.NewClient(ctx, src)
}
