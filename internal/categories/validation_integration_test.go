package categories

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Invalid input is rejected with 400 + invalid_request and writes nothing;
// a valid group is accepted. countRows proves the no-write side.
func TestValidationRejectsNothingWritten(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	a := createTestUser(t, ctx, db)
	b := createTestUser(t, ctx, db)
	store := NewStore(db)
	h := NewHandler(store)

	// B seeds via List so a foreign parent id exists.
	rec, req := authedRequest(t, http.MethodGet, "/api/categories", b, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed B = %d", rec.Code)
	}
	var foreignParent string
	for _, g := range decodeGroups(t, rec) {
		if g.Kind == KindExpense {
			foreignParent = g.ID
			break
		}
	}
	if foreignParent == "" {
		t.Fatal("no expense group seeded for B")
	}

	// A seeds via List so a same-kind parent exists for the mismatch case.
	rec, req = authedRequest(t, http.MethodGet, "/api/categories", a, "")
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed A = %d", rec.Code)
	}
	var ownParent string
	for _, g := range decodeGroups(t, rec) {
		if g.Kind == KindExpense {
			ownParent = g.ID
			break
		}
	}
	if ownParent == "" {
		t.Fatal("no expense group seeded for A")
	}
	before := countRows(t, ctx, store, a)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty name", `{"kind":"expense","name":""}`},
		{"whitespace name", `{"kind":"expense","name":"   "}`},
		{"missing kind", `{"name":"X"}`},
		{"bad kind", `{"kind":"bogus","name":"X"}`},
		{"too long", `{"kind":"expense","name":"` + strings.Repeat("a", 81) + `"}`},
		{"bad json", `{"kind":`},
		{"foreign parent", `{"kind":"expense","name":"Cudza","parent_id":"` + foreignParent + `"}`},
		{"kind mismatch", `{"kind":"income","name":"Zla","parent_id":"` + ownParent + `"}`},
		{"unknown parent", `{"kind":"expense","name":"X","parent_id":"00000000-0000-0000-0000-000000000000"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, req := authedRequest(t, http.MethodPost, "/api/categories", a, tc.body)
			h.Create(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, `"error":"invalid_request"`) {
				t.Fatalf("body = %q, want invalid_request", body)
			}
			if got := countRows(t, ctx, store, a); got != before {
				t.Fatalf("count = %d, want %d (rejected write mutated rows)", got, before)
			}
		})
	}

	// Valid group writes exactly one row.
	rec, req = authedRequest(t, http.MethodPost, "/api/categories", a, `{"kind":"expense","name":"Ogród"}`)
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid create = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := countRows(t, ctx, store, a); got != before+1 {
		t.Fatalf("count = %d, want %d", got, before+1)
	}
}
