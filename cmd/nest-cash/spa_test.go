package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoutesKeepAPIBoundaryAndServeSPA(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>nest-cash</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(staticDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := (application{staticDir: staticDir}).routes()

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string
		contentType string
		cacheHeader string
	}{
		{
			name:        "health check stays JSON",
			method:      http.MethodGet,
			path:        "/healthz",
			wantStatus:  http.StatusServiceUnavailable,
			contentType: "application/json",
		},
		{
			name:        "unknown API route stays JSON",
			method:      http.MethodGet,
			path:        "/api/unknown",
			wantStatus:  http.StatusNotFound,
			wantBody:    `"error":"not_found"`,
			contentType: "application/json",
		},
		{
			name:        "API root stays JSON",
			method:      http.MethodGet,
			path:        "/api",
			wantStatus:  http.StatusNotFound,
			wantBody:    `"error":"not_found"`,
			contentType: "application/json",
		},
		{
			name:        "root serves the SPA",
			method:      http.MethodGet,
			path:        "/",
			wantStatus:  http.StatusOK,
			wantBody:    "<html>nest-cash</html>",
			contentType: "text/html",
			cacheHeader: "no-cache",
		},
		{
			name:        "history route falls back to the SPA",
			method:      http.MethodGet,
			path:        "/summary/month",
			wantStatus:  http.StatusOK,
			wantBody:    "<html>nest-cash</html>",
			contentType: "text/html",
			cacheHeader: "no-cache",
		},
		{
			name:        "HEAD history route keeps the SPA fallback",
			method:      http.MethodHead,
			path:        "/summary/month",
			wantStatus:  http.StatusOK,
			contentType: "text/html",
			cacheHeader: "no-cache",
		},
		{
			name:        "asset is served with immutable cache",
			method:      http.MethodGet,
			path:        "/assets/app.js",
			wantStatus:  http.StatusOK,
			wantBody:    "console.log('ok')",
			cacheHeader: "public, max-age=31536000, immutable",
		},
		{
			name:       "missing asset is not an SPA route",
			method:     http.MethodGet,
			path:       "/assets/missing.js",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantBody != "" && !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", response.Body.String(), test.wantBody)
			}
			if test.contentType != "" && !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type = %q, want prefix %q", response.Header().Get("Content-Type"), test.contentType)
			}
			if test.cacheHeader != "" && response.Header().Get("Cache-Control") != test.cacheHeader {
				t.Fatalf("cache control = %q, want %q", response.Header().Get("Cache-Control"), test.cacheHeader)
			}
		})
	}
}
