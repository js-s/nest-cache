package account

import (
	"encoding/json"
	"net/http"
)

// Resolver is the single trusted source of the owner identifier for a request.
// The session layer implements it in S-01; F-01 only defines the contract.
type Resolver interface {
	ResolveAccountID(r *http.Request) (AccountID, bool)
}

// RequireAccount rejects requests without a valid owner resolved by resolver
// and passes the rest downstream with the AccountID attached to the request
// context. The identifier never comes from request input on its own.
func RequireAccount(resolver Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if resolver == nil {
			writeUnauthorized(w)
			return
		}

		id, ok := resolver.ResolveAccountID(r)
		if !ok || !id.Valid() {
			writeUnauthorized(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithAccountID(r.Context(), id)))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
