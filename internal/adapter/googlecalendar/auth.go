package googlecalendar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

const tokenCacheSubPath = "tgifreezeday/token.json"

// NewOAuthHTTPClient returns an HTTP client authorized as the user via OAuth2.
// On first run it opens a browser for consent and caches the token locally.
// Subsequent runs load the cached token and refresh it automatically.
func NewOAuthHTTPClient(ctx context.Context, credentialsPath string) (*http.Client, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OAuth credentials: %w", err)
	}

	cachePath, err := resolveTokenCachePath()
	if err != nil {
		return nil, err
	}

	token, err := loadToken(cachePath)
	if err != nil {
		token, err = runBrowserOAuthFlow(config)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OAuth token: %w", err)
		}
		if err := saveToken(cachePath, token); err != nil {
			logger.WithError(err).Warn("Failed to cache OAuth token — you may be prompted again next run")
		}
	}

	return config.Client(ctx, token), nil
}

// resolveTokenCachePath returns the path to the cached OAuth token.
// Override with GOOGLE_OAUTH_TOKEN_CACHE_PATH env var.
func resolveTokenCachePath() (string, error) {
	if p := os.Getenv("GOOGLE_OAUTH_TOKEN_CACHE_PATH"); p != "" {
		return p, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve token cache path: %w", err)
	}
	return filepath.Join(configDir, tokenCacheSubPath), nil
}

func runBrowserOAuthFlow(config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate state token: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Println("Opening browser for Google authorization...")
	fmt.Printf("If the browser does not open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth state mismatch — possible CSRF attempt")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no authorization code in callback")
			return
		}
		fmt.Fprintln(w, "Authorization successful! You can close this tab.")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener) //nolint:errcheck
	defer server.Close()

	select {
	case code := <-codeCh:
		token, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authorization timed out after 2 minutes")
	}
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode cached token: %w", err)
	}
	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create token cache directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to write token cache: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
