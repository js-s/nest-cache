package categories

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/user/nest-cash/internal/account"
)

// maxRequestBodyBytes caps category bodies; same 4KB budget as auth.
const maxRequestBodyBytes = 4 << 10

// Handler serves GET/POST over the per-user categories store.
type Handler struct {
	store *Store
}

// NewHandler returns a Handler over store.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

type subDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type groupDTO struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Children []subDTO `json:"children"`
}

type createInput struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
}

// List returns the seeded tree for the logged-in account.
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
	if err := h.store.EnsureSeeded(r.Context(), string(id)); err != nil {
		writeServerError(w)
		return
	}
	groups, err := h.store.List(r.Context(), string(id))
	if err != nil {
		writeServerError(w)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(groups))
}

// Create adds a group (no parent_id) or subcategory (with parent_id).
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
	kind := strings.TrimSpace(in.Kind)
	name := strings.TrimSpace(in.Name)
	if (kind != KindExpense && kind != KindIncome) || name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		writeInvalid(w)
		return
	}
	if err := h.store.EnsureSeeded(r.Context(), string(id)); err != nil {
		writeServerError(w)
		return
	}
	if strings.TrimSpace(in.ParentID) == "" {
		g, created, err := h.store.CreateGroup(r.Context(), string(id), kind, name)
		if err != nil {
			// Invalid input already rejected above; anything else is a server fault.
			if strings.Contains(err.Error(), "invalid input") {
				writeInvalid(w)
				return
			}
			writeServerError(w)
			return
		}
		if !created {
			writeConflict(w)
			return
		}
		writeJSON(w, http.StatusCreated, groupDTO{ID: g.ID, Kind: g.Kind, Name: g.Name, Children: []subDTO{}})
		return
	}
	// Subcategory: parent must exist, belong to caller, match kind.
	// ponytail: one List to gate parent (400) before insert; insert miss after that means duplicate (409).
	groups, err := h.store.List(r.Context(), string(id))
	if err != nil {
		writeServerError(w)
		return
	}
	parentKind := ""
	for _, g := range groups {
		if g.ID == strings.TrimSpace(in.ParentID) {
			parentKind = g.Kind
			break
		}
	}
	if parentKind == "" || parentKind != kind {
		writeInvalid(w)
		return
	}
	sub, created, err := h.store.CreateSubcategory(r.Context(), string(id), strings.TrimSpace(in.ParentID), name)
	if err != nil {
		if strings.Contains(err.Error(), "invalid input") {
			writeInvalid(w)
			return
		}
		writeServerError(w)
		return
	}
	if !created {
		writeConflict(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": sub.ID, "name": sub.Name, "parent_id": strings.TrimSpace(in.ParentID), "kind": kind})
}

func toDTO(groups []Group) []groupDTO {
	out := make([]groupDTO, 0, len(groups))
	for _, g := range groups {
		kids := make([]subDTO, 0, len(g.Children))
		for _, c := range g.Children {
			kids = append(kids, subDTO{ID: c.ID, Name: c.Name})
		}
		out = append(out, groupDTO{ID: g.ID, Kind: g.Kind, Name: g.Name, Children: kids})
	}
	return out
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

func writeConflict(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict"})
}

func writeServerError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_server_error"})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
}
