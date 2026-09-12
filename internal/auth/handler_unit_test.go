package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidEmail(t *testing.T) {
	valid := []string{"a@b.co", "user.name+tag@example.com"}
	invalid := []string{"nope", "a@b", "a@b.", "a@b..c", "@b.co", "a b@example.com"}
	for _, e := range valid {
		if !validEmail(e) {
			t.Errorf("validEmail(%q) = false, want true", e)
		}
	}
	for _, e := range invalid {
		if validEmail(e) {
			t.Errorf("validEmail(%q) = true, want false", e)
		}
	}
}

func TestDummyPasswordHash(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil || cost != bcryptCost {
		t.Fatalf("dummy hash cost = %d, %v; want %d", cost, err, bcryptCost)
	}
	if CheckPassword(dummyPasswordHash, "anything") {
		t.Fatal("dummy hash must never match")
	}
}

func TestLimitByIP(t *testing.T) {
	h := NewHandler(nil, false)
	nextCalled := 0
	guarded := h.Limit(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	last := 0
	for i := 0; i <= authBurst; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(""))
		req.RemoteAddr = "203.0.113.7:1234"
		rec := httptest.NewRecorder()
		guarded(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("request %d = %d, want 429", authBurst+1, last)
	}
	if nextCalled != authBurst {
		t.Fatalf("allowed %d requests, want %d", nextCalled, authBurst)
	}

	// A different IP is not throttled by the first IP's bucket.
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(""))
	req.RemoteAddr = "198.51.100.9:1234"
	rec := httptest.NewRecorder()
	guarded(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("second IP = %d, want 200", rec.Code)
	}
}
