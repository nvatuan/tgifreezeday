package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/nvat/tgifreezeday/internal/web/handler"
)

func main() {
	log := logging.GetLogger()

	// Validate required env vars
	required := []string{
		"GOOGLE_OAUTH_CLIENT_ID",
		"GOOGLE_OAUTH_CLIENT_SECRET",
		"GOOGLE_OAUTH_REDIRECT_URL",
		"SESSION_SECRET",
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			log.Fatalf("required environment variable %s is not set", key)
		}
	}

	// Validate OAuth config is buildable
	oauthCfg := googlecalendar.NewOAuthConfig()
	if oauthCfg.ClientID == "" {
		log.Fatal("OAuth config invalid")
	}

	secret := []byte(os.Getenv("SESSION_SECRET"))

	// Open database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tgifreezeday.db"
	}
	database, err := db.Open(dbPath)
	if err != nil {
		log.WithError(err).Fatal("failed to open database")
	}
	defer database.Close()

	// Stores
	users := db.NewUserStore(database)
	tokens := db.NewTokenStore(database)
	configs := db.NewConfigStore(database)

	// Handlers
	authH := handler.NewAuthHandler(users, tokens, secret)
	dashH := handler.NewDashboardHandler(configs)
	cfgH := handler.NewConfigHandler(configs, tokens)

	// Auth middleware
	requireAuth := func(h http.Handler) http.Handler {
		return handler.RequireAuth(users, secret, h)
	}

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /login", authH.HandleLoginPage)
	mux.HandleFunc("GET /oauth/start", authH.HandleOAuthStart)
	mux.HandleFunc("GET /oauth/callback", authH.HandleOAuthCallback)
	mux.HandleFunc("POST /logout", authH.HandleLogout)

	// Protected routes (wrapped with auth middleware)
	mux.Handle("GET /dashboard", requireAuth(http.HandlerFunc(dashH.HandleDashboard)))

	mux.Handle("GET /configs/new", requireAuth(http.HandlerFunc(cfgH.HandleNew)))
	mux.Handle("POST /configs", requireAuth(http.HandlerFunc(cfgH.HandleCreate)))
	mux.Handle("GET /configs/{id}", requireAuth(http.HandlerFunc(cfgH.HandleDetail)))
	mux.Handle("GET /configs/{id}/edit", requireAuth(http.HandlerFunc(cfgH.HandleEdit)))
	mux.Handle("POST /configs/{id}", requireAuth(http.HandlerFunc(cfgH.HandleUpdate)))
	mux.Handle("POST /configs/{id}/delete", requireAuth(http.HandlerFunc(cfgH.HandleDelete)))

	// HTMX endpoints
	mux.Handle("POST /configs/{id}/validate", requireAuth(http.HandlerFunc(cfgH.HandleValidate)))
	mux.Handle("POST /configs/{id}/sync", requireAuth(http.HandlerFunc(cfgH.HandleSync)))
	mux.Handle("POST /configs/{id}/wipe", requireAuth(http.HandlerFunc(cfgH.HandleWipe)))
	mux.Handle("GET /configs/{id}/blockers", requireAuth(http.HandlerFunc(cfgH.HandleListBlockers)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.WithField("addr", addr).Info("Starting server")
	fmt.Printf("Server running at http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.WithError(err).Fatal("server failed")
	}
}
