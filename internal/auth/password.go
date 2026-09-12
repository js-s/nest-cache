package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is ~250ms/login at personal scale (plan: off hot path).
const bcryptCost = 12

// maxPasswordBytes is bcrypt's hard input limit.
const maxPasswordBytes = 72

// ErrPasswordTooLong reports input exceeding bcrypt's 72-byte limit.
// Handlers map it to 400 invalid_request.
var ErrPasswordTooLong = fmt.Errorf("auth: password exceeds %d bytes", maxPasswordBytes)

// HashPassword hashes pw with bcrypt cost 12. Rejects >72-byte input.
func HashPassword(pw string) (string, error) {
	if len([]byte(pw)) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether pw matches hash. False on mismatch or
// malformed hash — callers return uniform 401 either way (no oracle).
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
