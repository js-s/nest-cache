package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound signals that the addressed transaction does not exist for the
// caller (missing, or owned by another account). Callers map it to 404.
var ErrNotFound = errors.New("transactions: not found")

// Kinds of transactions.
const (
	KindExpense = "expense"
	KindIncome  = "income"
)

// MaxListLimit caps the page size even for direct store callers;
// the handler clamps to the same value via parseBounded.
const MaxListLimit = 100

// MaxDescriptionLen caps descriptions; mirrored by the DB CHECK constraint.
const MaxDescriptionLen = 500

// MaxAmount is the largest value NUMERIC(12,2) holds; the accepted format caps digits accordingly.
const MaxAmount = "9999999999.99"

// amountRe accepts at most 10 integer digits and 2 decimal digits, matching NUMERIC(12,2).
var amountRe = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// uuidRe matches the canonical UUID shape Postgres stores, so bad category IDs
// are rejected as input (ok=false) instead of surfacing as a DB error.
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Transaction is one operation with its display category names resolved.
type Transaction struct {
	ID           string
	Kind         string
	CategoryID   string
	CategoryName string
	ParentName   string
	Amount       string
	OccurredOn   time.Time
	Description  string
	CreatedAt    time.Time
}

// Store persists per-user transactions over database/sql (pgx driver).
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over db. It keeps the pool the caller configured.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// positiveAmount reports whether amount is a valid, strictly positive money string.
// Digit-only comparison avoids float parsing entirely.
func positiveAmount(amount string) bool {
	if !amountRe.MatchString(amount) {
		return false
	}
	for _, r := range amount {
		if r != '0' && r != '.' {
			return true
		}
	}
	return false
}

// truncateRunes caps s at max runes; descriptions are stored, not rejected.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// Create records one transaction owned by userID. kind is expense|income
// (empty trims to expense). ok=false means the input was rejected (bad
// kind/amount/date, or a category that is missing, foreign, or of the other
// kind); an error is returned only for a real database failure.
func (s *Store) Create(ctx context.Context, userID, categoryID, kind, amount, occurredOn, description string) (Transaction, bool, error) {
	amount = strings.TrimSpace(amount)
	categoryID = strings.TrimSpace(categoryID)
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = KindExpense
	}
	occurred, err := time.Parse("2006-01-02", occurredOn)
	if userID == "" || categoryID == "" || !uuidRe.MatchString(categoryID) || !positiveAmount(amount) || err != nil {
		return Transaction{}, false, nil
	}
	if kind != KindExpense && kind != KindIncome {
		return Transaction{}, false, nil
	}
	description = truncateRunes(strings.TrimSpace(description), MaxDescriptionLen)

	// ponytail: one INSERT ... SELECT gates ownership + kind atomically; no SELECT-then-INSERT race.
	var t Transaction
	var parent sql.NullString
	err = s.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO transactions (user_id, category_id, kind, amount, occurred_on, description)
		     SELECT $1, c.id, $6, $2::numeric, $3::date, $4
		     FROM categories c
		     WHERE c.id = $5 AND c.user_id = $1 AND c.kind = $6
		     RETURNING id, kind, category_id, amount, occurred_on, description, created_at
		 )
		 SELECT ins.id, ins.kind, ins.category_id, c.name, p.name, ins.amount::text, ins.occurred_on, ins.description, ins.created_at
		 FROM ins
		 JOIN categories c ON c.id = ins.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id`,
		userID, amount, occurred, description, categoryID, kind,
	).Scan(&t.ID, &t.Kind, &t.CategoryID, &t.CategoryName, &parent, &t.Amount, &t.OccurredOn, &t.Description, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return Transaction{}, false, nil
	}
	if err != nil {
		return Transaction{}, false, fmt.Errorf("transactions: create: %w", err)
	}
	t.ParentName = parent.String
	return t, true, nil
}

// Update replaces the editable fields of one owned transaction. The row's kind
// is immutable; the new category must belong to the user and match that kind.
// ok=false means rejected input (bad id/amount/date/category); ErrNotFound
// means no such owned row; any other error is a real database failure.
func (s *Store) Update(ctx context.Context, userID, txID, categoryID, amount, occurredOn, description string) (Transaction, bool, error) {
	amount = strings.TrimSpace(amount)
	categoryID = strings.TrimSpace(categoryID)
	txID = strings.TrimSpace(txID)
	occurred, err := time.Parse("2006-01-02", strings.TrimSpace(occurredOn))
	if userID == "" || !uuidRe.MatchString(txID) || !uuidRe.MatchString(categoryID) || !positiveAmount(amount) || err != nil {
		return Transaction{}, false, nil
	}
	description = truncateRunes(strings.TrimSpace(description), MaxDescriptionLen)

	var t Transaction
	var parent sql.NullString
	err = s.db.QueryRowContext(ctx,
		`WITH upd AS (
		     UPDATE transactions t
		     SET category_id = c.id, amount = $3::numeric, occurred_on = $4::date, description = $5
		     FROM categories c
		     WHERE t.id = $1 AND t.user_id = $2
		       AND c.id = $6 AND c.user_id = $2 AND c.kind = t.kind
		     RETURNING t.id, t.kind, t.category_id, t.amount, t.occurred_on, t.description, t.created_at
		 )
		 SELECT upd.id, upd.kind, upd.category_id, c.name, p.name, upd.amount::text, upd.occurred_on, upd.description, upd.created_at
		 FROM upd
		 JOIN categories c ON c.id = upd.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id`,
		txID, userID, amount, occurred, description, categoryID,
	).Scan(&t.ID, &t.Kind, &t.CategoryID, &t.CategoryName, &parent, &t.Amount, &t.OccurredOn, &t.Description, &t.CreatedAt)
	if err == sql.ErrNoRows {
		// ponytail: cheap probe separates 404 (no owned row) from 400 (bad category); not transactional, fine for single-user.
		var exists int
		switch probeErr := s.db.QueryRowContext(ctx,
			`SELECT 1 FROM transactions WHERE id = $1 AND user_id = $2`, txID, userID,
		).Scan(&exists); probeErr {
		case nil:
			return Transaction{}, false, nil
		case sql.ErrNoRows:
			return Transaction{}, false, ErrNotFound
		default:
			return Transaction{}, false, fmt.Errorf("transactions: update probe: %w", probeErr)
		}
	}
	if err != nil {
		return Transaction{}, false, fmt.Errorf("transactions: update: %w", err)
	}
	t.ParentName = parent.String
	return t, true, nil
}

// Delete removes one owned transaction. ok=false means malformed input;
// ErrNotFound means no such owned row; nil error with true means deleted.
func (s *Store) Delete(ctx context.Context, userID, txID string) (bool, error) {
	txID = strings.TrimSpace(txID)
	if userID == "" || !uuidRe.MatchString(txID) {
		return false, nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM transactions WHERE id = $1 AND user_id = $2`, txID, userID)
	if err != nil {
		return false, fmt.Errorf("transactions: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("transactions: delete rows: %w", err)
	}
	if n == 0 {
		return false, ErrNotFound
	}
	return true, nil
}

// List returns one page of the user's operations, newest first, with the total
// row count so callers can drive "load more". kind is all|expense|income
// (empty trims to all); anything else is an invalid-input error.
func (s *Store) List(ctx context.Context, userID string, page, limit int, kind string) ([]Transaction, int, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("transactions: list: invalid input")
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != KindExpense && kind != KindIncome {
		return nil, 0, fmt.Errorf("transactions: list: invalid input")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}

	kindFilter := ""
	countArgs := []any{userID}
	listArgs := []any{userID}
	if kind != "all" {
		kindFilter = ` AND t.kind = $2`
		countArgs = append(countArgs, kind)
		listArgs = append(listArgs, kind)
	}

	// ponytail: COUNT(*) per request is fine at personal scale; denormalize if it ever matters.
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM transactions t WHERE t.user_id = $1`+kindFilter, countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("transactions: list count: %w", err)
	}

	limitPos := len(listArgs) + 1
	offsetPos := len(listArgs) + 2
	listArgs = append(listArgs, limit, (page-1)*limit)
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.kind, t.category_id, c.name, p.name, t.amount::text, t.occurred_on, t.description, t.created_at
		 FROM transactions t
		 JOIN categories c ON c.id = t.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id
		 WHERE t.user_id = $1`+kindFilter+`
		 ORDER BY t.occurred_on DESC, t.created_at DESC
		 LIMIT $`+strconv.Itoa(limitPos)+` OFFSET $`+strconv.Itoa(offsetPos),
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("transactions: list: %w", err)
	}
	defer rows.Close()

	items := []Transaction{}
	for rows.Next() {
		var t Transaction
		var parent sql.NullString
		if err := rows.Scan(&t.ID, &t.Kind, &t.CategoryID, &t.CategoryName, &parent, &t.Amount, &t.OccurredOn, &t.Description, &t.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("transactions: list scan: %w", err)
		}
		t.ParentName = parent.String
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("transactions: list rows: %w", err)
	}
	return items, total, nil
}
