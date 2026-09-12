package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Kinds of transactions. The store only writes expenses for now; income lands in S-06.
const (
	KindExpense = "expense"
	KindIncome  = "income"
)

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

// Create records one expense owned by userID. ok=false means the input was
// rejected (bad amount/date, or a category that is missing, foreign, or not an
// expense); an error is returned only for a real database failure.
func (s *Store) Create(ctx context.Context, userID, categoryID, amount, occurredOn, description string) (Transaction, bool, error) {
	amount = strings.TrimSpace(amount)
	categoryID = strings.TrimSpace(categoryID)
	occurred, err := time.Parse("2006-01-02", occurredOn)
	if userID == "" || categoryID == "" || !uuidRe.MatchString(categoryID) || !positiveAmount(amount) || err != nil {
		return Transaction{}, false, nil
	}
	description = truncateRunes(strings.TrimSpace(description), MaxDescriptionLen)

	// ponytail: one INSERT ... SELECT gates ownership + kind atomically; no SELECT-then-INSERT race.
	var t Transaction
	var parent sql.NullString
	err = s.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO transactions (user_id, category_id, kind, amount, occurred_on, description)
		     SELECT $1, c.id, 'expense', $2::numeric, $3::date, $4
		     FROM categories c
		     WHERE c.id = $5 AND c.user_id = $1 AND c.kind = 'expense'
		     RETURNING id, kind, category_id, amount, occurred_on, description, created_at
		 )
		 SELECT ins.id, ins.kind, ins.category_id, c.name, p.name, ins.amount::text, ins.occurred_on, ins.description, ins.created_at
		 FROM ins
		 JOIN categories c ON c.id = ins.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id`,
		userID, amount, occurred, description, categoryID,
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

// List returns one page of the user's operations, newest first, with the total
// row count so callers can drive "load more".
func (s *Store) List(ctx context.Context, userID string, page, limit int) ([]Transaction, int, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("transactions: list: invalid input")
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	// ponytail: COUNT(*) per request is fine at personal scale; denormalize if it ever matters.
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM transactions WHERE user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("transactions: list count: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.kind, t.category_id, c.name, p.name, t.amount::text, t.occurred_on, t.description, t.created_at
		 FROM transactions t
		 LEFT JOIN categories c ON c.id = t.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id
		 WHERE t.user_id = $1
		 ORDER BY t.occurred_on DESC, t.created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, (page-1)*limit,
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
