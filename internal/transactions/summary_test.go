package transactions

import (
	"context"
	"testing"
)

func TestSummaryAggregatesFiltersAndIsolation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	aFood := createCategory(t, ctx, db, a, KindExpense, "Jedzenie")
	aSub := createSubcategory(t, ctx, db, a, aFood, "Zakupy")
	aTrans := createCategory(t, ctx, db, a, KindExpense, "Transport")
	bCat := createCategory(t, ctx, db, b, KindExpense, "Obce")

	seed := []struct {
		cat, amount, date string
	}{
		{aSub, "10.00", "2026-09-05"},
		{aSub, "2.50", "2026-09-20"},
		{aTrans, "5.00", "2026-09-10"},
		{aTrans, "7.00", "2026-08-10"},
	}
	for _, srow := range seed {
		if _, ok, err := s.Create(ctx, a, srow.cat, KindExpense, srow.amount, srow.date, ""); err != nil || !ok {
			t.Fatalf("seed A %v: %v %v", srow, ok, err)
		}
	}
	if _, ok, err := s.Create(ctx, b, bCat, KindExpense, "99.99", "2026-09-15", ""); err != nil || !ok {
		t.Fatalf("seed B: %v %v", ok, err)
	}

	rows, total, items, budget, ok, err := s.Summary(ctx, a, "2026-09-01", "2026-09-30", nil)
	if err != nil || !ok {
		t.Fatalf("Summary = %v %v", ok, err)
	}
	if total != "17.50" {
		t.Fatalf("total = %q, want 17.50 (string, no float)", total)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 categories", rows)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.CategoryName] = r.Total
	}
	if got["Zakupy"] != "12.50" || got["Transport"] != "5.00" {
		t.Fatalf("rows = %+v, want Zakupy=12.50 Transport=5.00", rows)
	}
	if rows[0].ParentName != "Jedzenie" && rows[1].ParentName != "Jedzenie" {
		t.Fatalf("subcategory row missing parent_name: %+v", rows)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3 (september only, B isolated)", len(items))
	}
	if items[0].OccurredOn.Format("2006-01-02") != "2026-09-20" {
		t.Fatalf("first item = %s, want newest 2026-09-20", items[0].OccurredOn.Format("2006-01-02"))
	}
	if budget.ExpenseTotal != "17.50" || budget.IncomeTotal != "0" || budget.RatioPct != nil {
		got := "<nil>"
		if budget.RatioPct != nil {
			got = *budget.RatioPct
		}
		t.Fatalf("budget = %+v ratio=%s, want 17.50/0/nil (no income seeded)", budget, got)
	}

	// Multi-select narrows to one category.
	rows, total, items, budget, ok, err = s.Summary(ctx, a, "2026-09-01", "2026-09-30", []string{aTrans})
	if err != nil || !ok {
		t.Fatalf("Summary filtered = %v %v", ok, err)
	}
	if total != "5.00" || len(rows) != 1 || len(items) != 1 {
		t.Fatalf("filtered total=%q rows=%+v items=%d, want 5.00/1/1", total, rows, len(items))
	}
	// Filtered view narrows rows but budget stays whole-period.
	if budget.ExpenseTotal != "17.50" || budget.IncomeTotal != "0" || budget.RatioPct != nil {
		t.Fatalf("filtered budget = %+v, want whole-period 17.50/0/nil", budget)
	}

	// Empty but valid range is 200-shape with zeros, not a rejection.
	rows, total, items, budget, ok, err = s.Summary(ctx, a, "2025-01-01", "2025-01-31", nil)
	if err != nil || !ok {
		t.Fatalf("Summary empty = %v %v", ok, err)
	}
	if total != "0" || len(rows) != 0 || len(items) != 0 {
		t.Fatalf("empty total=%q rows=%d items=%d, want 0/0/0", total, len(rows), len(items))
	}
	if budget.ExpenseTotal != "0" || budget.IncomeTotal != "0" || budget.RatioPct != nil {
		t.Fatalf("empty budget = %+v, want 0/0/nil", budget)
	}
}

func TestSummaryBudgetRatio(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	aExp := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	aInc := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	bInc := createCategory(t, ctx, db, b, KindIncome, "Obce wpływy")

	if _, ok, err := s.Create(ctx, a, aExp, KindExpense, "12.50", "2026-09-05", ""); err != nil || !ok {
		t.Fatalf("seed expense: %v %v", ok, err)
	}
	if _, ok, err := s.Create(ctx, a, aInc, KindIncome, "20.00", "2026-09-06", ""); err != nil || !ok {
		t.Fatalf("seed income: %v %v", ok, err)
	}
	if _, ok, err := s.Create(ctx, b, bInc, KindIncome, "999.00", "2026-09-07", ""); err != nil || !ok {
		t.Fatalf("seed foreign income: %v %v", ok, err)
	}

	_, _, _, budget, ok, err := s.Summary(ctx, a, "2026-09-01", "2026-09-30", nil)
	if err != nil || !ok {
		t.Fatalf("Summary = %v %v", ok, err)
	}
	if budget.ExpenseTotal != "12.50" || budget.IncomeTotal != "20.00" {
		t.Fatalf("budget totals = %+v, want 12.50/20.00 (B isolated)", budget)
	}
	if budget.RatioPct == nil || *budget.RatioPct != "62.5" {
		got := "<nil>"
		if budget.RatioPct != nil {
			got = *budget.RatioPct
		}
		t.Fatalf("ratio = %s, want 62.5 (string, no float)", got)
	}

	// Income rows never leak into the expense breakdown.
	rows, total, _, _, ok, err := s.Summary(ctx, a, "2026-09-01", "2026-09-30", nil)
	if err != nil || !ok {
		t.Fatalf("Summary = %v %v", ok, err)
	}
	if total != "12.50" || len(rows) != 1 {
		t.Fatalf("expense view total=%q rows=%d, want 12.50/1", total, len(rows))
	}
}

func TestRatioPct(t *testing.T) {
	for _, tc := range []struct {
		exp, inc string
		want     string // "<nil>" means nil
	}{
		{"12.50", "20.00", "62.5"},
		{"90.00", "100.00", "90.0"},
		{"110.00", "100.00", "110.0"},
		{"0", "100.00", "0.0"},
		{"10.00", "0", "<nil>"},
		{"0", "0", "<nil>"},
	} {
		got := ratioPct(tc.exp, tc.inc)
		if tc.want == "<nil>" {
			if got != nil {
				t.Fatalf("ratioPct(%s,%s) = %q, want nil", tc.exp, tc.inc, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			val := "<nil>"
			if got != nil {
				val = *got
			}
			t.Fatalf("ratioPct(%s,%s) = %s, want %s", tc.exp, tc.inc, val, tc.want)
		}
	}
}

func TestSummaryRejectsInvalidInput(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s := NewStore(db)
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")

	cases := []struct {
		name string
		from string
		to   string
		cats []string
	}{
		{"empty user tested separately", "2026-09-01", "2026-09-30", nil},
		{"bad from", "09-01-2026", "2026-09-30", nil},
		{"bad to", "2026-09-01", "yesterday", nil},
		{"from after to", "2026-09-30", "2026-09-01", nil},
		{"malformed uuid", "2026-09-01", "2026-09-30", []string{"not-a-uuid"}},
		{"unknown uuid", "2026-09-01", "2026-09-30", []string{"00000000-0000-0000-0000-000000000000"}},
		{"foreign category", "2026-09-01", "2026-09-30", []string{foreign}},
		{"income category", "2026-09-01", "2026-09-30", []string{income}},
		{"mixed valid+foreign", "2026-09-01", "2026-09-30", []string{expense, foreign}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := a
			if tc.name == "empty user tested separately" {
				user = ""
			}
			if _, _, _, _, ok, err := s.Summary(ctx, user, tc.from, tc.to, tc.cats); err != nil || ok {
				t.Fatalf("expected ok=false nil error, got ok=%v err=%v", ok, err)
			}
		})
	}
}
