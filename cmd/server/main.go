package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
	"github.com/nvat/tgifreezeday/internal/adapter/googlecalendar"
	"github.com/nvat/tgifreezeday/internal/logging"
	"github.com/nvat/tgifreezeday/internal/perm"
	"github.com/nvat/tgifreezeday/internal/web/handler"
)

func main() {
	log := logging.GetLogger()

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

	secret := []byte(os.Getenv("SESSION_SECRET"))
	// Set true when the server is behind HTTPS so cookies get Secure flag.
	httpsOnly := os.Getenv("HTTPS_ONLY") == "true"

	// BASE_PATH lets the app run under a sub-path (e.g. /tgifreezeday).
	// Must start with "/" and have no trailing slash when non-empty.
	basePath := strings.TrimRight(os.Getenv("BASE_PATH"), "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	oauthCfg := googlecalendar.NewOAuthConfig()
	if oauthCfg.ClientID == "" {
		log.Fatal("OAuth config invalid")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tgifreezeday.db"
	}
	database, err := db.Open(dbPath)
	if err != nil {
		log.WithError(err).Fatal("failed to open database")
	}
	defer database.Close() //nolint:errcheck

	users := db.NewUserStore(database)
	tokens := db.NewTokenStore(database)
	configs := db.NewConfigStore(database)

	resolver := perm.New(
		os.Getenv("POWER_USER_EMAIL_LIST"),
		os.Getenv("WRITE_USER_EMAIL_LIST"),
	)

	authH := handler.NewAuthHandler(users, tokens, secret, httpsOnly, oauthCfg, basePath)
	dashH := handler.NewDashboardHandler(configs, users, tokens, oauthCfg, basePath)
	cfgH := handler.NewConfigHandler(configs, tokens, oauthCfg, basePath)
	schemaH := handler.NewSchemaHandler(basePath)

	loginPath := basePath + "/login"
	requireAuth := func(h http.Handler) http.Handler {
		return handler.RequireAuth(users, secret, resolver, loginPath, h)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != basePath && r.URL.Path != basePath+"/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, basePath+"/dashboard", http.StatusSeeOther)
	})
	mux.HandleFunc("GET "+basePath+"/login", authH.HandleLoginPage)
	mux.HandleFunc("GET "+basePath+"/oauth/start", authH.HandleOAuthStart)
	mux.HandleFunc("GET "+basePath+"/oauth/callback", authH.HandleOAuthCallback)
	mux.HandleFunc("POST "+basePath+"/logout", authH.HandleLogout)

	mux.Handle("GET "+basePath+"/dashboard", requireAuth(http.HandlerFunc(dashH.HandleDashboard)))

	mux.Handle("GET "+basePath+"/configs/new", requireAuth(http.HandlerFunc(cfgH.HandleNew)))
	mux.Handle("POST "+basePath+"/configs", requireAuth(http.HandlerFunc(cfgH.HandleCreate)))
	mux.Handle("GET "+basePath+"/configs/{id}", requireAuth(http.HandlerFunc(cfgH.HandleDetail)))
	mux.Handle("GET "+basePath+"/configs/{id}/edit", requireAuth(http.HandlerFunc(cfgH.HandleEdit)))
	mux.Handle("POST "+basePath+"/configs/{id}", requireAuth(http.HandlerFunc(cfgH.HandleUpdate)))
	mux.Handle("POST "+basePath+"/configs/{id}/delete", requireAuth(http.HandlerFunc(cfgH.HandleDelete)))

	mux.Handle("POST "+basePath+"/configs/{id}/validate", requireAuth(http.HandlerFunc(cfgH.HandleValidate)))
	mux.Handle("POST "+basePath+"/configs/{id}/sync", requireAuth(http.HandlerFunc(cfgH.HandleSync)))
	mux.Handle("POST "+basePath+"/configs/{id}/wipe", requireAuth(http.HandlerFunc(cfgH.HandleWipe)))
	mux.Handle("GET "+basePath+"/configs/{id}/blockers", requireAuth(http.HandlerFunc(cfgH.HandleListBlockers)))

	// Schema reference (public — no auth needed, no secrets exposed)
	mux.HandleFunc("GET "+basePath+"/schema/{version}", schemaH.HandleSchemaRef)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.WithField("addr", addr).WithField("base_path", basePath).Info("Starting server")
	fmt.Printf("Server running at http://localhost%s%s\n", addr, basePath)
	if err := srv.ListenAndServe(); err != nil {
		log.WithError(err).Fatal("server failed")
	}
}
