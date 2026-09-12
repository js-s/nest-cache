package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashCheckRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-8")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-horse-8" {
		t.Fatal("hash must not equal plaintext")
	}
	if !CheckPassword(hash, "correct-horse-8") {
		t.Fatal("CheckPassword(valid) = false")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("CheckPassword(invalid) = true")
	}
}

func TestHashPasswordTooLong(t *testing.T) {
	pw := strings.Repeat("a", 73)
	if _, err := HashPassword(pw); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("73-byte password err = %v, want ErrPasswordTooLong", err)
	}
	if _, err := HashPassword(strings.Repeat("a", 72)); err != nil {
		t.Fatalf("72-byte password err = %v, want nil", err)
	}
}

func TestNewToken(t *testing.T) {
	raw1, hash1, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	raw2, hash2, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatal("tokens must be unique")
	}
	if raw1 == hash1 {
		t.Fatal("raw token must differ from stored hash")
	}
	if got := HashToken(raw1); got != hash1 {
		t.Fatal("HashToken(raw) must equal stored hash")
	}
	if strings.ContainsAny(raw1, "+/=") {
		t.Fatalf("raw token must be base64url, got %q", raw1)
	}
}
