package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// CookieName is the session cookie. Path=/ is set by handlers (the
// net/http default would scope it to /api/auth).
const CookieName = "nest_session"

// SessionDuration is the sliding window: 30 days, refreshed on me hits.
const SessionDuration = 30 * 24 * time.Hour

// SessionMaxAge is SessionDuration in seconds for the cookie Max-Age.
const SessionMaxAge = 30 * 24 * 60 * 60 // 2592000

// NewToken generates a 256-bit opaque token. raw goes to the cookie;
// hash (sha256 hex) is what gets stored — raw is never persisted.
// ponytail: sha256 not HMAC; SESSION_SECRET is a startup presence gate only.
func NewToken() (raw, hash string, err error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", "", fmt.Errorf("auth: new token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf[:])
	return raw, HashToken(raw), nil
}

// HashToken maps a presented raw token to its stored hash.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
