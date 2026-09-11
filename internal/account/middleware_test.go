package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubResolver struct {
	id AccountID
	ok bool
}

func (s stubResolver) ResolveAccountID(*http.Request) (AccountID, bool) {
	return s.id, s.ok
}

func TestRequireAccountRejectsMissingResolver(t *testing.T) {
	called := false
	handler := RequireAccount(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	assertUnauthorized(t, response)
	if called {
		t.Fatal("downstream handler called, want not called")
	}
}

func TestRequireAccountRejectsUnresolved(t *testing.T) {
	called := false
	handler := RequireAccount(stubResolver{id: "acct-secret", ok: false}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	assertUnauthorized(t, response)
	if called {
		t.Fatal("downstream handler called, want not called")
	}
	if strings.Contains(response.Body.String(), "acct-secret") {
		t.Error("error response leaked resolver identifier")
	}
}

func TestRequireAccountRejectsInvalidID(t *testing.T) {
	for _, id := range []AccountID{"", "   ", "\t"} {
		called := false
		handler := RequireAccount(stubResolver{id: id, ok: true}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

		assertUnauthorized(t, response)
		if called {
			t.Fatalf("downstream handler called for invalid AccountID(%q)", id)
		}
	}
}

func TestRequireAccountAllowsResolved(t *testing.T) {
	var got AccountID
	handler := RequireAccount(stubResolver{id: "acct-42", ok: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = AccountIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got != "acct-42" {
		t.Fatalf("context AccountID = %q, want %q", got, "acct-42")
	}
}

func assertUnauthorized(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", body["error"], "unauthorized")
	}
}
