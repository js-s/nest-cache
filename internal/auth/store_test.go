package auth

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	dbmigrate "github.com/user/nest-cash/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL unset; store test needs real Postgres")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("database unreachable, skipping: %v", err)
	}
	if err := dbmigrate.Migrate(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func createTestUser(t *testing.T, ctx context.Context, s *Store, db *sql.DB) User {
	t.Helper()
	email := "StoreCase+" + time.Now().Format("150405.000000") + "@example.com"
	user, ok, err := s.CreateUser(ctx, "  "+email+" ", "hash-1")
	if err != nil || !ok {
		t.Fatalf("CreateUser = %+v, %v, %v", user, ok, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})
	return user
}

func TestCreateUserFindUser(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)

	user := createTestUser(t, ctx, s, db)
	if user.Email != NormalizeEmail(user.Email) {
		t.Fatalf("email not normalized: %q", user.Email)
	}

	// Case-insensitive duplicate must not create.
	if _, ok, err := s.CreateUser(ctx, NormalizeEmail(user.Email), "hash-2"); err != nil || ok {
		t.Fatalf("duplicate CreateUser = ok %v, err %v", ok, err)
	}

	found, ok, err := s.FindUserByEmail(ctx, user.Email)
	if err != nil || !ok || found.ID != user.ID {
		t.Fatalf("FindUserByEmail = %+v, %v, %v", found, ok, err)
	}
	if _, ok, err := s.FindUserByEmail(ctx, "nobody@example.com"); err != nil || ok {
		t.Fatalf("missing email should miss: ok %v, err %v", ok, err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)

	user := createTestUser(t, ctx, s, db)
	live := "live-" + user.ID
	expired := "expired-" + user.ID
	if err := s.CreateSession(ctx, live, user.ID, time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("CreateSession live: %v", err)
	}
	if err := s.CreateSession(ctx, expired, user.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}

	if _, _, ok, err := s.FindSessionUser(ctx, live); err != nil || !ok {
		t.Fatalf("live session should resolve: ok %v, err %v", ok, err)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, expired); err != nil || ok {
		t.Fatalf("expired session must not resolve: ok %v, err %v", ok, err)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, "unknown"); err != nil || ok {
		t.Fatalf("unknown session must not resolve: ok %v, err %v", ok, err)
	}

	// Sliding refresh keeps a live session resolvable...
	if err := s.TouchSession(ctx, live, time.Now().Add(60*24*time.Hour)); err != nil {
		t.Fatalf("TouchSession live: %v", err)
	}
	// ...and actually moves expires_at forward (not a silent no-op).
	var touched time.Time
	if err := db.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token_hash = $1`, live).Scan(&touched); err != nil {
		t.Fatalf("read touched expiry: %v", err)
	}
	if !touched.After(time.Now().Add(59 * 24 * time.Hour)) {
		t.Fatalf("TouchSession did not extend expiry: got %v", touched)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, live); err != nil || !ok {
		t.Fatalf("touched session should resolve: ok %v, err %v", ok, err)
	}
	// Touching an expired session is a no-op: still unresolvable.
	if err := s.TouchSession(ctx, expired, time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("TouchSession expired: %v", err)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, expired); err != nil || ok {
		t.Fatalf("touched expired session must not resolve: ok %v, err %v", ok, err)
	}

	if err := s.DeleteSession(ctx, live); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, live); err != nil || ok {
		t.Fatalf("deleted session must not resolve: ok %v, err %v", ok, err)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)

	user := createTestUser(t, ctx, s, db)
	expired := "expired-" + user.ID
	if err := s.CreateSession(ctx, expired, user.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}

	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil || n < 1 {
		t.Fatalf("DeleteExpiredSessions = %d, %v", n, err)
	}
	if _, _, ok, err := s.FindSessionUser(ctx, expired); err != nil || ok {
		t.Fatalf("purged session must not resolve: ok %v, err %v", ok, err)
	}
}
