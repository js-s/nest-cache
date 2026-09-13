package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/nest-cash/internal/auth"
	"github.com/user/nest-cash/internal/categories"
	"github.com/user/nest-cash/internal/transactions"
)

// Guards that every protected route stays behind RequireAccount.
// DB-free by design: resolver has a nil store so it fails closed to 401,
// and handlers are never reached so their nil stores are never touched.
func TestRoutesRejectUnauthenticated(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>nest-cash</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := (application{
		staticDir:    staticDir,
		auth:         auth.NewHandler(nil, false),
		resolver:     auth.NewResolver(nil),
		categories:   categories.NewHandler(nil),
		transactions: transactions.NewHandler(nil),
	}).routes()

	protected := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/categories"},
		{http.MethodPost, "/api/categories"},
		{http.MethodGet, "/api/transactions"},
		{http.MethodPost, "/api/transactions"},
		{http.MethodGet, "/api/summary"},
	}

	for _, route := range protected {
		for _, cookieCase := range []struct {
			name   string
			cookie *http.Cookie
		}{
			{"no cookie", nil},
			{"bogus session", &http.Cookie{Name: auth.CookieName, Value: "bogus-token"}},
		} {
			t.Run(route.method+" "+route.path+" "+cookieCase.name, func(t *testing.T) {
				request := httptest.NewRequest(route.method, route.path, nil)
				if cookieCase.cookie != nil {
					request.AddCookie(cookieCase.cookie)
				}
				response := httptest.NewRecorder()

				handler.ServeHTTP(response, request)

				if response.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
				}
				if ct := response.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
					t.Fatalf("content type = %q, want prefix %q", ct, "application/json")
				}
				if body := response.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
					t.Fatalf("body = %q, want it to contain %q", body, `"error":"unauthorized"`)
				}
				if setCookie := response.Header().Get("Set-Cookie"); setCookie != "" {
					t.Fatalf("Set-Cookie = %q, want empty", setCookie)
				}
			})
		}
	}

	t.Run("open routes stay distinguishable", func(t *testing.T) {
		cases := []struct {
			name       string
			method     string
			path       string
			wantStatus int
			wantBody   string
		}{
			{"healthz degraded without db", http.MethodGet, "/healthz", http.StatusServiceUnavailable, ""},
			{"unknown api stays 404", http.MethodGet, "/api/unknown", http.StatusNotFound, `"error":"not_found"`},
			{"root serves spa", http.MethodGet, "/", http.StatusOK, "<html>nest-cash</html>"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				request := httptest.NewRequest(tc.method, tc.path, nil)
				response := httptest.NewRecorder()

				handler.ServeHTTP(response, request)

				if response.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d", response.Code, tc.wantStatus)
				}
				if tc.wantBody != "" && !strings.Contains(response.Body.String(), tc.wantBody) {
					t.Fatalf("body = %q, want it to contain %q", response.Body.String(), tc.wantBody)
				}
			})
		}
	})
}
