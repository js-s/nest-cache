package auth

import (
	"net/http"

	"github.com/user/nest-cash/internal/account"
)

// Resolver is the trusted account.Resolver over opaque session cookies.
// Unknown/expired/error sessions resolve to ok=false (fail-closed 401).
type Resolver struct {
	store *Store
}

// NewResolver returns a Resolver over store.
func NewResolver(store *Store) *Resolver {
	return &Resolver{store: store}
}

// ResolveAccountID implements account.Resolver. The identifier comes from
// the server-side session lookup, never from request input.
func (s *Resolver) ResolveAccountID(r *http.Request) (account.AccountID, bool) {
	if s == nil || s.store == nil {
		return "", false
	}
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	user, _, ok, err := s.store.FindSessionUser(r.Context(), HashToken(cookie.Value))
	if err != nil || !ok {
		return "", false
	}
	id := account.AccountID(user.ID)
	if !id.Valid() {
		return "", false
	}
	return id, true
}
