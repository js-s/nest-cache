package transactions

import (
	"encoding/json"
	"errors"
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
	Kind        string `json:"kind,omitempty"`
}

// updateInput carries the mutable fields only; kind is immutable, so a PUT
// cannot change a transaction's type.
type updateInput struct {
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

type summaryRowDTO struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	ParentName   string `json:"parent_name,omitempty"`
	Total        string `json:"total"`
}

type summaryDTO struct {
	From   string           `json:"from"`
	To     string           `json:"to"`
	Rows   []summaryRowDTO  `json:"rows"`
	Total  string           `json:"total"`
	Items  []transactionDTO `json:"items"`
	Budget budgetDTO        `json:"budget"`
}

type budgetDTO struct {
	ExpenseTotal string  `json:"expense_total"`
	IncomeTotal  string  `json:"income_total"`
	RatioPct     *string `json:"ratio_pct"`
}

// Create records a transaction for the logged-in account. kind is optional
// (empty trims to expense); anything outside expense|income is a 400.
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
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = KindExpense
	}
	if amount == "" || categoryID == "" {
		writeInvalid(w)
		return
	}
	if kind != KindExpense && kind != KindIncome {
		writeInvalid(w)
		return
	}
	if _, err := time.Parse("2006-01-02", occurredOn); err != nil {
		writeInvalid(w)
		return
	}
	// The store rejects bad amounts, dates, and foreign/mismatched/missing
	// categories uniformly as ok=false; only a real DB fault is an error.
	tx, created, err := h.store.Create(r.Context(), string(id), categoryID, kind, amount, occurredOn, in.Description)
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

// Update replaces the editable fields of an owned transaction. kind is not part
// of the payload; a foreign or unknown id is a 404.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	txID := r.PathValue("id")
	if !uuidRe.MatchString(txID) {
		writeInvalid(w)
		return
	}
	var in updateInput
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
	tx, updated, err := h.store.Update(r.Context(), string(id), txID, categoryID, amount, occurredOn, in.Description)
	if errors.Is(err, ErrNotFound) {
		writeNotFound(w)
		return
	}
	if err != nil {
		writeServerError(w)
		return
	}
	if !updated {
		writeInvalid(w)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(tx))
}

// Delete removes an owned transaction and answers 204 with an empty body. A
// foreign or unknown id is a 404; a malformed one is a 400.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	txID := r.PathValue("id")
	if !uuidRe.MatchString(txID) {
		writeInvalid(w)
		return
	}
	deleted, err := h.store.Delete(r.Context(), string(id), txID)
	if errors.Is(err, ErrNotFound) {
		writeNotFound(w)
		return
	}
	if err != nil {
		writeServerError(w)
		return
	}
	if !deleted {
		writeInvalid(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
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
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind == "" {
		kind = "all"
	}
	if kind != "all" && kind != KindExpense && kind != KindIncome {
		writeInvalid(w)
		return
	}
	items, total, err := h.store.List(r.Context(), string(id), page, limit, kind)
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

// Summary returns expense aggregates per category plus the newest items
// under the same from/to/category_id filter.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id, ok := account.AccountIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	q := r.URL.Query()
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))
	if from == "" || to == "" {
		writeInvalid(w)
		return
	}
	var categoryIDs []string
	if raw := strings.TrimSpace(q.Get("category_id")); raw != "" {
		categoryIDs = strings.Split(raw, ",")
	}
	rows, total, items, budget, ok, err := h.store.Summary(r.Context(), string(id), from, to, categoryIDs)
	if err != nil {
		writeServerError(w)
		return
	}
	if !ok {
		writeInvalid(w)
		return
	}
	rowDTOs := make([]summaryRowDTO, 0, len(rows))
	for _, row := range rows {
		rowDTOs = append(rowDTOs, summaryRowDTO{
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			ParentName:   row.ParentName,
			Total:        row.Total,
		})
	}
	itemDTOs := make([]transactionDTO, 0, len(items))
	for _, t := range items {
		itemDTOs = append(itemDTOs, toDTO(t))
	}
	writeJSON(w, http.StatusOK, summaryDTO{From: from, To: to, Rows: rowDTOs, Total: total, Items: itemDTOs,
		Budget: budgetDTO{ExpenseTotal: budget.ExpenseTotal, IncomeTotal: budget.IncomeTotal, RatioPct: budget.RatioPct}})
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

func writeNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
}
