package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/nest-cash/internal/account"
)

func testHandler(t *testing.T) (*Handler, *Resolver) {
	t.Helper()
	db := openTestDB(t)
	t.Cleanup(func() { db.Close() })
	s := NewStore(db)
	return NewHandler(s, false), NewResolver(s)
}

func doRequest(t *testing.T, h http.HandlerFunc, method, target, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	t.Fatal("no nest_session cookie set")
	return nil
}

func TestHandlerCycle(t *testing.T) {
	h, resolver := testHandler(t)
	email := "Cycle+" + time.Now().Format("150405.000000") + "@example.com"

	// Register → 201 + cookie + flags.
	rec := doRequest(t, h.Register, "POST", "/api/auth/register",
		`{"email":"`+email+`","password":"password-123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register = %d %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookie(t, rec)
	assertCookieFlags(t, cookie, false)

	// Me behind RequireAccount → 200 {id,email}.
	me := account.RequireAccount(resolver, http.HandlerFunc(h.Me))
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	me.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me = %d %s", rec.Code, rec.Body.String())
	}
	var p profile
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil || p.Email != NormalizeEmail(email) || p.ID == "" {
		t.Fatalf("me body = %s, err %v", rec.Body.String(), err)
	}

	// Sliding refresh re-sets the cookie on me.
	if refreshed := sessionCookie(t, rec); refreshed.Value == "" {
		t.Fatal("me should refresh the session cookie")
	}

	// Logout → 204 + cleared cookie; me after → 401.
	rec = doRequest(t, h.Logout, "POST", "/api/auth/logout", "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d %s", rec.Code, rec.Body.String())
	}
	cleared := sessionCookie(t, rec)
	if cleared.MaxAge != -1 {
		t.Fatalf("logout cookie MaxAge = %d, want -1", cleared.MaxAge)
	}

	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	me.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout = %d, want 401", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"unauthorized"}` {
		t.Fatalf("me after logout body = %q", rec.Body.String())
	}

	// Login with the same credentials → 200, then me → 200.
	rec = doRequest(t, h.Login, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"password-123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d %s", rec.Code, rec.Body.String())
	}
	cookie = sessionCookie(t, rec)
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	me.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me after login = %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerErrors(t *testing.T) {
	h, _ := testHandler(t)
	email := "Errs+" + time.Now().Format("150405.000000") + "@example.com"
	good := `{"email":"` + email + `","password":"password-123"}`

	cases := []struct {
		name   string
		method string
		target string
		fn     http.HandlerFunc
		body   string
		want   int
	}{
		{"bad json", "POST", "/api/auth/register", h.Register, "{", 400},
		{"bad email", "POST", "/api/auth/register", h.Register, `{"email":"nope","password":"password-123"}`, 400},
		{"short pw", "POST", "/api/auth/register", h.Register, `{"email":"a@b.co","password":"short"}`, 400},
		{"long pw", "POST", "/api/auth/register", h.Register, `{"email":"a@b.co","password":"` + strings.Repeat("a", 73) + `"}`, 400},
		{"login unknown", "POST", "/api/auth/login", h.Login, `{"email":"nobody@example.com","password":"password-123"}`, 401},
		{"me no cookie", "GET", "/api/auth/me", h.Me, "", 401},
	}
	for _, tc := range cases {
		rec := doRequest(t, tc.fn, tc.method, tc.target, tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s = %d, want %d (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}

	// Register once, then duplicate → 409 email_taken.
	if rec := doRequest(t, h.Register, "POST", "/api/auth/register", good); rec.Code != http.StatusCreated {
		t.Fatalf("first register = %d %s", rec.Code, rec.Body.String())
	}
	rec := doRequest(t, h.Register, "POST", "/api/auth/register", good)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "email_taken") {
		t.Fatalf("duplicate register = %d %s, want 409 email_taken", rec.Code, rec.Body.String())
	}
	// Case-insensitive duplicate → 409 too.
	rec = doRequest(t, h.Register, "POST", "/api/auth/register",
		`{"email":"`+strings.ToUpper(email)+`","password":"password-123"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("upper-case duplicate = %d, want 409", rec.Code)
	}

	// Wrong password → uniform 401 identical to unknown-email shape.
	badPW := doRequest(t, h.Login, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"wrong-pass-1"}`)
	unknown := doRequest(t, h.Login, "POST", "/api/auth/login",
		`{"email":"nobody@example.com","password":"wrong-pass-1"}`)
	if badPW.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
		t.Fatalf("bad-pw = %d, unknown = %d, want both 401", badPW.Code, unknown.Code)
	}
	if badPW.Body.String() != unknown.Body.String() {
		t.Fatalf("login oracle: bad-pw %q vs unknown %q", badPW.Body.String(), unknown.Body.String())
	}
}

func TestTwoAccountIsolation(t *testing.T) {
	h, resolver := testHandler(t)
	stamp := time.Now().Format("150405.000000")
	mk := func(tag string) *http.Cookie {
		rec := doRequest(t, h.Register, "POST", "/api/auth/register",
			`{"email":"`+tag+`+`+stamp+`@example.com","password":"password-123"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("register %s = %d %s", tag, rec.Code, rec.Body.String())
		}
		return sessionCookie(t, rec)
	}
	cookieA, cookieB := mk("IsoA"), mk("IsoB")

	meOf := func(c *http.Cookie) profile {
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		account.RequireAccount(resolver, http.HandlerFunc(h.Me)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("me = %d %s", rec.Code, rec.Body.String())
		}
		var p profile
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
			t.Fatalf("decode me: %v", err)
		}
		return p
	}
	a, b := meOf(cookieA), meOf(cookieB)
	if a.ID == b.ID || a.Email == b.Email {
		t.Fatalf("sessions cross accounts: %+v vs %+v", a, b)
	}
}

func TestResolver(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	s := NewStore(db)
	r := NewResolver(s)

	withCookie := func(v string) *http.Request {
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		if v != "" {
			req.AddCookie(&http.Cookie{Name: CookieName, Value: v})
		}
		return req
	}

	// Missing cookie → false.
	if _, ok := r.ResolveAccountID(withCookie("")); ok {
		t.Fatal("missing cookie should not resolve")
	}
	// Unknown token → false.
	if _, ok := r.ResolveAccountID(withCookie("bogus")); ok {
		t.Fatal("unknown token should not resolve")
	}
	// Nil resolver → false, no panic.
	var nilR *Resolver
	if _, ok := nilR.ResolveAccountID(withCookie("x")); ok {
		t.Fatal("nil resolver should not resolve")
	}

	// Live session resolves; expired does not.
	hash, err := HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	user, ok, err := s.CreateUser(ctx, "Resolver+"+time.Now().Format("150405.000000")+"@example.com", hash)
	if err != nil || !ok {
		t.Fatalf("CreateUser: %+v %v %v", user, ok, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})
	raw, tokenHash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if err := s.CreateSession(ctx, tokenHash, user.ID, time.Now().Add(SessionDuration)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	id, ok := r.ResolveAccountID(withCookie(raw))
	if !ok || string(id) != user.ID {
		t.Fatalf("live session resolve = %q %v", id, ok)
	}

	rawExp, hashExp, _ := NewToken()
	if err := s.CreateSession(ctx, hashExp, user.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}
	if _, ok := r.ResolveAccountID(withCookie(rawExp)); ok {
		t.Fatal("expired session should not resolve")
	}
}

func TestSecureCookieGate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewStore(db)
	insecure, secure := NewHandler(s, false), NewHandler(s, true)

	mkReq := func() *http.Request { return httptest.NewRequest("POST", "/api/auth/register", nil) }
	cookieFor := func(h *Handler, req *http.Request) *http.Cookie {
		rec := httptest.NewRecorder()
		h.setCookie(rec, req, "raw-value")
		return sessionCookie(t, rec)
	}

	if c := cookieFor(insecure, mkReq()); c.Secure {
		t.Fatal("localhost dev cookie must not be Secure")
	}
	if c := cookieFor(secure, mkReq()); !c.Secure {
		t.Fatal("prod cookie must be Secure")
	}
	fwd := mkReq()
	fwd.Header.Set("X-Forwarded-Proto", "https")
	if c := cookieFor(insecure, fwd); !c.Secure {
		t.Fatal("https-forwarded cookie must be Secure")
	}
}

func assertCookieFlags(t *testing.T, c *http.Cookie, secure bool) {
	t.Helper()
	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want /", c.Path)
	}
	if !c.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax", c.SameSite)
	}
	if c.MaxAge != SessionMaxAge {
		t.Errorf("cookie MaxAge = %d, want %d", c.MaxAge, SessionMaxAge)
	}
	if c.Secure != secure {
		t.Errorf("cookie Secure = %v, want %v", c.Secure, secure)
	}
}
