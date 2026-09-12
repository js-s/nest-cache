package categories

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

//go:embed seed_categories.yaml
var seedMirrored []byte

// seedFile is the source of truth for starter categories.
//
// NOTE: this package embeds a mirror (seed_categories.yaml), not
// resources/categories.yaml directly — Go embed cannot reach outside the
// module subtree of this package (../../resources is outside
// internal/categories). Keep the mirror in sync with
// resources/categories.yaml; seed_parse_test.go fails if they drift.
const seedFile = "internal/categories/seed_categories.yaml"

// seedDoc mirrors resources/categories.yaml: grouped expenses,
// flat incomes.
type seedDoc struct {
	Wydatki   map[string][]string `yaml:"wydatki"`
	Przychody []string            `yaml:"przychody"`
}

// seedGroup is one starter group with its children.
type seedGroup struct {
	kind     string
	name     string
	children []string
}

// displayNames overrides humanize() where Polish diacritics matter.
var displayNames = map[string]string{
	"styl_zycia": "Styl życia",
	"pozostale":  "Pozostałe",
}

// humanize turns a yaml slug into a display name: codzienne → Codzienne.
func humanize(slug string) string {
	if name, ok := displayNames[slug]; ok {
		return name
	}
	s := strings.ReplaceAll(slug, "_", " ")
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// parseSeed decodes starter categories. Flat income entries become
// childless groups (simplest shape that still filters by kind downstream).
func parseSeed(raw []byte) ([]seedGroup, error) {
	var doc seedDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("categories: parse seed: %w", err)
	}
	out := make([]seedGroup, 0, len(doc.Wydatki)+len(doc.Przychody))
	for slug, kids := range doc.Wydatki {
		out = append(out, seedGroup{kind: KindExpense, name: humanize(slug), children: kids})
	}
	for _, name := range doc.Przychody {
		out = append(out, seedGroup{kind: KindIncome, name: name})
	}
	return out, nil
}

// EnsureSeeded copies starter categories to a fresh account. It is
// idempotent: non-empty accounts are left alone, and concurrent first
// calls collide harmlessly on ON CONFLICT DO NOTHING.
func (s *Store) EnsureSeeded(ctx context.Context, userID string) error {
	n, err := s.count(ctx, userID)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	groups, err := parseSeed(seedMirrored)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("categories: seed begin: %w", err)
	}
	// ponytail: single tx, no prepared stmt; <60 rows once per account.
	for _, g := range groups {
		var gid string
		err := tx.QueryRowContext(ctx,
			`INSERT INTO categories (user_id, kind, name) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING RETURNING id`,
			userID, g.kind, g.name,
		).Scan(&gid)
		if err == sql.ErrNoRows {
			// Lost a concurrent seed race on this group; reload its id.
			if err := tx.QueryRowContext(ctx,
				`SELECT id FROM categories WHERE user_id = $1 AND kind = $2 AND parent_id IS NULL AND lower(name) = lower($3)`,
				userID, g.kind, g.name,
			).Scan(&gid); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("categories: seed reread group: %w", err)
			}
		} else if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("categories: seed group: %w", err)
		}
		for _, kid := range g.children {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO categories (user_id, parent_id, kind, name) VALUES ($1, $2, $3, $4)
				 ON CONFLICT DO NOTHING`,
				userID, gid, g.kind, kid,
			); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("categories: seed child: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("categories: seed commit: %w", err)
	}
	return nil
}
