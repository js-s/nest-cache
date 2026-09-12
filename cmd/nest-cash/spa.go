package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/user/nest-cash/internal/account"
)

func (a application) routes() http.Handler {
	staticDir := a.staticDir
	if staticDir == "" {
		staticDir = "web/dist"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	if a.auth != nil && a.resolver != nil {
		mux.HandleFunc("POST /api/auth/register", a.auth.Limit(a.auth.Register))
		mux.HandleFunc("POST /api/auth/login", a.auth.Limit(a.auth.Login))
		mux.HandleFunc("POST /api/auth/logout", a.auth.Logout)
		mux.Handle("GET /api/auth/me", account.RequireAccount(a.resolver, http.HandlerFunc(a.auth.Me)))
	}
	if a.categories != nil && a.resolver != nil {
		mux.Handle("GET /api/categories", account.RequireAccount(a.resolver, http.HandlerFunc(a.categories.List)))
		mux.Handle("POST /api/categories", account.RequireAccount(a.resolver, http.HandlerFunc(a.categories.Create)))
	}
	if a.transactions != nil && a.resolver != nil {
		mux.Handle("GET /api/transactions", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.List)))
		mux.Handle("POST /api/transactions", account.RequireAccount(a.resolver, http.HandlerFunc(a.transactions.Create)))
	}
	mux.HandleFunc("/api", a.apiNotFound)
	mux.Handle("/api/", http.HandlerFunc(a.apiNotFound))
	mux.Handle("/", newSPAHandler(staticDir))

	return mux
}

func (a application) apiNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "not_found",
	})
}

type spaHandler struct {
	root string
}

func newSPAHandler(root string) http.Handler {
	return spaHandler{root: root}
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method_not_allowed",
		})
		return
	}

	relativePath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if relativePath != "" && relativePath != "." {
		filePath := filepath.Join(h.root, filepath.FromSlash(relativePath))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			if strings.HasPrefix(relativePath, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, r, filePath)
			return
		}

		if strings.HasPrefix(relativePath, "assets/") || path.Ext(relativePath) != "" {
			http.NotFound(w, r)
			return
		}
	}

	indexPath := filepath.Join(h.root, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "frontend_not_built",
		})
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, indexPath)
}
