package categories

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/nest-cash/internal/account"
)

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

func decodeGroups(t *testing.T, rec *httptest.ResponseRecorder) []groupDTO {
	t.Helper()
	var out []groupDTO
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	return out
}

func TestHandlerListSeedsFreshAccount(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	h := NewHandler(NewStore(db))

	rec, req := authedRequest(t, http.MethodGet, "/api/categories", userID, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("List = %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	groups := decodeGroups(t, rec)
	if len(groups) == 0 {
		t.Fatal("fresh account should see seeded groups")
	}
	foundKid := false
	for _, g := range groups {
		if len(g.Children) > 0 {
			foundKid = true
		}
		if g.ID == "" || g.Kind == "" || g.Name == "" {
			t.Fatalf("group missing fields: %+v", g)
		}
	}
	if !foundKid {
		t.Fatal("seeded tree should include children")
	}
}

func TestHandlerUnauthorizedWithoutAccount(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	h := NewHandler(NewStore(db))

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/categories", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("List without account = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(`{"kind":"expense","name":"X"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Create without account = %d, want 401", rec.Code)
	}
}

func TestHandlerCreateGroup(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	userID := createTestUser(t, ctx, db)
	h := NewHandler(NewStore(db))

	post := func(body string) *httptest.ResponseRecorder {
		rec, req := authedRequest(t, http.MethodPost, "/api/categories", userID, body)
		h.Create(rec, req)
		return rec
	}

	if rec := post(`{"kind":"expense","name":"Ogród"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create group = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if rec := post(`{"kind":"expense","name":"ogród"}`); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate group = %d, want 409", rec.Code)
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty", `{"kind":"expense","name":"  "}`},
		{"bad kind", `{"kind":"bogus","name":"X"}`},
		{"missing kind", `{"name":"X"}`},
		{"too long", `{"kind":"expense","name":"` + strings.Repeat("a", 81) + `"}`},
		{"bad json", `{"kind":`},
	} {
		if rec := post(tc.body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", tc.name, rec.Code)
		}
	}
}

func TestHandlerCreateSubcategory(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	h := NewHandler(NewStore(db))

	// Seed A via GET so parent IDs exist.
	rec, req := authedRequest(t, http.MethodGet, "/api/categories", a, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed A = %d", rec.Code)
	}
	groups := decodeGroups(t, rec)
	var expenseParent string
	for _, g := range groups {
		if g.Kind == KindExpense {
			expenseParent = g.ID
			break
		}
	}
	if expenseParent == "" {
		t.Fatal("no expense group seeded")
	}

	post := func(userID, body string) *httptest.ResponseRecorder {
		rec, req := authedRequest(t, http.MethodPost, "/api/categories", userID, body)
		h.Create(rec, req)
		return rec
	}

	body := `{"kind":"expense","name":"Nasiona","parent_id":"` + expenseParent + `"}`
	if rec := post(a, body); rec.Code != http.StatusCreated {
		t.Fatalf("create sub = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if rec := post(a, body); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate sub = %d, want 409", rec.Code)
	}
	// Foreign parent (B's account) → 400.
	if rec := post(b, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-user parent = %d, want 400", rec.Code)
	}
	// Kind mismatch → 400.
	mismatch := `{"kind":"income","name":"Nasiona2","parent_id":"` + expenseParent + `"}`
	if rec := post(a, mismatch); rec.Code != http.StatusBadRequest {
		t.Fatalf("kind mismatch = %d, want 400", rec.Code)
	}
	// Unknown parent → 400.
	if rec := post(a, `{"kind":"expense","name":"X","parent_id":"00000000-0000-0000-0000-000000000000"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown parent = %d, want 400", rec.Code)
	}
}

func TestHandlerIsolationBetweenUsers(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	h := NewHandler(NewStore(db))

	rec, req := authedRequest(t, http.MethodPost, "/api/categories", a, `{"kind":"expense","name":"TylkoA"}`)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create A = %d", rec.Code)
	}
	rec, req = authedRequest(t, http.MethodGet, "/api/categories", b, "")
	h.List(rec, req)
	for _, g := range decodeGroups(t, rec) {
		if g.Name == "TylkoA" {
			t.Fatal("user B sees A's group")
		}
	}
}
