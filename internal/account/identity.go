package account

import (
	"context"
	"strings"
)

// AccountID identifies the single owner of account-scoped data. It is an
// opaque, provider-neutral string: the source of the value (session, token,
// cookie) is decided by the layer that implements Resolver.
type AccountID string

// Valid reports whether the identifier is present and not whitespace-only.
func (id AccountID) Valid() bool {
	return strings.TrimSpace(string(id)) != ""
}

type accountIDKey struct{}

// WithAccountID returns a context carrying the owner identifier for downstream
// handlers. It stores the value as-is; AccountIDFromContext is the validity
// gate on read.
func WithAccountID(ctx context.Context, id AccountID) context.Context {
	return context.WithValue(ctx, accountIDKey{}, id)
}

// AccountIDFromContext returns the owner identifier carried by ctx. It reports
// false when no identifier is present or the stored value is invalid.
func AccountIDFromContext(ctx context.Context) (AccountID, bool) {
	id, ok := ctx.Value(accountIDKey{}).(AccountID)
	if !ok || !id.Valid() {
		return "", false
	}
	return id, true
}
