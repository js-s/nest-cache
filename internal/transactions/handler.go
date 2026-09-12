package transactions

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/user/nest-cash/internal/account"
)

// maxRequestBodyBytes caps transaction bodies; same 4KB budget as categories.
const maxRequestBodyBytes = 4 << 10

// defaultLimit and maxLimit bound the list page size.
const (
	defaultLimit = 20
	maxLimit     = 100
)

// Handler serves GET/POST over the per-user transactions store.
type Handler struct {
	store *Store
}

// NewHandler returns a Handler over store.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

type transactionDTO struct {
	ID           string `json:"id"`
	Amount       string `json:"amount"`
	Kind         string `json:"kind"`
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	ParentName   string `json:"parent_name,omitempty"`
	OccurredOn   string `json:"occurred_on"`
	Description  string `json:"description"`
}

type createInput struct {
	Amount      string `json:"amount"`
	CategoryID  string `json:"category_id"`
	OccurredOn  string `json:"occurred_on"`
	Description string `json:"description,omitempty"`
}

type listDTO struct {
	Items []transactionDTO `json:"items"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
	Total int              `json:"total"`
}

// Create records an expense for the logged-in account.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	var in createInput
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalid(w)
		return
	}
	amount := strings.TrimSpace(in.Amount)
	categoryID := strings.TrimSpace(in.CategoryID)
	occurredOn := strings.TrimSpace(in.OccurredOn)
	if amount == "" || categoryID == "" {
		writeInvalid(w)
		return
	}
	if _, err := time.Parse("2006-01-02", occurredOn); err != nil {
		writeInvalid(w)
		return
	}
	// The store rejects bad amounts, dates, and foreign/income/missing
	// categories uniformly as ok=false; only a real DB fault is an error.
	tx, created, err := h.store.Create(r.Context(), string(id), categoryID, amount, occurredOn, in.Description)
	if err != nil {
		writeServerError(w)
		return
	}
	if !created {
		writeInvalid(w)
		return
	}
	writeJSON(w, http.StatusCreated, toDTO(tx))
}

// List returns one page of the logged-in account's operations, newest first.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	page := parseBounded(r.URL.Query().Get("page"), 1, 0)
	limit := parseBounded(r.URL.Query().Get("limit"), defaultLimit, maxLimit)
	items, total, err := h.store.List(r.Context(), string(id), page, limit)
	if err != nil {
		writeServerError(w)
		return
	}
	out := make([]transactionDTO, 0, len(items))
	for _, t := range items {
		out = append(out, toDTO(t))
	}
	writeJSON(w, http.StatusOK, listDTO{Items: out, Page: page, Limit: limit, Total: total})
}

// parseBounded reads a positive query int, falling back to def when absent or
// malformed and clamping to max when max > 0.
func parseBounded(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	if max > 0 && n > max {
		return max
	}
	return n
}

func toDTO(t Transaction) transactionDTO {
	return transactionDTO{
		ID:           t.ID,
		Amount:       t.Amount,
		Kind:         t.Kind,
		CategoryID:   t.CategoryID,
		CategoryName: t.CategoryName,
		ParentName:   t.ParentName,
		OccurredOn:   t.OccurredOn.Format("2006-01-02"),
		Description:  t.Description,
	}
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
