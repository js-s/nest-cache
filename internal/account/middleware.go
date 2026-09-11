package account

import (
	"encoding/json"
	"net/http"
	"reflect"
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
	if isNilValue(resolver) {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeUnauthorized(w)
		})
	}
	if isNilValue(next) {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeServerError(w)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		id, ok := resolver.ResolveAccountID(r)
		if !ok || !id.Valid() {
			writeUnauthorized(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithAccountID(r.Context(), id)))
	})
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
}

func writeServerError(w http.ResponseWriter) {
	writeJSONError(w, http.StatusInternalServerError, "internal_server_error")
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
