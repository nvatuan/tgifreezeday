package googlecalendar

import (
	"context"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

const (
	oauthScopeEmail   = "https://www.googleapis.com/auth/userinfo.email"
	oauthScopeProfile = "https://www.googleapis.com/auth/userinfo.profile"
)

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
func NewHTTPClientFromToken(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) *http.Client {
	return cfg.Client(ctx, token)
}
