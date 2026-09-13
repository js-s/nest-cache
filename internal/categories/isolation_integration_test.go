package categories

import (
	"context"
	"database/sql"
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

// B must not read A's categories and must not write under A's parent id.
// Every step goes through the real session → resolver → handler chain.
func TestCrossAccountIsolationThroughSessions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	store := NewStore(db)
	resolver := auth.NewResolver(auth.NewStore(db))
	h := NewHandler(store)

	aID, aCookie := sessionOwner(t, ctx, db, "IsoCatA")
	_, bCookie := sessionOwner(t, ctx, db, "IsoCatB")

	// A seeds via List, then creates a custom group through its session.
	rec := serve(t, resolver, h.List, http.MethodGet, "/api/categories", "", aCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("A seed list = %d (%s)", rec.Code, rec.Body.String())
	}
	const customName = "TylkoA"
	rec = serve(t, resolver, h.Create, http.MethodPost, "/api/categories",
		`{"kind":"expense","name":"`+customName+`"}`, aCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A create group = %d (%s)", rec.Code, rec.Body.String())
	}
	var aParent string
	for _, g := range decodeGroups(t, serve(t, resolver, h.List, http.MethodGet, "/api/categories", "", aCookie)) {
		if g.Kind == KindExpense {
			aParent = g.ID
			break
		}
	}
	if aParent == "" {
		t.Fatal("no expense group for A")
	}

	// B's list contains none of A's custom names.
	rec = serve(t, resolver, h.List, http.MethodGet, "/api/categories", "", bCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("B list = %d (%s)", rec.Code, rec.Body.String())
	}
	for _, g := range decodeGroups(t, rec) {
		if g.Name == customName {
			t.Fatal("B list leaks A's group")
		}
	}

	// B's write under A's parent_id is rejected and writes nothing.
	before := countRows(t, ctx, store, aID)
	rec = serve(t, resolver, h.Create, http.MethodPost, "/api/categories",
		`{"kind":"expense","name":"Podkradziona","parent_id":"`+aParent+`"}`, bCookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("B cross-account create = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if got := countRows(t, ctx, store, aID); got != before {
		t.Fatalf("A category count %d -> %d after rejected B write", before, got)
	}

	// B's own chain works: B creates its own group.
	rec = serve(t, resolver, h.Create, http.MethodPost, "/api/categories",
		`{"kind":"expense","name":"WlasnaB"}`, bCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("B create own group = %d (%s)", rec.Code, rec.Body.String())
	}
}
