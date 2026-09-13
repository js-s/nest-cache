package transactions

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/nest-cash/internal/account"
	"github.com/user/nest-cash/internal/auth"
)

// sessionOwner creates a real user + live session through the auth store and
// returns the user id with a request cookie. Exercises the resolver chain
// instead of injecting the account id into the context.
func sessionOwner(t *testing.T, ctx context.Context, db *sql.DB, tag string) (string, *http.Cookie) {
	t.Helper()
	s := auth.NewStore(db)
	user, ok, err := s.CreateUser(ctx, tag+"+"+time.Now().Format("150405.000000000")+"@example.com", "hash-1")
	if err != nil || !ok {
		t.Fatalf("CreateUser %s: %+v %v", tag, user, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if err := s.CreateSession(ctx, hash, user.ID, time.Now().Add(auth.SessionDuration)); err != nil {
		t.Fatalf("CreateSession %s: %v", tag, err)
	}
	return user.ID, &http.Cookie{Name: auth.CookieName, Value: raw}
}

// serve drives a handler through RequireAccount with a session cookie.
func serve(t *testing.T, resolver *auth.Resolver, h http.HandlerFunc, method, target, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	account.RequireAccount(resolver, h).ServeHTTP(rec, req)
	return rec
}

func txCount(t *testing.T, ctx context.Context, db *sql.DB, userID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	return n
}

// B must not read A's rows and must not write through A's resource ids.
// Every step goes through the real session → resolver → handler chain.
func TestCrossAccountIsolationThroughSessions(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	resolver := auth.NewResolver(auth.NewStore(db))
	h := NewHandler(NewStore(db))

	aID, aCookie := sessionOwner(t, ctx, db, "IsoTxA")
	_, bCookie := sessionOwner(t, ctx, db, "IsoTxB")
	aCat := createCategory(t, ctx, db, aID, KindExpense, "WydatkiA")

	// A creates a transaction through its own session.
	rec := serve(t, resolver, h.Create, http.MethodPost, "/api/transactions",
		`{"amount":"12.50","category_id":"`+aCat+`","occurred_on":"2026-09-05"}`, aCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A create = %d (%s)", rec.Code, rec.Body.String())
	}
	var created transactionDTO
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// B's list shows none of A's rows.
	rec = serve(t, resolver, h.List, http.MethodGet, "/api/transactions", "", bCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("B list = %d (%s)", rec.Code, rec.Body.String())
	}
	var page listDTO
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode B list: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("B list total=%d len=%d, want 0/0", page.Total, len(page.Items))
	}
	for _, item := range page.Items {
		if item.ID == created.ID {
			t.Fatal("B list leaks A's transaction id")
		}
	}

	// B's summary totals zero, no rows.
	rec = serve(t, resolver, h.Summary, http.MethodGet, "/api/summary?from=2026-09-01&to=2026-09-30", "", bCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("B summary = %d (%s)", rec.Code, rec.Body.String())
	}
	var out summaryDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode B summary: %v", err)
	}
	if out.Total != "0" || len(out.Rows) != 0 || len(out.Items) != 0 {
		t.Fatalf("B summary leaks A: total=%q rows=%d items=%d", out.Total, len(out.Rows), len(out.Items))
	}

	// B's write through A's category_id is rejected and writes nothing.
	before := txCount(t, ctx, db, aID)
	rec = serve(t, resolver, h.Create, http.MethodPost, "/api/transactions",
		`{"amount":"99.99","category_id":"`+aCat+`","occurred_on":"2026-09-06"}`, bCookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("B cross-account create = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if got := txCount(t, ctx, db, aID); got != before {
		t.Fatalf("A transaction count %d -> %d after rejected B write", before, got)
	}

	// B's summary scoped to A's category is rejected.
	rec = serve(t, resolver, h.Summary, http.MethodGet,
		"/api/summary?from=2026-09-01&to=2026-09-30&category_id="+aCat, "", bCookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("B cross-account summary = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}
