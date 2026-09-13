package transactions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/nest-cash/internal/account"
)

// uuidZero is a well-formed but never-existing transaction id.
const uuidZero = "00000000-0000-0000-0000-000000000000"

func authedRequest(t *testing.T, method, target, userID, body string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req = req.WithContext(account.WithAccountID(req.Context(), account.AccountID(userID)))
	return httptest.NewRecorder(), req
}

func TestHandlerCreateAndList(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, userID, KindExpense, "Jedzenie")
	h := NewHandler(NewStore(db))

	body := `{"amount":"12.34","category_id":"` + cat + `","occurred_on":"2026-02-03","description":"obiad"}`
	rec, req := authedRequest(t, http.MethodPost, "/api/transactions", userID, body)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	var created transactionDTO
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == "" || created.Amount != "12.34" || created.Kind != KindExpense || created.CategoryName != "Jedzenie" || created.OccurredOn != "2026-02-03" || created.Description != "obiad" {
		t.Fatalf("unexpected DTO: %+v", created)
	}
	if created.ParentName != "" {
		t.Fatalf("group-level parent_name = %q, want omitted", created.ParentName)
	}

	rec, req = authedRequest(t, http.MethodGet, "/api/transactions?page=1&limit=2", userID, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("List = %d, want 200", rec.Code)
	}
	var page listDTO
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if page.Total != 1 || page.Page != 1 || page.Limit != 2 || len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("unexpected list: %+v", page)
	}
}

func TestHandlerCreateValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")
	h := NewHandler(NewStore(db))

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
		{"bad kind", `{"amount":"5.00","category_id":"` + expense + `","occurred_on":"2026-02-03","kind":"grant"}`},
		{"kind mismatch", `{"amount":"5.00","category_id":"` + income + `","occurred_on":"2026-02-03","kind":"expense"}`},
		{"unknown category", valid("00000000-0000-0000-0000-000000000000", "5.00", "2026-02-03")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodPost, "/api/transactions", a, tc.body)
			h.Create(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandlerCreateIncomeAndListKind(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, userID, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, userID, KindIncome, "Wpływy")
	h := NewHandler(NewStore(db))

	body := `{"amount":"100.00","category_id":"` + income + `","occurred_on":"2026-03-02","kind":"income"}`
	rec, req := authedRequest(t, http.MethodPost, "/api/transactions", userID, body)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create income = %d (%s)", rec.Code, rec.Body.String())
	}
	var created transactionDTO
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Kind != KindIncome {
		t.Fatalf("kind = %q, want income", created.Kind)
	}

	body = `{"amount":"10.00","category_id":"` + expense + `","occurred_on":"2026-03-01"}`
	rec, req = authedRequest(t, http.MethodPost, "/api/transactions", userID, body)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create default kind = %d (%s)", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		target string
		total  int
		kind   string
	}{
		{"/api/transactions?kind=income", 1, KindIncome},
		{"/api/transactions?kind=expense", 1, KindExpense},
		{"/api/transactions?kind=all", 2, ""},
		{"/api/transactions", 2, ""},
	} {
		rec, req := authedRequest(t, http.MethodGet, tc.target, userID, "")
		h.List(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("List %s = %d", tc.target, rec.Code)
		}
		var page listDTO
		if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if page.Total != tc.total || len(page.Items) != tc.total {
			t.Fatalf("List %s total=%d len=%d, want %d", tc.target, page.Total, len(page.Items), tc.total)
		}
		if tc.kind != "" && page.Items[0].Kind != tc.kind {
			t.Fatalf("List %s kind=%q, want %q", tc.target, page.Items[0].Kind, tc.kind)
		}
	}

	rec, req = authedRequest(t, http.MethodGet, "/api/transactions?kind=grant", userID, "")
	h.List(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad kind = %d, want 400", rec.Code)
	}
}

func TestSummaryHandler(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, userID, KindExpense, "Jedzenie")
	h := NewHandler(NewStore(db))

	body := `{"amount":"12.50","category_id":"` + cat + `","occurred_on":"2026-09-05"}`
	rec, req := authedRequest(t, http.MethodPost, "/api/transactions", userID, body)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed Create = %d (%s)", rec.Code, rec.Body.String())
	}

	rec, req = authedRequest(t, http.MethodGet, "/api/summary?from=2026-09-01&to=2026-09-30", userID, "")
	h.Summary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Summary = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	var out summaryDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if out.From != "2026-09-01" || out.To != "2026-09-30" || out.Total != "12.50" {
		t.Fatalf("unexpected summary: %+v", out)
	}
	if len(out.Rows) != 1 || out.Rows[0].Total != "12.50" || len(out.Items) != 1 {
		t.Fatalf("unexpected rows/items: %+v", out)
	}
	if out.Budget.ExpenseTotal != "12.50" || out.Budget.IncomeTotal != "0" || out.Budget.RatioPct != nil {
		t.Fatalf("unexpected budget: %+v (want 12.50/0/nil)", out.Budget)
	}

	for _, tc := range []struct {
		name   string
		target string
	}{
		{"missing from", "/api/summary?to=2026-09-30"},
		{"missing to", "/api/summary?from=2026-09-01"},
		{"bad date", "/api/summary?from=x&to=2026-09-30"},
		{"from after to", "/api/summary?from=2026-09-30&to=2026-09-01"},
		{"bad uuid", "/api/summary?from=2026-09-01&to=2026-09-30&category_id=nope"},
		{"unknown uuid", "/api/summary?from=2026-09-01&to=2026-09-30&category_id=00000000-0000-0000-0000-000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodGet, tc.target, userID, "")
			h.Summary(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}

	rec = httptest.NewRecorder()
	h.Summary(rec, httptest.NewRequest(http.MethodGet, "/api/summary?from=2026-09-01&to=2026-09-30", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Summary without account = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Summary(rec, httptest.NewRequest(http.MethodPost, "/api/summary?from=2026-09-01&to=2026-09-30", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Summary wrong method = %d, want 405", rec.Code)
	}
}

func TestHandlerUnauthorizedWithoutAccount(t *testing.T) {
	db := openTestDB(t)
	h := NewHandler(NewStore(db))

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/transactions", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("List without account = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodPost, "/api/transactions", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Create without account = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Update(rec, httptest.NewRequest(http.MethodPut, "/api/transactions/"+uuidZero, strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Update without account = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Delete(rec, httptest.NewRequest(http.MethodDelete, "/api/transactions/"+uuidZero, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Delete without account = %d, want 401", rec.Code)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	db := openTestDB(t)
	h := NewHandler(NewStore(db))

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodPost, "/api/transactions", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("List wrong method = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodGet, "/api/transactions", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Create wrong method = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Update(rec, httptest.NewRequest(http.MethodGet, "/api/transactions/"+uuidZero, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Update wrong method = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Delete(rec, httptest.NewRequest(http.MethodPost, "/api/transactions/"+uuidZero, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Delete wrong method = %d, want 405", rec.Code)
	}
}

func TestTransactionsHandlerUpdate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, userID, KindExpense, "Jedzenie")
	other := createCategory(t, ctx, db, userID, KindExpense, "Transport")
	h := NewHandler(NewStore(db))

	rec, req := authedRequest(t, http.MethodPost, "/api/transactions", userID,
		`{"amount":"12.34","category_id":"`+cat+`","occurred_on":"2026-02-03","description":"obiad"}`)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed Create = %d (%s)", rec.Code, rec.Body.String())
	}
	var created transactionDTO
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	rec, req = authedRequest(t, http.MethodPut, "/api/transactions/"+created.ID, userID,
		`{"amount":"55.00","category_id":"`+other+`","occurred_on":"2026-02-10","description":"przejazd"}`)
	req.SetPathValue("id", created.ID)
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Update = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var updated transactionDTO
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.ID != created.ID || updated.Kind != KindExpense || updated.Amount != "55.00" ||
		updated.CategoryID != other || updated.CategoryName != "Transport" ||
		updated.OccurredOn != "2026-02-10" || updated.Description != "przejazd" {
		t.Fatalf("unexpected DTO: %+v", updated)
	}

	rec, req = authedRequest(t, http.MethodGet, "/api/transactions", userID, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("List = %d, want 200", rec.Code)
	}
	var page listDTO
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Amount != "55.00" || page.Items[0].CategoryName != "Transport" {
		t.Fatalf("list not updated: %+v", page)
	}
}

func TestTransactionsHandlerUpdateValidation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	expense := createCategory(t, ctx, db, a, KindExpense, "Wydatki")
	income := createCategory(t, ctx, db, a, KindIncome, "Wpływy")
	foreign := createCategory(t, ctx, db, b, KindExpense, "Obce")
	h := NewHandler(NewStore(db))

	seed, ok, err := h.store.Create(ctx, a, expense, KindExpense, "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	valid := func(cat, amount, date string) string {
		return `{"amount":"` + amount + `","category_id":"` + cat + `","occurred_on":"` + date + `"}`
	}

	for _, tc := range []struct {
		name string
		id   string
		body string
		want int
	}{
		{"bad json", seed.ID, `{"amount":`, http.StatusBadRequest},
		{"malformed id", "not-a-uuid", valid(expense, "5.00", "2026-02-03"), http.StatusBadRequest},
		{"empty amount", seed.ID, valid(expense, "", "2026-02-03"), http.StatusBadRequest},
		{"empty category", seed.ID, valid("", "5.00", "2026-02-03"), http.StatusBadRequest},
		{"bad date", seed.ID, valid(expense, "5.00", "03-02-2026"), http.StatusBadRequest},
		{"foreign category", seed.ID, valid(foreign, "5.00", "2026-02-03"), http.StatusBadRequest},
		{"other kind category", seed.ID, valid(income, "5.00", "2026-02-03"), http.StatusBadRequest},
		{"unknown id", uuidZero, valid(expense, "5.00", "2026-02-03"), http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodPut, "/api/transactions/"+tc.id, a, tc.body)
			req.SetPathValue("id", tc.id)
			h.Update(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("code = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestTransactionsHandlerDelete(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	cat := createCategory(t, ctx, db, userID, KindExpense, "Wydatki")
	h := NewHandler(NewStore(db))

	seed, ok, err := h.store.Create(ctx, userID, cat, KindExpense, "5.00", "2026-02-03", "")
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}

	rec, req := authedRequest(t, http.MethodDelete, "/api/transactions/"+seed.ID, userID, "")
	req.SetPathValue("id", seed.ID)
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Delete = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("Delete body = %q, want empty", rec.Body.String())
	}

	rec, req = authedRequest(t, http.MethodDelete, "/api/transactions/"+seed.ID, userID, "")
	req.SetPathValue("id", seed.ID)
	h.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("repeat Delete = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}

	rec, req = authedRequest(t, http.MethodDelete, "/api/transactions/not-a-uuid", userID, "")
	req.SetPathValue("id", "not-a-uuid")
	h.Delete(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed Delete = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}
