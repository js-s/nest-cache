package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// SummaryRow is one aggregated total per category. Totals stay strings:
// Postgres NUMERIC travels as text so Go never touches float money.
type SummaryRow struct {
	CategoryID   string
	CategoryName string
	ParentName   string
	Total        string
}

// BudgetRatio carries whole-period totals plus the expense/income percentage.
// RatioPct is nil when income is zero (frontend renders a dash, not 0%).
type BudgetRatio struct {
	ExpenseTotal string
	IncomeTotal  string
	RatioPct     *string
}

// ratioPct computes expenses*100/incomes with one decimal, string-only.
// nil means zero income. Half-away rounding from big.Rat is fine for badges.
func ratioPct(expenses, incomes string) *string {
	inc, ok := new(big.Rat).SetString(strings.TrimSpace(incomes))
	if !ok || inc.Sign() == 0 {
		return nil
	}
	exp, ok := new(big.Rat).SetString(strings.TrimSpace(expenses))
	if !ok {
		return nil
	}
	pct := new(big.Rat).Mul(exp, big.NewRat(100, 1))
	pct.Quo(pct, inc)
	out := pct.FloatString(1)
	return &out
}

// maxSummaryItems caps the transaction list under the aggregates.
// Full history stays on /operations; summary shows newest-first context.
const maxSummaryItems = 50

// Summary aggregates the user's expenses in [from, to] per category and
// returns the newest transactions under the same filter. Empty categoryIDs
// means all expense categories. ok=false means rejected input (bad dates,
// from>to, bad/foreign/non-expense category); only real DB faults are errors.
// The returned budget holds whole-period expense/income totals plus ratio,
// ignoring the category filter.
func (s *Store) Summary(ctx context.Context, userID, from, to string, categoryIDs []string) ([]SummaryRow, string, []Transaction, BudgetRatio, bool, error) {
	if userID == "" {
		return nil, "", nil, BudgetRatio{}, false, nil
	}
	fromDate, err := time.Parse("2006-01-02", strings.TrimSpace(from))
	if err != nil {
		return nil, "", nil, BudgetRatio{}, false, nil
	}
	toDate, err := time.Parse("2006-01-02", strings.TrimSpace(to))
	if err != nil {
		return nil, "", nil, BudgetRatio{}, false, nil
	}
	if fromDate.After(toDate) {
		return nil, "", nil, BudgetRatio{}, false, nil
	}

	ids := make([]string, 0, len(categoryIDs))
	seen := make(map[string]struct{}, len(categoryIDs))
	for _, raw := range categoryIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !uuidRe.MatchString(id) {
			return nil, "", nil, BudgetRatio{}, false, nil
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	// Gate: every requested category must exist, belong to caller, be expense.
	// Unknown/foreign/income IDs are invalid input (400), not silent zeros.
	// ponytail: IN with expanded placeholders, no array driver dependency.
	catArgs := make([]any, 0, len(ids)+1)
	catArgs = append(catArgs, userID)
	inList := ""
	if len(ids) > 0 {
		holders := make([]string, 0, len(ids))
		for i, id := range ids {
			holders = append(holders, "$"+strconv.Itoa(i+2))
			catArgs = append(catArgs, id)
		}
		inList = " AND id IN (" + strings.Join(holders, ",") + ")"
		var matched int
		if err := s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM categories WHERE user_id = $1 AND kind = 'expense'`+inList,
			catArgs...,
		).Scan(&matched); err != nil {
			return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary gate: %w", err)
		}
		if matched != len(ids) {
			return nil, "", nil, BudgetRatio{}, false, nil
		}
	}

	filter := `t.user_id = $1 AND t.kind = 'expense' AND t.occurred_on BETWEEN $2 AND $3`
	args := []any{userID, fromDate, toDate}
	if len(ids) > 0 {
		holders := make([]string, 0, len(ids))
		for i, id := range ids {
			holders = append(holders, "$"+strconv.Itoa(i+4))
			args = append(args, id)
		}
		filter += ` AND t.category_id IN (` + strings.Join(holders, ",") + `)`
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.name, p.name, SUM(t.amount)::text
		 FROM transactions t
		 JOIN categories c ON c.id = t.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id
		 WHERE `+filter+`
		 GROUP BY c.id, c.name, p.name
		 ORDER BY SUM(t.amount) DESC, c.name`,
		args...,
	)
	if err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary rows: %w", err)
	}
	defer rows.Close()

	out := []SummaryRow{}
	for rows.Next() {
		var r SummaryRow
		var parent sql.NullString
		if err := rows.Scan(&r.CategoryID, &r.CategoryName, &parent, &r.Total); err != nil {
			return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary scan: %w", err)
		}
		r.ParentName = parent.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary rows: %w", err)
	}

	var total string
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(t.amount)::text, '0') FROM transactions t WHERE `+filter,
		args...,
	).Scan(&total); err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary total: %w", err)
	}

	itemRows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.kind, t.category_id, c.name, p.name, t.amount::text, t.occurred_on, t.description, t.created_at
		 FROM transactions t
		 JOIN categories c ON c.id = t.category_id
		 LEFT JOIN categories p ON p.id = c.parent_id
		 WHERE `+filter+`
		 ORDER BY t.occurred_on DESC, t.created_at DESC
		 LIMIT `+fmt.Sprint(maxSummaryItems),
		args...,
	)
	if err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary items: %w", err)
	}
	defer itemRows.Close()

	items := []Transaction{}
	for itemRows.Next() {
		var t Transaction
		var parent sql.NullString
		if err := itemRows.Scan(&t.ID, &t.Kind, &t.CategoryID, &t.CategoryName, &parent, &t.Amount, &t.OccurredOn, &t.Description, &t.CreatedAt); err != nil {
			return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary items scan: %w", err)
		}
		t.ParentName = parent.String
		items = append(items, t)
	}
	if err := itemRows.Err(); err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary items rows: %w", err)
	}

	// Whole-period budget: same dates, no category filter. Two indexed SUMs.
	var budgetExpense, budgetIncome string
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(t.amount)::text, '0') FROM transactions t WHERE t.user_id = $1 AND t.kind = 'expense' AND t.occurred_on BETWEEN $2 AND $3`,
		userID, fromDate, toDate,
	).Scan(&budgetExpense); err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary budget expense: %w", err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(t.amount)::text, '0') FROM transactions t WHERE t.user_id = $1 AND t.kind = 'income' AND t.occurred_on BETWEEN $2 AND $3`,
		userID, fromDate, toDate,
	).Scan(&budgetIncome); err != nil {
		return nil, "", nil, BudgetRatio{}, false, fmt.Errorf("transactions: summary budget income: %w", err)
	}
	budget := BudgetRatio{ExpenseTotal: budgetExpense, IncomeTotal: budgetIncome, RatioPct: ratioPct(budgetExpense, budgetIncome)}
	return out, total, items, budget, true, nil
}
