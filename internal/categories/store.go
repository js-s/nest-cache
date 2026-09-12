package categories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Kinds of categories. A subcategory always inherits its group's kind.
const (
	KindExpense = "expense"
	KindIncome  = "income"
)

// MaxNameLen caps names; mirrored by the DB CHECK constraint.
const MaxNameLen = 80

// Group is a top-level category with its subcategories.
type Group struct {
	ID       string
	Kind     string
	Name     string
	Children []Subcategory
}

// Subcategory is a second-level category under one group.
type Subcategory struct {
	ID   string
	Name string
}

// Store persists per-user categories over database/sql (pgx driver).
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over db. It keeps the pool the caller configured.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// normalize trims; empty stays empty so callers reject it.
func normalize(name string) string {
	return strings.TrimSpace(name)
}

func validKind(kind string) bool {
	return kind == KindExpense || kind == KindIncome
}

// isConflict reports unique-violation (SQLSTATE 23505) from the pgx driver.
func isConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// List returns the user's groups with children, ordered by kind then name.
// Flat income seeds come back as groups without children.
func (s *Store) List(ctx context.Context, userID string) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, parent_id, kind, name FROM categories WHERE user_id = $1 ORDER BY kind, name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("categories: list: %w", err)
	}
	defer rows.Close()

	groups := []Group{}
	byID := map[string]*Group{}
	for rows.Next() {
		var id, kind, name string
		var parentID sql.NullString
		if err := rows.Scan(&id, &parentID, &kind, &name); err != nil {
			return nil, fmt.Errorf("categories: list scan: %w", err)
		}
		if !parentID.Valid {
			g := Group{ID: id, Kind: kind, Name: name}
			groups = append(groups, g)
			byID[id] = &groups[len(groups)-1]
			continue
		}
		if g, ok := byID[parentID.String]; ok {
			g.Children = append(g.Children, Subcategory{ID: id, Name: name})
		}
		// ponytail: orphan child (parent filtered out) dropped; FK cascade makes this unreachable.
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories: list rows: %w", err)
	}
	return groups, nil
}

// count returns the user's row count.
func (s *Store) count(ctx context.Context, userID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM categories WHERE user_id = $1`, userID,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("categories: count: %w", err)
	}
	return n, nil
}

// CreateGroup inserts a top-level category. Duplicates return ok=false.
func (s *Store) CreateGroup(ctx context.Context, userID, kind, name string) (Group, bool, error) {
	name = normalize(name)
	if userID == "" || !validKind(kind) || name == "" || len([]rune(name)) > MaxNameLen {
		return Group{}, false, fmt.Errorf("categories: create group: invalid input")
	}
	var g Group
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, kind, name) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING
		 RETURNING id, kind, name`,
		userID, kind, name,
	).Scan(&g.ID, &g.Kind, &g.Name)
	if err == sql.ErrNoRows {
		return Group{}, false, nil
	}
	if err != nil {
		return Group{}, false, fmt.Errorf("categories: create group: %w", err)
	}
	return g, true, nil
}

// CreateSubcategory inserts a child under a group owned by the same user.
// The child inherits the group's kind. Unknown/foreign parents and
// duplicates return ok=false.
func (s *Store) CreateSubcategory(ctx context.Context, userID, parentID, name string) (Subcategory, bool, error) {
	name = normalize(name)
	if userID == "" || parentID == "" || name == "" || len([]rune(name)) > MaxNameLen {
		return Subcategory{}, false, fmt.Errorf("categories: create subcategory: invalid input")
	}
	var kind string
	err := s.db.QueryRowContext(ctx,
		`SELECT kind FROM categories WHERE id = $1 AND user_id = $2 AND parent_id IS NULL`,
		parentID, userID,
	).Scan(&kind)
	if err == sql.ErrNoRows {
		return Subcategory{}, false, nil
	}
	if err != nil {
		return Subcategory{}, false, fmt.Errorf("categories: create subcategory lookup parent: %w", err)
	}
	var sub Subcategory
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, parent_id, kind, name) VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING
		 RETURNING id, name`,
		userID, parentID, kind, name,
	).Scan(&sub.ID, &sub.Name)
	if err == sql.ErrNoRows {
		return Subcategory{}, false, nil
	}
	if err != nil {
		return Subcategory{}, false, fmt.Errorf("categories: create subcategory: %w", err)
	}
	return sub, true, nil
}
