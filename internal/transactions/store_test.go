package transactions

import (
	"context"
	"database/sql"
	"errors"
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
		t.Skip("DATABASE_URL unset; transactions store test needs real Postgres")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	// Registered before per-test row cleanups: LIFO runs those first, then this close.
	t.Cleanup(func() { _ = db.Close() })
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
	email := "TxCase+" + time.Now().Format("150405.000000000") + "@example.com"
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, "hash-1",
	).Scan(&id); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM transactions WHERE user_id = $1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func createCategory(t *testing.T, ctx context.Context, db *sql.DB, userID, kind, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, kind, name) VALUES ($1, $2, $3) RETURNING id`,
		userID, kind, name,
	).Scan(&id); err != nil {
		t.Fatalf("create test category: %v", err)
	}
	return id
}

func createSubcategory(t *testing.T, ctx context.Context, db *sql.DB, userID, parentID, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, parent_id, kind, name) SELECT $1, id, kind, $3 FROM categories WHERE id = $2 RETURNING id`,
		userID, parentID, name,
	).Scan(&id); err != nil {
		t.Fatalf("create test subcategory: %v", err)
	}
	return id
}

func TestCreateExpense(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)
	parent := createCategory(t, ctx, db, userID, KindExpense, "Jedzenie")
	sub := createSubcategory(t, ctx, db, userID, parent, "Zakupy")

	tx, ok, err := s.Create(ctx, userID, sub, KindExpense, "12.34", "2026-02-03", "  obiad  ")
	if err != nil || !ok {
		t.Fatalf("Create = %+v, %v, %v", tx, ok, err)
	}
	if tx.Amount != "12.34" {
		t.Fatalf("amount = %q, want 12.34 (no float rounding)", tx.Amount)
	}
	if tx.Kind != KindExpense || tx.CategoryID != sub || tx.CategoryName != "Zakupy" || tx.ParentName != "Jedzenie" {
		t.Fatalf("unexpected fields: %+v", tx)
	}
	if tx.Description != "obiad" {
		t.Fatalf("description = %q, want trimmed", tx.Description)
	}
	if got := tx.OccurredOn.Format("2006-01-02"); got != "2026-02-03" {
		t.Fatalf("occurred_on = %q", got)
	}
	if tx.ID == "" || tx.CreatedAt.IsZero() {
		t.Fatalf("missing generated fields: %+v", tx)
	}
}

func TestCreatePreservesMoneyPrecision(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, userID, KindExpense, "Precyzja")

	for _, amount := range []string{"0.01", "0.10", "1000000.00", MaxAmount} {
		tx, ok, err := s.Create(ctx, userID, cat, KindExpense, amount, "2026-02-03", "")
		if err != nil || !ok {
			t.Fatalf("Create(%s) = %v, %v", amount, ok, err)
		}
		if tx.Amount != amount {
			t.Fatalf("amount round-trip = %q, want %q", tx.Amount, amount)
		}
	}

	// A group-level category has no parent name.
	tx, ok, err := s.Create(ctx, userID, cat, KindExpense, "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("Create group-level = %v, %v", ok, err)
	}
	if tx.ParentName != "" {
		t.Fatalf("group-level parent_name = %q, want empty", tx.ParentName)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")

	cases := []struct {
		name     string
		userID   string
		category string
		amount   string
		date     string
	}{
		{"zero", a, expense, "0.00", "2026-02-03"},
		{"zero no decimals", a, expense, "0", "2026-02-03"},
		{"negative", a, expense, "-5.00", "2026-02-03"},
		{"too many decimals", a, expense, "1.234", "2026-02-03"},
		{"not a number", a, expense, "abc", "2026-02-03"},
		{"too large", a, expense, "12345678901.00", "2026-02-03"},
		{"empty amount", a, expense, "", "2026-02-03"},
		{"foreign category", a, foreign, "5.00", "2026-02-03"},
		{"income category", a, income, "5.00", "2026-02-03"},
		{"unknown category", a, "00000000-0000-0000-0000-000000000000", "5.00", "2026-02-03"},
		{"empty category", a, "", "5.00", "2026-02-03"},
		{"malformed category", a, "not-a-uuid", "5.00", "2026-02-03"},
		{"bad date", a, expense, "5.00", "03-02-2026"},
		{"bad date text", a, expense, "5.00", "yesterday"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tx, ok, err := s.Create(ctx, tc.userID, tc.category, KindExpense, tc.amount, tc.date, "x"); err != nil || ok {
				t.Fatalf("expected ok=false, nil error; got %+v, %v, %v", tx, ok, err)
			}
		})
	}
}

func TestListPaginationAndIsolation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	aCat := createCategory(t, ctx, db, a, KindExpense, "A")
	bCat := createCategory(t, ctx, db, b, KindExpense, "B")

	for _, d := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
		if _, ok, err := s.Create(ctx, a, aCat, KindExpense, "1.00", d, ""); err != nil || !ok {
			t.Fatalf("seed A %s: %v %v", d, ok, err)
		}
	}
	if _, ok, err := s.Create(ctx, b, bCat, KindExpense, "9.99", "2026-01-05", ""); err != nil || !ok {
		t.Fatalf("seed B: %v %v", ok, err)
	}

	page1, total, err := s.List(ctx, a, 1, 2, "all")
	if err != nil {
		t.Fatalf("List A page1: %v", err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("A page1 total=%d len=%d, want 3/2", total, len(page1))
	}
	if got := page1[0].OccurredOn.Format("2006-01-02"); got != "2026-01-03" {
		t.Fatalf("first item = %s, want newest 2026-01-03", got)
	}

	page2, total, err := s.List(ctx, a, 2, 2, "all")
	if err != nil {
		t.Fatalf("List A page2: %v", err)
	}
	if total != 3 || len(page2) != 1 {
		t.Fatalf("A page2 total=%d len=%d, want 3/1", total, len(page2))
	}
	if page2[0].ID == page1[0].ID || page2[0].ID == page1[1].ID {
		t.Fatal("pages overlap")
	}

	// B sees only its own row.
	bItems, bTotal, err := s.List(ctx, b, 1, 20, "all")
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	if bTotal != 1 || len(bItems) != 1 || bItems[0].CategoryName != "B" {
		t.Fatalf("isolation broken: total=%d items=%+v", bTotal, bItems)
	}
}

func TestCreateIncomeAndKindGate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreignIncome := createCategory(t, ctx, db, b, KindIncome, "Obce wpływy")

	// Income writes with an income category.
	tx, ok, err := s.Create(ctx, a, income, KindIncome, "100.00", "2026-02-03", "pensja")
	if err != nil || !ok {
		t.Fatalf("Create income = %v %v", ok, err)
	}
	if tx.Kind != KindIncome || tx.Amount != "100.00" || tx.CategoryName != "Wpływy" {
		t.Fatalf("unexpected income fields: %+v", tx)
	}

	// Empty kind defaults to expense.
	tx, ok, err = s.Create(ctx, a, expense, "", "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("Create default kind = %v %v", ok, err)
	}
	if tx.Kind != KindExpense {
		t.Fatalf("default kind = %q, want expense", tx.Kind)
	}

	// Mismatched and foreign kinds are rejected as ok=false.
	for _, tc := range []struct {
		name     string
		category string
		kind     string
	}{
		{"expense into income category", income, KindExpense},
		{"income into expense category", expense, KindIncome},
		{"income into foreign category", foreignIncome, KindIncome},
		{"bad kind", expense, "grant"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok, err := s.Create(ctx, a, tc.category, tc.kind, "5.00", "2026-02-03", ""); err != nil || ok {
				t.Fatalf("expected ok=false nil error, got ok=%v err=%v", ok, err)
			}
		})
	}
}

func TestListKindFilter(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	exp := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	inc := createCategory(t, ctx, db, a, KindIncome, "Wpływy")

	if _, ok, err := s.Create(ctx, a, exp, KindExpense, "10.00", "2026-03-01", ""); err != nil || !ok {
		t.Fatalf("seed expense: %v %v", ok, err)
	}
	if _, ok, err := s.Create(ctx, a, inc, KindIncome, "100.00", "2026-03-02", ""); err != nil || !ok {
		t.Fatalf("seed income: %v %v", ok, err)
	}

	all, total, err := s.List(ctx, a, 1, 20, "all")
	if err != nil || total != 2 || len(all) != 2 {
		t.Fatalf("all = %d/%d, want 2/2 (err=%v)", total, len(all), err)
	}
	expOnly, total, err := s.List(ctx, a, 1, 20, KindExpense)
	if err != nil || total != 1 || len(expOnly) != 1 || expOnly[0].Kind != KindExpense {
		t.Fatalf("expense filter broken: total=%d items=%+v err=%v", total, expOnly, err)
	}
	incOnly, total, err := s.List(ctx, a, 1, 20, KindIncome)
	if err != nil || total != 1 || len(incOnly) != 1 || incOnly[0].Kind != KindIncome {
		t.Fatalf("income filter broken: total=%d items=%+v err=%v", total, incOnly, err)
	}
	if _, _, err := s.List(ctx, a, 1, 20, "grant"); err == nil {
		t.Fatal("bad kind should error")
	}
}

func TestListEmpty(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)

	items, total, err := s.List(ctx, userID, 1, 20, "all")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("empty list = %d/%d, want 0/0", total, len(items))
	}
}

func TestTransactionsUpdate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	userID := createTestUser(t, ctx, db)
	parent := createCategory(t, ctx, db, userID, KindExpense, "Jedzenie")
	sub := createSubcategory(t, ctx, db, userID, parent, "Zakupy")
	initial := createCategory(t, ctx, db, userID, KindExpense, "Transport")

	created, ok, err := s.Create(ctx, userID, initial, KindExpense, "12.34", "2026-02-03", "obiad")
	if err != nil || !ok {
		t.Fatalf("seed create: %v %v", ok, err)
	}

	updated, ok, err := s.Update(ctx, userID, created.ID, sub, "55.00", "2026-02-10", "  przejazd  ")
	if err != nil || !ok {
		t.Fatalf("Update = %+v, %v, %v", updated, ok, err)
	}
	if updated.ID != created.ID || updated.Kind != KindExpense {
		t.Fatalf("identity changed: %+v", updated)
	}
	if updated.Amount != "55.00" || updated.CategoryID != sub || updated.CategoryName != "Zakupy" || updated.ParentName != "Jedzenie" {
		t.Fatalf("unexpected update: %+v", updated)
	}
	if updated.Description != "przejazd" {
		t.Fatalf("description = %q, want trimmed", updated.Description)
	}
	if got := updated.OccurredOn.Format("2006-01-02"); got != "2026-02-10" {
		t.Fatalf("occurred_on = %q", got)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("created_at changed: %v -> %v", created.CreatedAt, updated.CreatedAt)
	}

	items, total, err := s.List(ctx, userID, 1, 20, "all")
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("List after update: total=%d items=%d err=%v", total, len(items), err)
	}
	if items[0].Amount != "55.00" || items[0].CategoryName != "Zakupy" || items[0].Description != "przejazd" {
		t.Fatalf("list not updated: %+v", items[0])
	}
}

func TestTransactionsUpdateRejectsInvalidInput(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")

	seed, ok, err := s.Create(ctx, a, expense, KindExpense, "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}

	for _, tc := range []struct {
		name, txID, cat, amount, date string
	}{
		{"malformed txID", "not-a-uuid", expense, "5.00", "2026-02-03"},
		{"malformed category", seed.ID, "not-a-uuid", "5.00", "2026-02-03"},
		{"empty category", seed.ID, "", "5.00", "2026-02-03"},
		{"zero amount", seed.ID, expense, "0.00", "2026-02-03"},
		{"not a number", seed.ID, expense, "abc", "2026-02-03"},
		{"bad date", seed.ID, expense, "5.00", "03-02-2026"},
		{"foreign category", seed.ID, foreign, "5.00", "2026-02-03"},
		{"other kind category", seed.ID, income, "5.00", "2026-02-03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tx, ok, err := s.Update(ctx, a, tc.txID, tc.cat, tc.amount, tc.date, "x"); err != nil || ok {
				t.Fatalf("expected ok=false nil error; got %+v, %v, %v", tx, ok, err)
			}
		})
	}

	// A well-formed but unknown id, and a foreign row, are not found.
	unknown := "00000000-0000-0000-0000-000000000000"
	if _, ok, err := s.Update(ctx, a, unknown, expense, "5.00", "2026-02-03", ""); !errors.Is(err, ErrNotFound) || ok {
		t.Fatalf("unknown id = ok=%v err=%v, want ErrNotFound", ok, err)
	}
	if _, ok, err := s.Update(ctx, b, seed.ID, foreign, "5.00", "2026-02-03", ""); !errors.Is(err, ErrNotFound) || ok {
		t.Fatalf("cross-account id = ok=%v err=%v, want ErrNotFound", ok, err)
	}

	// The rejected updates left the row intact.
	items, total, err := s.List(ctx, a, 1, 20, "all")
	if err != nil || total != 1 || items[0].Amount != "5.00" || items[0].CategoryID != expense {
		t.Fatalf("row changed after rejects: total=%d items=%+v err=%v", total, items, err)
	}
}

func TestTransactionsDelete(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, a, KindExpense, "Wydatki")

	seed, ok, err := s.Create(ctx, a, cat, KindExpense, "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	other, ok, err := s.Create(ctx, a, cat, KindExpense, "6.00", "2026-02-04", "")
	if err != nil || !ok {
		t.Fatalf("seed other: %v %v", ok, err)
	}

	if deleted, err := s.Delete(ctx, a, seed.ID); err != nil || !deleted {
		t.Fatalf("Delete = %v, %v", deleted, err)
	}
	if _, err := s.Delete(ctx, a, seed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeat delete err = %v, want ErrNotFound", err)
	}
	if deleted, err := s.Delete(ctx, a, "not-a-uuid"); err != nil || deleted {
		t.Fatalf("malformed delete = %v, %v, want false,nil", deleted, err)
	}
	if _, err := s.Delete(ctx, b, other.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account delete err = %v, want ErrNotFound", err)
	}

	items, total, err := s.List(ctx, a, 1, 20, "all")
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != other.ID {
		t.Fatalf("A rows after deletes: total=%d items=%+v err=%v", total, items, err)
	}
}
