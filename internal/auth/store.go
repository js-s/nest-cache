package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// User is the account owner row. PasswordHash never leaves the store.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Session binds an opaque token hash to its owner until expiry.
type Session struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Store persists users and sessions over database/sql (pgx driver).
type Store struct {
	db *sql.DB
}

// NewStore returns a Store over db. It keeps the pool the caller configured.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// NormalizeEmail trims and lowercases so uniqueness is case-insensitive.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CreateUser inserts a user with an already-hashed password.
// Email is normalized; duplicates return ok=false with no error.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (user User, ok bool, err error) {
	email = NormalizeEmail(email)
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 ON CONFLICT (email) DO NOTHING
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("auth: create user: %w", err)
	}
	return user, true, nil
}

// FindUserByEmail returns the user for a normalized email, ok=false when absent.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (user User, ok bool, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`,
		NormalizeEmail(email),
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("auth: find user by email: %w", err)
	}
	return user, true, nil
}

// FindUserByID returns the user for id, ok=false when absent.
// A malformed id returns err (not ok=false); callers map it to 500, not 404.
func (s *Store) FindUserByID(ctx context.Context, id string) (user User, ok bool, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("auth: find user by id: %w", err)
	}
	return user, true, nil
}

// CreateSession stores a token hash for userID until expiresAt.
func (s *Store) CreateSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	if tokenHash == "" || userID == "" {
		return fmt.Errorf("auth: create session: empty token hash or user id")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("auth: create session: %w", err)
	}
	return nil
}

// FindSessionUser resolves a live session to its owner.
// Expired or unknown hashes return ok=false — they never resolve.
// PasswordHash is deliberately not loaded: session resolution only needs
// identity.
func (s *Store) FindSessionUser(ctx context.Context, tokenHash string) (user User, session Session, ok bool, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT u.id, u.email, u.created_at,
		        s.token_hash, s.user_id, s.expires_at, s.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash,
	).Scan(&user.ID, &user.Email, &user.CreatedAt,
		&session.TokenHash, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return User{}, Session{}, false, nil
	}
	if err != nil {
		return User{}, Session{}, false, fmt.Errorf("auth: find session user: %w", err)
	}
	return user, session, true, nil
}

// TouchSession extends a live session's expiry (sliding session).
// Unknown or expired hashes are a silent no-op returning nil.
func (s *Store) TouchSession(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	if tokenHash == "" {
		return fmt.Errorf("auth: touch session: empty token hash")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $2 WHERE token_hash = $1 AND expires_at > now()`,
		tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("auth: touch session: %w", err)
	}
	return nil
}

// DeleteSession revokes one session; unknown hashes are a no-op.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes dead rows; returns the purged count.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("auth: delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}
