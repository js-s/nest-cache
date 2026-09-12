package categories

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
		t.Skip("DATABASE_URL unset; categories store test needs real Postgres")
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

func createTestUser(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	var id string
	email := "CatCase+" + time.Now().Format("150405.000000") + "@example.com"
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, "hash-1",
	).Scan(&id); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func countRows(t *testing.T, ctx context.Context, s *Store, userID string) int {
	t.Helper()
	n, err := s.count(ctx, userID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestEnsureSeededIdempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)

	if err := s.EnsureSeeded(ctx, userID); err != nil {
		t.Fatalf("EnsureSeeded: %v", err)
	}
	first := countRows(t, ctx, s, userID)
	if first == 0 {
		t.Fatal("seeded zero rows")
	}
	if err := s.EnsureSeeded(ctx, userID); err != nil {
		t.Fatalf("EnsureSeeded again: %v", err)
	}
	if got := countRows(t, ctx, s, userID); got != first {
		t.Fatalf("second seed changed rows: %d -> %d", first, got)
	}

	groups, err := s.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(groups) == 0 || len(groups[0].Children) == 0 {
		t.Fatalf("List missing tree shape: %+v", groups)
	}
}

func TestCreateDuplicateRejected(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)

	g, ok, err := s.CreateGroup(ctx, userID, KindExpense, "Ogród")
	if err != nil || !ok {
		t.Fatalf("CreateGroup = %+v, %v, %v", g, ok, err)
	}
	if _, ok, err := s.CreateGroup(ctx, userID, KindExpense, "  ogród "); err != nil || ok {
		t.Fatalf("case-insensitive duplicate group should miss: ok %v, err %v", ok, err)
	}
	// Same name, other kind is a different bucket.
	if _, ok, err := s.CreateGroup(ctx, userID, KindIncome, "Ogród"); err != nil || !ok {
		t.Fatalf("same name other kind should insert: ok %v, err %v", ok, err)
	}

	if _, ok, err := s.CreateSubcategory(ctx, userID, g.ID, "Nasiona"); err != nil || !ok {
		t.Fatalf("CreateSubcategory = %v, %v", ok, err)
	}
	if _, ok, err := s.CreateSubcategory(ctx, userID, g.ID, "NASIONA"); err != nil || ok {
		t.Fatalf("duplicate subcategory should miss: ok %v, err %v", ok, err)
	}
	if _, ok, err := s.CreateSubcategory(ctx, userID, "00000000-0000-0000-0000-000000000000", "X"); err != nil || ok {
		t.Fatalf("unknown parent should miss: ok %v, err %v", ok, err)
	}
	if _, _, err := s.CreateGroup(ctx, userID, KindExpense, ""); err == nil {
		t.Fatal("empty name should error")
	}
	if _, _, err := s.CreateGroup(ctx, userID, "bogus", "X"); err == nil {
		t.Fatal("bad kind should error")
	}
}

func TestIsolationBetweenUsers(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)

	g, ok, err := s.CreateGroup(ctx, a, KindExpense, "TylkoA")
	if err != nil || !ok {
		t.Fatalf("CreateGroup: %v %v", ok, err)
	}
	if groups, err := s.List(ctx, b); err != nil || len(groups) != 0 {
		t.Fatalf("user B should see nothing: %+v, %v", groups, err)
	}
	// B cannot parent under A's group.
	if _, ok, err := s.CreateSubcategory(ctx, b, g.ID, "Sneaky"); err != nil || ok {
		t.Fatalf("cross-user parent should miss: ok %v, err %v", ok, err)
	}
	if err := s.EnsureSeeded(ctx, b); err != nil {
		t.Fatalf("EnsureSeeded B: %v", err)
	}
	if groups, err := s.List(ctx, a); err != nil || len(groups) != 1 {
		t.Fatalf("A must keep only own group: %+v, %v", groups, err)
	}
}
