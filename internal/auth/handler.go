package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/user/nest-cash/internal/account"
)

// minPasswordLength mirrors the web form rule (plan: inline min 8).
const minPasswordLength = 8

// Handler serves register/login/logout/me over the users/sessions store.
type Handler struct {
	store *Store
	// secure is the base Secure-cookie flag for non-TLS requests
	// (prod env). Per-request TLS / X-Forwarded-Proto upgrades to Secure.
	secure bool
}

// NewHandler returns a Handler over store. secure enables the Secure
// cookie attribute outside TLS (prod); localhost dev passes false.
func NewHandler(store *Store, secure bool) *Handler {
	return &Handler{store: store, secure: secure}
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type profile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Register validates input, creates the user, and auto-logs in (201).
// Duplicate email → 409 email_taken; validation failures → 400.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	email, pw, ok := h.parseCredentials(w, r)
	if !ok {
		return
	}
	hash, err := HashPassword(pw)
	if errors.Is(err, ErrPasswordTooLong) {
		writeInvalid(w)
		return
	}
	if err != nil {
		writeServerError(w)
		return
	}
	user, created, err := h.store.CreateUser(r.Context(), email, hash)
	if err != nil {
		writeServerError(w)
		return
	}
	if !created {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email_taken"})
		return
	}
	if !h.loginSession(w, r, user.ID) {
		return
	}
	writeJSON(w, http.StatusCreated, profile{ID: user.ID, Email: user.Email})
}

// Login checks credentials with a uniform 401 for bad-email vs
// bad-password (no account oracle).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	email, pw, ok := h.parseCredentials(w, r)
	if !ok {
		return
	}
	user, found, err := h.store.FindUserByEmail(r.Context(), email)
	if err != nil {
		writeServerError(w)
		return
	}
	if !found || !CheckPassword(user.PasswordHash, pw) {
		writeUnauthorized(w)
		return
	}
	if !h.loginSession(w, r, user.ID) {
		return
	}
	writeJSON(w, http.StatusOK, profile{ID: user.ID, Email: user.Email})
}

// Logout revokes the presented session and clears the cookie.
// Idempotent: missing/unknown cookies still clear + return 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
		if err := h.store.DeleteSession(r.Context(), HashToken(cookie.Value)); err != nil {
			writeServerError(w)
			return
		}
	}
	h.clearCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns {id,email} for the account attached by RequireAccount.
// Malformed stored id → 500 (see Store.FindUserByID); unknown user → 401.
// Refreshes the sliding expiry + cookie Max-Age (best-effort).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	user, found, err := h.store.FindUserByID(r.Context(), string(id))
	if err != nil {
		writeServerError(w)
		return
	}
	if !found {
		writeUnauthorized(w)
		return
	}
	// ponytail: sliding refresh best-effort; read failure must not break me.
	if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
		_ = h.store.TouchSession(r.Context(), HashToken(cookie.Value), time.Now().Add(SessionDuration))
		h.setCookie(w, r, cookie.Value)
	}
	writeJSON(w, http.StatusOK, profile{ID: user.ID, Email: user.Email})
}

// parseCredentials decodes + validates the JSON body. False means the
// error response is already written (400 invalid_request).
func (h *Handler) parseCredentials(w http.ResponseWriter, r *http.Request) (email, pw string, ok bool) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeInvalid(w)
		return "", "", false
	}
	email = NormalizeEmail(c.Email)
	if !validEmail(email) || len([]byte(c.Password)) < minPasswordLength || len([]byte(c.Password)) > maxPasswordBytes {
		writeInvalid(w)
		return "", "", false
	}
	return email, c.Password, true
}

// loginSession stores a fresh token for userID and sets the cookie.
// False means a 500 was already written.
func (h *Handler) loginSession(w http.ResponseWriter, r *http.Request, userID string) bool {
	raw, hash, err := NewToken()
	if err != nil {
		writeServerError(w)
		return false
	}
	if err := h.store.CreateSession(r.Context(), hash, userID, time.Now().Add(SessionDuration)); err != nil {
		writeServerError(w)
		return false
	}
	h.setCookie(w, r, raw)
	return true
}

func (h *Handler) secureFor(r *http.Request) bool {
	if h.secure {
		return true
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *Handler) setCookie(w http.ResponseWriter, r *http.Request, raw string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureFor(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   SessionMaxAge,
		Expires:  time.Now().Add(SessionDuration),
	})
}

// clearCookie expires the cookie with identical Path+Name so logout
// actually drops it.
func (h *Handler) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureFor(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at < 1 || at == len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeInvalid(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

func writeServerError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_server_error"})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
}
