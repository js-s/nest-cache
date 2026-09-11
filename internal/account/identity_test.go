package account

import (
	"context"
	"testing"
)

func TestAccountIDContextRoundTrip(t *testing.T) {
	ctx := WithAccountID(context.Background(), AccountID("acct-1"))

	got, ok := AccountIDFromContext(ctx)
	if !ok {
		t.Fatal("AccountIDFromContext ok = false, want true")
	}
	if got != AccountID("acct-1") {
		t.Fatalf("AccountIDFromContext = %q, want %q", got, "acct-1")
	}
}

func TestAccountIDFromContextMissing(t *testing.T) {
	if _, ok := AccountIDFromContext(context.Background()); ok {
		t.Fatal("AccountIDFromContext ok = true for empty context, want false")
	}
}

func TestAccountIDRejectsInvalid(t *testing.T) {
	for _, id := range []AccountID{"", "   ", "\t\n"} {
		if id.Valid() {
			t.Errorf("AccountID(%q).Valid() = true, want false", id)
		}
		ctx := WithAccountID(context.Background(), id)
		if _, ok := AccountIDFromContext(ctx); ok {
			t.Errorf("AccountIDFromContext ok = true for invalid AccountID(%q), want false", id)
		}
	}
}

func TestAccountIDFromContextIgnoresForeignValue(t *testing.T) {
	type foreignKey struct{}

	ctx := context.WithValue(context.Background(), foreignKey{}, "acct-1")
	if _, ok := AccountIDFromContext(ctx); ok {
		t.Fatal("AccountIDFromContext ok = true for foreign context value, want false")
	}
}
