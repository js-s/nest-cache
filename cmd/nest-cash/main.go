package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/user/nest-cash/internal/auth"
	dbmigrate "github.com/user/nest-cash/internal/db"
)

type application struct {
	db        *sql.DB
	staticDir string
	auth      *auth.Handler
	resolver  *auth.Resolver
}

// minSessionSecret is the startup presence gate for auth routes.
// The opaque scheme stores sha256(token) and signs nothing with the
// secret today; length-gating keeps the Fly 64-char contract fail-closed
// for the future HMAC domain separator.
const minSessionSecret = 32

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}

	var db *sql.DB
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		var err error
		db, err = sql.Open("pgx", databaseURL)
		if err != nil {
			log.Fatalf("open database: %v", err)
		}
		db.SetMaxOpenConns(5)
		db.SetMaxIdleConns(5)
		db.SetConnMaxIdleTime(5 * time.Minute)
		defer db.Close()

		migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer migrateCancel()
		if err := dbmigrate.Migrate(migrateCtx, db); err != nil {
			log.Fatalf("apply migrations: %v", err)
		}
	}

	// Auth wiring: fail-closed when a database is configured — weak or
	// missing SESSION_SECRET kills boot instead of serving unauthed API.
	// Without DATABASE_URL the server stays degraded (/healthz only).
	var authHandler *auth.Handler
	var authResolver *auth.Resolver
	if db != nil {
		if len(os.Getenv("SESSION_SECRET")) < minSessionSecret {
			log.Fatalf("auth disabled: SESSION_SECRET must be at least %d characters", minSessionSecret)
		}
		store := auth.NewStore(db)
		authResolver = auth.NewResolver(store)
		authHandler = auth.NewHandler(store, prodSecure())
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: (application{db: db, staticDir: staticDir, auth: authHandler, resolver: authResolver}).routes(),
	}

	log.Printf("nest-cash listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (a application) healthz(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"database": "not_configured",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		log.Printf("health check: database unavailable: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"database": "unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"database": "ok",
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// prodSecure reports whether Secure cookies are on for non-TLS requests.
// True on Fly (FLY_APP_NAME) or explicit production env; localhost dev
// stays false so http:// login works. Per-request TLS always upgrades.
func prodSecure() bool {
	if os.Getenv("FLY_APP_NAME") != "" {
		return true
	}
	switch os.Getenv("APP_ENV") {
	case "production", "prod":
		return true
	}
	switch os.Getenv("GO_ENV") {
	case "production", "prod":
		return true
	}
	return false
}
