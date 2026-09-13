package transactions

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Invalid input is rejected with 400 + invalid_request and writes nothing;
// valid input is accepted. txCount (isolation file) proves the no-write side.
func TestValidationRejectsNothingWritten(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wplywy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")
	h := NewHandler(NewStore(db))

	// Valid case writes exactly one row.
	rec, req := authedRequest(t, http.MethodPost, "/api/transactions", a,
		`{"amount":"12.34","category_id":"`+expense+`","occurred_on":"2026-02-03"}`)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid create = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := txCount(t, ctx, db, a); got != 1 {
		t.Fatalf("count after valid = %d, want 1", got)
	}

	valid := func(cat, amount, date string) string {
		return `{"amount":"` + amount + `","category_id":"` + cat + `","occurred_on":"` + date + `"}`
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{"bad json", `{"amount":`},
		{"zero", valid(expense, "0.00", "2026-02-03")},
		{"negative", valid(expense, "-5.00", "2026-02-03")},
		{"too many decimals", valid(expense, "1.234", "2026-02-03")},
		{"not a number", valid(expense, "abc", "2026-02-03")},
		{"empty amount", valid(expense, "", "2026-02-03")},
		{"bad date", valid(expense, "5.00", "03-02-2026")},
		{"empty category", valid("", "5.00", "2026-02-03")},
		{"foreign category", valid(foreign, "5.00", "2026-02-03")},
		{"income category", valid(income, "5.00", "2026-02-03")},
		{"unknown category", valid("00000000-0000-0000-0000-000000000000", "5.00", "2026-02-03")},
		{"bad kind", `{"amount":"5.00","category_id":"` + expense + `","occurred_on":"2026-02-03","kind":"grant"}`},
		{"kind mismatch", `{"amount":"5.00","category_id":"` + income + `","occurred_on":"2026-02-03","kind":"expense"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodPost, "/api/transactions", a, tc.body)
			h.Create(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, `"error":"invalid_request"`) {
				t.Fatalf("body = %q, want invalid_request", body)
			}
			if got := txCount(t, ctx, db, a); got != 1 {
				t.Fatalf("count = %d, want 1 (rejected write mutated rows)", got)
			}
		})
	}

	// Semantically invalid filters are 400.
	for _, target := range []string{
		"/api/transactions?kind=grant",
		"/api/summary?to=2026-09-30",
		"/api/summary?from=2026-09-01",
		"/api/summary?from=x&to=2026-09-30",
		"/api/summary?from=2026-09-30&to=2026-09-01",
		"/api/summary?from=2026-09-01&to=2026-09-30&category_id=nope",
		"/api/summary?from=2026-09-01&to=2026-09-30&category_id=00000000-0000-0000-0000-000000000000",
		"/api/summary?from=2026-09-01&to=2026-09-30&category_id=" + foreign,
		"/api/summary?from=2026-09-01&to=2026-09-30&category_id=" + income,
	} {
		t.Run("filter "+target, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodGet, target, a, "")
			if strings.HasPrefix(target, "/api/summary") {
				h.Summary(rec, req)
			} else {
				h.List(rec, req)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}

	// Lenient boundary (documented, not a 400): malformed page/limit default.
	rec, req = authedRequest(t, http.MethodGet, "/api/transactions?page=abc&limit=-1", a, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lenient page/limit = %d, want 200", rec.Code)
	}
	var page listDTO
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if page.Page != 1 || page.Limit != 20 {
		t.Fatalf("page/limit = %d/%d, want 1/20 defaults", page.Page, page.Limit)
	}
}
